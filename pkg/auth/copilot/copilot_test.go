package copilot

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/auth"
)

type server struct {
	*httptest.Server
	polls       atomic.Int32
	tokenStatus int
	githubAuth  string
}

func newServer(t *testing.T) (*Auth, *server) {
	t.Helper()
	s := &server{tokenStatus: http.StatusOK}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/device/code":
			if r.Form.Get("client_id") != ClientID || r.Form.Get("scope") != "read:user" {
				t.Errorf("device request = %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"WDJB-MJHT","verification_uri":"https://github.com/login/device","expires_in":60}`))
		case "/access_token":
			if s.polls.Add(1) < 3 {
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"gh-token"}`))
		case "/copilot_token":
			s.githubAuth = r.Header.Get("Authorization")
			if r.Header.Get("Editor-Version") == "" {
				t.Error("editor headers missing on the token exchange")
			}
			if s.tokenStatus != http.StatusOK {
				w.WriteHeader(s.tokenStatus)
				return
			}
			_, _ = w.Write([]byte(`{"token":"copilot-token","expires_at":4102444800}`))
		}
	}))
	t.Cleanup(s.Close)

	a := New()
	a.deviceCodeURL = s.URL + "/device/code"
	a.accessTokenURL = s.URL + "/access_token"
	a.copilotTokenURL = s.URL + "/copilot_token"
	a.pollInterval = time.Millisecond
	return a, s
}

func TestDeviceSignIn(t *testing.T) {
	a, s := newServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := a.Start(ctx, catwalk.KindDevice)
	if err != nil {
		t.Fatal(err)
	}
	if p := sess.Prompt(); p.UserCode != "WDJB-MJHT" || p.URL != "https://github.com/login/device" {
		t.Errorf("prompt = %+v", p)
	}
	cred, err := sess.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "copilot-token" || cred.RefreshToken != "gh-token" || cred.ExpiresAt != 4102444800 || cred.Provider != "copilot" {
		t.Errorf("credential = %+v", cred)
	}
	if s.githubAuth != "Bearer gh-token" {
		t.Errorf("exchange used %q", s.githubAuth)
	}
}

func TestNotAvailable(t *testing.T) {
	a, s := newServer(t)
	s.tokenStatus = http.StatusForbidden
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := a.Start(ctx, catwalk.KindDevice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Wait(ctx); !errors.Is(err, auth.ErrNotAvailable) {
		t.Errorf("got %v, want ErrNotAvailable", err)
	}
}

func TestRefresh(t *testing.T) {
	a, s := newServer(t)
	ctx := context.Background()

	cred, err := a.Refresh(ctx, &catwalk.Credential{RefreshToken: "gh-token"})
	if err != nil || cred.AccessToken != "copilot-token" || cred.RefreshToken != "gh-token" {
		t.Errorf("got %+v, %v", cred, err)
	}

	s.tokenStatus = http.StatusUnauthorized
	if _, err := a.Refresh(ctx, &catwalk.Credential{RefreshToken: "gh-token"}); !errors.Is(err, auth.ErrReauthRequired) {
		t.Errorf("got %v, want ErrReauthRequired", err)
	}
	if _, err := a.Refresh(ctx, &catwalk.Credential{}); !errors.Is(err, auth.ErrReauthRequired) {
		t.Errorf("no refresh token: got %v, want ErrReauthRequired", err)
	}
}

type recorder struct{ req *http.Request }

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.req = req
	return &http.Response{StatusCode: 200, Body: http.NoBody, Request: req}, nil
}

func TestRequestInitiator(t *testing.T) {
	r := New().Request(&catwalk.Credential{AccessToken: "copilot-token"})
	if r.BaseURL != APIBaseURL || r.APIKey != "copilot-token" || r.Headers["Copilot-Integration-Id"] != integrationID {
		t.Errorf("request = %+v", r)
	}

	tests := []struct {
		name, body, want string
	}{
		{"first message", `{"messages":[{"role":"user","content":"hi"}]}`, "user"},
		{"has assistant turn", `{"messages":[{"role":"user"},{"role": "assistant"}]}`, "agent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{}
			req, _ := http.NewRequest(http.MethodPost, "https://api.githubcopilot.com/chat/completions", strings.NewReader(tt.body))
			if _, err := r.Client(rec).Do(req); err != nil {
				t.Fatal(err)
			}
			if got := rec.req.Header.Get("X-Initiator"); got != tt.want {
				t.Errorf("X-Initiator = %q, want %q", got, tt.want)
			}
			data, _ := io.ReadAll(rec.req.Body)
			if string(data) != tt.body {
				t.Errorf("body changed: %q", data)
			}
		})
	}

	t.Run("no body", func(t *testing.T) {
		rec := &recorder{}
		req, _ := http.NewRequest(http.MethodGet, "https://api.githubcopilot.com/models", nil)
		if _, err := r.Client(rec).Do(req); err != nil {
			t.Fatal(err)
		}
		if rec.req.Header.Get("X-Initiator") != "user" {
			t.Errorf("X-Initiator = %q", rec.req.Header.Get("X-Initiator"))
		}
	})
}

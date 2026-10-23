package openai

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/pkg/auth"
)

func jwt(payload string) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(payload)) + ".sig"
}

type stub struct {
	*httptest.Server
	form url.Values
}

func newStub(t *testing.T, handler func(url.Values, http.ResponseWriter)) *stub {
	t.Helper()
	s := &stub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		s.form = r.Form
		handler(r.Form, w)
	}))
	t.Cleanup(s.Close)
	return s
}

func newAuth(s *stub) *Auth {
	a := New()
	a.issuer = s.URL
	a.ports = nil
	return a
}

func TestBrowserSignIn(t *testing.T) {
	s := newStub(t, func(_ url.Values, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"id_token":"` +
			jwt(`{"https://api.openai.com/auth":{"chatgpt_account_id":"acct-1"}}`) + `"}`))
	})
	a := newAuth(s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := a.Start(ctx, catwalk.KindBrowser)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	u, err := url.Parse(sess.Prompt().URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sess.Prompt().URL, s.URL+"/oauth/authorize?") {
		t.Fatalf("authorize URL = %s", sess.Prompt().URL)
	}
	q := u.Query()
	want := map[string]string{
		"response_type":              "code",
		"client_id":                  ClientID,
		"code_challenge_method":      "S256",
		"id_token_add_organizations": "true",
		"codex_cli_simplified_flow":  "true",
		"scope":                      scope,
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, q.Get(k), v)
		}
	}
	redirect := q.Get("redirect_uri")
	if !strings.HasPrefix(redirect, "http://localhost:") || !strings.HasSuffix(redirect, callbackPath) {
		t.Fatalf("redirect_uri = %s", redirect)
	}

	go func() {
		resp, err := http.Get(strings.Replace(redirect, "localhost", "127.0.0.1", 1) + "?code=the-code&state=" + url.QueryEscape(q.Get("state")))
		if err == nil {
			resp.Body.Close()
		}
	}()
	cred, err := sess.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "access" || cred.RefreshToken != "refresh" || cred.Extra[AccountID] != "acct-1" || cred.Provider != "openai" {
		t.Errorf("credential = %+v", cred)
	}
	if s.form.Get("code") != "the-code" || s.form.Get("grant_type") != "authorization_code" {
		t.Errorf("token request = %v", s.form)
	}
	if auth.Challenge(s.form.Get("code_verifier")) != q.Get("code_challenge") {
		t.Error("code_verifier does not match the challenge sent to the browser")
	}
}

func TestRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("keeps refresh token and account", func(t *testing.T) {
		s := newStub(t, func(_ url.Values, w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"access_token":"new","expires_in":3600}`))
		})
		prev := &catwalk.Credential{RefreshToken: "old", Extra: map[string]string{AccountID: "acct-1"}}
		cred, err := newAuth(s).Refresh(ctx, prev)
		if err != nil {
			t.Fatal(err)
		}
		if cred.AccessToken != "new" || cred.RefreshToken != "old" || cred.Extra[AccountID] != "acct-1" {
			t.Errorf("credential = %+v", cred)
		}
		if s.form.Get("grant_type") != "refresh_token" || s.form.Get("refresh_token") != "old" {
			t.Errorf("request = %v", s.form)
		}
	})

	t.Run("rotated refresh token", func(t *testing.T) {
		s := newStub(t, func(_ url.Values, w http.ResponseWriter) {
			_, _ = w.Write([]byte(`{"access_token":"new","refresh_token":"rotated","expires_in":60}`))
		})
		cred, err := newAuth(s).Refresh(ctx, &catwalk.Credential{RefreshToken: "old"})
		if err != nil || cred.RefreshToken != "rotated" {
			t.Errorf("got %+v, %v", cred, err)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		s := newStub(t, func(_ url.Values, w http.ResponseWriter) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		})
		_, err := newAuth(s).Refresh(ctx, &catwalk.Credential{RefreshToken: "old"})
		if !errors.Is(err, auth.ErrReauthRequired) {
			t.Errorf("got %v, want ErrReauthRequired", err)
		}
	})

	t.Run("no refresh token", func(t *testing.T) {
		_, err := New().Refresh(ctx, &catwalk.Credential{})
		if !errors.Is(err, auth.ErrReauthRequired) {
			t.Errorf("got %v, want ErrReauthRequired", err)
		}
	})
}

func TestUnsupportedKind(t *testing.T) {
	if _, err := New().Start(context.Background(), catwalk.KindDevice); !errors.Is(err, auth.ErrUnsupportedKind) {
		t.Errorf("got %v, want ErrUnsupportedKind", err)
	}
}

func TestAccountID(t *testing.T) {
	tests := []struct {
		name, token, want string
	}{
		{"top level claim", jwt(`{"chatgpt_account_id":"top"}`), "top"},
		{"nested claim", jwt(`{"https://api.openai.com/auth":{"chatgpt_account_id":"nested"}}`), "nested"},
		{"not a jwt", "opaque", ""},
		{"bad payload", "a.!!!.c", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := accountID(tt.token); got != tt.want {
				t.Errorf("accountID() = %q, want %q", got, tt.want)
			}
		})
	}
}

type recorder struct{ req *http.Request }

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	r.req = req
	return &http.Response{StatusCode: 200, Body: http.NoBody, Request: req}, nil
}

func TestRequest(t *testing.T) {
	cred := &catwalk.Credential{AccessToken: "tok", Extra: map[string]string{AccountID: "acct-1"}}
	a := New()
	a.Originator = "fantasia"
	r := a.Request(cred)
	if r.BaseURL != CodexBaseURL || r.APIKey != "tok" || r.Headers["chatgpt-account-id"] != "acct-1" || r.Headers["originator"] != "fantasia" {
		t.Errorf("request = %+v", r)
	}

	send := func(t *testing.T, target, body string) *http.Request {
		t.Helper()
		rec := &recorder{}
		client := r.Client(rec)
		req, _ := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer tok")
		if _, err := client.Do(req); err != nil {
			t.Fatal(err)
		}
		return rec.req
	}

	t.Run("codex backend", func(t *testing.T) {
		got := send(t, "https://chatgpt.com/backend-api/codex/responses", `{"model":"m","max_output_tokens":10}`)
		data, _ := io.ReadAll(got.Body)
		if strings.Contains(string(data), "max_output_tokens") || !strings.Contains(string(data), `"model":"m"`) {
			t.Errorf("body = %s", data)
		}
		if got.ContentLength != int64(len(data)) {
			t.Errorf("ContentLength = %d, body is %d bytes", got.ContentLength, len(data))
		}
		if got.Header.Get("chatgpt-account-id") != "acct-1" || got.Header.Get("originator") != "fantasia" || got.Header.Get("Authorization") == "" {
			t.Errorf("headers = %v", got.Header)
		}
	})

	t.Run("official api host", func(t *testing.T) {
		got := send(t, "https://api.openai.com/v1/responses", `{"max_output_tokens":10}`)
		data, _ := io.ReadAll(got.Body)
		if !strings.Contains(string(data), "max_output_tokens") || got.Header.Get("Authorization") == "" {
			t.Errorf("official host was altered: %s %v", data, got.Header)
		}
	})

	t.Run("foreign host never gets credentials", func(t *testing.T) {
		got := send(t, "https://evil.example/v1/responses", `{}`)
		if got.Header.Get("Authorization") != "" || got.Header.Get("chatgpt-account-id") != "" {
			t.Errorf("credentials leaked: %v", got.Header)
		}
	})

	t.Run("non-json body", func(t *testing.T) {
		got := send(t, "https://chatgpt.com/x", `not json`)
		data, _ := io.ReadAll(got.Body)
		if string(data) != "not json" {
			t.Errorf("body = %q", data)
		}
	})
}

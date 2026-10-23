package grok

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/catwalk/pkg/auth"
)

func newAuth(t *testing.T, handler http.HandlerFunc) *Auth {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	a := New()
	a.issuer = srv.URL
	a.pollInterval = time.Millisecond
	return a
}

func TestBrowserSignIn(t *testing.T) {
	var form url.Values
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.Form
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := a.Start(ctx, catwalk.KindBrowser)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()

	u, _ := url.Parse(sess.Prompt().URL)
	q := u.Query()
	if u.Path != "/oauth2/authorize" || q.Get("client_id") != ClientID || q.Get("nonce") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorize URL = %s", sess.Prompt().URL)
	}
	redirect := q.Get("redirect_uri")
	if !strings.HasPrefix(redirect, "http://127.0.0.1:") || !strings.HasSuffix(redirect, "/callback") {
		t.Fatalf("redirect_uri = %s", redirect)
	}

	go func() {
		resp, err := http.Get(redirect + "?code=c&state=" + url.QueryEscape(q.Get("state")))
		if err == nil {
			resp.Body.Close()
		}
	}()
	cred, err := sess.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "access" || cred.RefreshToken != "refresh" || cred.Provider != "xai" {
		t.Errorf("credential = %+v", cred)
	}
	if auth.Challenge(form.Get("code_verifier")) != q.Get("code_challenge") || form.Get("redirect_uri") != redirect {
		t.Errorf("token request = %v", form)
	}
}

func TestDeviceSignIn(t *testing.T) {
	var polls atomic.Int32
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/oauth2/device/code":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD-EFGH","verification_uri":"https://verify.example","expires_in":60}`))
		case "/oauth2/token":
			if r.Form.Get("device_code") != "dev" {
				t.Errorf("device_code = %q", r.Form.Get("device_code"))
			}
			if polls.Add(1) < 3 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`))
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := a.Start(ctx, catwalk.KindDevice)
	if err != nil {
		t.Fatal(err)
	}
	p := sess.Prompt()
	if p.Kind != catwalk.KindDevice || p.UserCode != "ABCD-EFGH" || p.URL != "https://verify.example" || p.ExpiresAt.IsZero() {
		t.Errorf("prompt = %+v", p)
	}
	cred, err := sess.Wait(ctx)
	if err != nil || cred.AccessToken != "access" || polls.Load() != 3 {
		t.Errorf("got %+v, %v after %d polls", cred, err, polls.Load())
	}
}

func TestDeviceDenied(t *testing.T) {
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/device/code" {
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"U","verification_uri":"https://v.example"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	})
	sess, err := a.Start(context.Background(), catwalk.KindDevice)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.Wait(context.Background()); !errors.Is(err, auth.ErrDenied) {
		t.Errorf("got %v, want ErrDenied", err)
	}
}

func TestRefreshAndRequest(t *testing.T) {
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("refresh_token") == "dead" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"new","expires_in":60}`))
	})
	ctx := context.Background()

	cred, err := a.Refresh(ctx, &catwalk.Credential{RefreshToken: "live"})
	if err != nil || cred.AccessToken != "new" || cred.RefreshToken != "live" {
		t.Errorf("got %+v, %v", cred, err)
	}
	if _, err := a.Refresh(ctx, &catwalk.Credential{RefreshToken: "dead"}); !errors.Is(err, auth.ErrReauthRequired) {
		t.Errorf("got %v, want ErrReauthRequired", err)
	}

	r := a.Request(cred)
	if r.BaseURL != APIBaseURL || r.APIKey != "new" || r.Wrap != nil {
		t.Errorf("request = %+v", r)
	}
}

package auth

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestChallengeRFC7636Vector(t *testing.T) {
	const (
		verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	)
	if got := Challenge(verifier); got != challenge {
		t.Errorf("Challenge() = %q, want %q", got, challenge)
	}
}

func TestNewPKCE(t *testing.T) {
	a, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewPKCE()
	if a.Verifier == b.Verifier {
		t.Error("verifiers repeat")
	}
	if len(a.Verifier) < 43 || len(a.Verifier) > 128 {
		t.Errorf("verifier length %d outside RFC 7636 bounds", len(a.Verifier))
	}
	if Challenge(a.Verifier) != a.Challenge {
		t.Error("challenge does not match verifier")
	}
}

func TestRefreshError(t *testing.T) {
	revoked := &TokenError{Status: 400, Code: "invalid_grant"}
	if err := RefreshError(revoked); !errors.Is(err, ErrReauthRequired) {
		t.Errorf("revoked refresh = %v, want ErrReauthRequired", err)
	}
	other := &TokenError{Status: 500, Body: "oops"}
	if err := RefreshError(other); errors.Is(err, ErrReauthRequired) {
		t.Errorf("server error = %v, must not require reauth", err)
	}
}

func TestPollToken(t *testing.T) {
	ctx := context.Background()

	t.Run("pending then done", func(t *testing.T) {
		calls := 0
		cred, err := PollToken(ctx, time.Millisecond, time.Minute, func(context.Context) (*catwalk.Credential, error) {
			calls++
			if calls < 3 {
				return nil, ErrPending
			}
			return &catwalk.Credential{AccessToken: "ok"}, nil
		})
		if err != nil || cred.AccessToken != "ok" || calls != 3 {
			t.Errorf("got %v, %v after %d calls", cred, err, calls)
		}
	})

	t.Run("standard device errors", func(t *testing.T) {
		tests := map[string]error{
			"expired_token": ErrExpired,
			"access_denied": ErrDenied,
		}
		for code, want := range tests {
			_, err := PollToken(ctx, time.Millisecond, time.Minute, func(context.Context) (*catwalk.Credential, error) {
				return nil, &TokenError{Status: 400, Code: code}
			})
			if !errors.Is(err, want) {
				t.Errorf("%s: got %v, want %v", code, err, want)
			}
		}
	})

	t.Run("pending code from the token endpoint", func(t *testing.T) {
		calls := 0
		_, err := PollToken(ctx, time.Millisecond, time.Minute, func(context.Context) (*catwalk.Credential, error) {
			calls++
			if calls == 1 {
				return nil, &TokenError{Status: 400, Code: "authorization_pending"}
			}
			return &catwalk.Credential{}, nil
		})
		if err != nil || calls != 2 {
			t.Errorf("err = %v after %d calls", err, calls)
		}
	})

	t.Run("deadline", func(t *testing.T) {
		_, err := PollToken(ctx, time.Millisecond, 5*time.Millisecond, func(context.Context) (*catwalk.Credential, error) {
			return nil, ErrPending
		})
		if !errors.Is(err, ErrExpired) {
			t.Errorf("got %v, want ErrExpired", err)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := PollToken(cctx, time.Hour, time.Hour, func(context.Context) (*catwalk.Credential, error) {
			return nil, ErrPending
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want context.Canceled", err)
		}
	})
}

func TestRequestToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Form.Get("grant_type") {
		case "good":
			_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","expires_in":3600}`))
		case "empty":
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"gone"}`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	tr, err := RequestToken(ctx, nil, srv.URL, url.Values{"grant_type": {"good"}})
	if err != nil || tr.AccessToken != "a" {
		t.Fatalf("good: %v, %v", tr, err)
	}
	cred := tr.Credential("openai", &catwalk.Credential{RefreshToken: "old", Extra: map[string]string{"x": "y"}})
	if cred.RefreshToken != "r" || cred.Extra["x"] != "y" || cred.ExpiresAt == 0 {
		t.Errorf("credential = %+v", cred)
	}

	if _, err := RequestToken(ctx, nil, srv.URL, url.Values{"grant_type": {"empty"}}); err == nil {
		t.Error("empty access token accepted")
	}

	_, err = RequestToken(ctx, nil, srv.URL, url.Values{"grant_type": {"bad"}})
	var te *TokenError
	if !errors.As(err, &te) || te.Code != "invalid_grant" || te.Description != "gone" || !te.Revoked() {
		t.Errorf("bad: %v", err)
	}
}

func TestBrowserSession(t *testing.T) {
	newSession := func(t *testing.T) (*BrowserSession, string) {
		t.Helper()
		loop, err := ListenLoopback("127.0.0.1", nil, "/cb", "Test")
		if err != nil {
			t.Fatal(err)
		}
		s := NewBrowserSession("https://example.test/authorize", "state-1", loop, func(_ context.Context, code string) (*catwalk.Credential, error) {
			return &catwalk.Credential{AccessToken: "token-for-" + code}, nil
		})
		t.Cleanup(func() { _ = s.Close() })
		return s, loop.RedirectURI()
	}
	visit := func(t *testing.T, redirect string, q url.Values) int {
		t.Helper()
		resp, err := http.Get(redirect + "?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("success", func(t *testing.T) {
		s, redirect := newSession(t)
		if p := s.Prompt(); p.Kind != catwalk.KindBrowser || p.URL == "" {
			t.Errorf("prompt = %+v", p)
		}
		visit(t, redirect, url.Values{"state": {"state-1"}, "code": {"abc"}})
		cred, err := s.Wait(ctx)
		if err != nil || cred.AccessToken != "token-for-abc" {
			t.Errorf("got %v, %v", cred, err)
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		s, redirect := newSession(t)
		visit(t, redirect, url.Values{"state": {"forged"}, "code": {"abc"}})
		if _, err := s.Wait(ctx); err == nil {
			t.Error("forged state accepted")
		}
	})

	t.Run("denied", func(t *testing.T) {
		s, redirect := newSession(t)
		if status := visit(t, redirect, url.Values{"error": {"access_denied"}}); status != http.StatusBadRequest {
			t.Errorf("landing status = %d", status)
		}
		if _, err := s.Wait(ctx); !errors.Is(err, ErrDenied) {
			t.Errorf("got %v, want ErrDenied", err)
		}
	})

	t.Run("complete without loopback", func(t *testing.T) {
		s, _ := newSession(t)
		cred, err := s.Complete(ctx, url.Values{"state": {"state-1"}, "code": {"manual"}})
		if err != nil || cred.AccessToken != "token-for-manual" {
			t.Errorf("got %v, %v", cred, err)
		}
		if _, err := s.Complete(ctx, url.Values{"state": {"state-1"}}); err == nil {
			t.Error("missing code accepted")
		}
	})

	t.Run("cancel", func(t *testing.T) {
		s, _ := newSession(t)
		cctx, ccancel := context.WithCancel(ctx)
		ccancel()
		if _, err := s.Wait(cctx); !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want context.Canceled", err)
		}
	})
}

func TestDeviceSessionClose(t *testing.T) {
	s := NewDeviceSession(catwalk.Prompt{Kind: catwalk.KindDevice}, func(ctx context.Context) (*catwalk.Credential, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	done := make(chan error, 1)
	go func() {
		_, err := s.Wait(context.Background())
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	_ = s.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not stop Wait")
	}
}

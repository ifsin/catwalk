package hyper

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	a.BaseURL = srv.URL
	a.DeviceName = "Test Mac"
	a.pollInterval = time.Millisecond
	return a
}

func TestDeviceSignIn(t *testing.T) {
	var polls atomic.Int32
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/device/auth":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["device_name"] != "Test Mac" {
				t.Errorf("device_name = %q", body["device_name"])
			}
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD","verification_url":"https://hyper.example/verify","expires_in":60}`))
		case r.Method == http.MethodGet && r.URL.Path == "/device/auth/dev":
			switch polls.Add(1) {
			case 1:
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			case 2:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			default:
				_, _ = w.Write([]byte(`{"refresh_token":"hyper-refresh","user_id":"u1"}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/token/exchange":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["refresh_token"] != "hyper-refresh" {
				t.Errorf("exchange body = %v", body)
			}
			_, _ = w.Write([]byte(`{"access_token":"hyper-access","expires_in":3600}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := a.Start(ctx, catwalk.KindDevice)
	if err != nil {
		t.Fatal(err)
	}
	if p := sess.Prompt(); p.UserCode != "ABCD" || p.URL != "https://hyper.example/verify" {
		t.Errorf("prompt = %+v", p)
	}
	cred, err := sess.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cred.AccessToken != "hyper-access" || cred.RefreshToken != "hyper-refresh" || cred.ExpiresAt == 0 || cred.Provider != "hyper" {
		t.Errorf("credential = %+v", cred)
	}
}

func TestDeviceFailure(t *testing.T) {
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"U","verification_url":"https://v.example"}`))
			return
		}
		_, _ = w.Write([]byte(`{"error":"rejected","error_description":"the request was rejected"}`))
	})
	sess, err := a.Start(context.Background(), catwalk.KindDevice)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sess.Wait(context.Background())
	if err == nil || err.Error() != "the request was rejected" {
		t.Errorf("got %v", err)
	}
}

func TestRefreshAndRequest(t *testing.T) {
	a := newAuth(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["refresh_token"] == "dead" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"fresh","expires_at":4102444800}`))
	})
	ctx := context.Background()

	cred, err := a.Refresh(ctx, &catwalk.Credential{RefreshToken: "live"})
	if err != nil || cred.AccessToken != "fresh" || cred.RefreshToken != "live" || cred.ExpiresAt != 4102444800 {
		t.Errorf("got %+v, %v", cred, err)
	}
	if _, err := a.Refresh(ctx, &catwalk.Credential{RefreshToken: "dead"}); !errors.Is(err, auth.ErrReauthRequired) {
		t.Errorf("got %v, want ErrReauthRequired", err)
	}

	r := a.Request(cred)
	if r.BaseURL != a.BaseURL+"/v1" || r.APIKey != "fresh" {
		t.Errorf("request = %+v", r)
	}
}

func TestBaseURLFromEnvironment(t *testing.T) {
	t.Setenv("HYPER_URL", "https://staging.example/")
	if got := New().base(); got != "https://staging.example" {
		t.Errorf("base() = %q", got)
	}
}

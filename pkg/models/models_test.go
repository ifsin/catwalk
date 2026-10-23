package models

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func serve(t *testing.T, status int, body string, check func(*http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if check != nil {
			check(r)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGetSendsBearerAndHeaders(t *testing.T) {
	srv := serve(t, 200, `ok`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer k" || r.Header.Get("X-Extra") != "1" {
			t.Errorf("bad request %s %v", r.URL, r.Header)
		}
	})
	body, err := Get(context.Background(), catwalk.Source{APIKey: "k", Headers: map[string]string{"X-Extra": "1"}}, srv.URL+"/v1/models")
	if err != nil || string(body) != "ok" {
		t.Fatalf("got %q, %v", body, err)
	}
}

func TestEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Copilot{}).Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, APIKey: "t", BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

func TestBadStatusAndBadJSON(t *testing.T) {
	srv := serve(t, 401, `{"error":"no"}`, nil)
	_, err := Copilot{}.Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, APIKey: "t", BaseURL: srv.URL})
	var se *StatusError
	if !errors.As(err, &se) || se.Status != 401 {
		t.Fatalf("got %v", err)
	}
	bad := serve(t, 200, `not json`, nil)
	if _, err := (Copilot{}).Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, APIKey: "t", BaseURL: bad.URL}); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestCopilotNeedsSignIn(t *testing.T) {
	for _, src := range []catwalk.Source{
		{BaseURL: "http://x", APIKey: "k"},
		{Auth: catwalk.AuthAPIKey, BaseURL: "http://x", APIKey: "k"},
		{Auth: catwalk.AuthSignIn, BaseURL: "http://x"},
	} {
		if _, err := (Copilot{}).Models(context.Background(), src); !errors.Is(err, ErrNotSignedIn) {
			t.Errorf("%+v: got %v", src, err)
		}
	}
}

func TestJoin(t *testing.T) {
	got, err := Join(catwalk.Source{BaseURL: "https://x.test/v1/"}, "/models")
	if err != nil || got != "https://x.test/v1/models" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := Join(catwalk.Source{}, "/models"); err == nil {
		t.Fatal("expected an error without a base URL")
	}
}

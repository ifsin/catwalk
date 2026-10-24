package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestOpenCodeZenMapsEndpointType(t *testing.T) {
	srv := serve(t, 200, `{"object":"list","data":[{"id":"claude-sonnet-4-6"},{"id":"gpt-5.5"},{"id":"big-pickle"},{"id":""}]}`, func(r *http.Request) {
		if r.URL.Path != "/zen/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := OpenCodeZen{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/zen/v1", APIKey: "k"})
	if err != nil || len(got) != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
	want := []catwalk.Model{
		{ID: "claude-sonnet-4-6", Name: "claude-sonnet-4-6", Type: catwalk.TypeMessages},
		{ID: "gpt-5.5", Name: "gpt-5.5", Type: catwalk.TypeResponses},
		{ID: "big-pickle", Name: "big-pickle"},
	}
	for i := range want {
		if got[i].ID != want[i].ID || got[i].Name != want[i].Name || got[i].Type != want[i].Type {
			t.Errorf("model %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOpenCodeZenEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"object":"list","data":[]}`, nil)
	if _, err := (OpenCodeZen{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

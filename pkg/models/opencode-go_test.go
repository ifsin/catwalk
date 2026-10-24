package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestOpenCodeGoMapsEndpointType(t *testing.T) {
	srv := serve(t, 200, `{"object":"list","data":[{"id":"minimax-m2.7"},{"id":"grok-4.6"},{"id":"glm-5.3"},{"id":""}]}`, func(r *http.Request) {
		if r.URL.Path != "/zen/go/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := OpenCodeGo{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/zen/go/v1", APIKey: "k"})
	if err != nil || len(got) != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].Type != catwalk.TypeMessages || got[1].Type != catwalk.TypeResponses || got[2].Type != "" {
		t.Errorf("got types %q %q %q", got[0].Type, got[1].Type, got[2].Type)
	}
}

func TestOpenCodeGoEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"object":"list","data":[{"id":""}]}`, nil)
	if _, err := (OpenCodeGo{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

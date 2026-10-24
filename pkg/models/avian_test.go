package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestAvianMapsModels(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"moonshotai/kimi-k2.5","display_name":"Kimi K2.5","context_length":131072,"max_output":8192,"reasoning":true,"pricing":{"input_per_million":0.6,"output_per_million":2.5,"cache_read_per_million":0.1}},
		{"id":"","display_name":"Empty"}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Avian{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "moonshotai/kimi-k2.5" || m.Name != "Kimi K2.5" || m.Pricing.Output != 2.5 || m.DefaultMaxTokens != 8192 || m.Reasoning.Thinking != catwalk.ThinkingToggleable {
		t.Fatalf("bad mapping %+v", m)
	}
}

func TestAvianEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Avian{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

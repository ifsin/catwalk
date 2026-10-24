package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestBasetenMapsAndFilters(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"zai-org/GLM-5.3","name":"GLM 5.3","context_length":200000,"max_completion_tokens":65536,"supported_features":["tools","reasoning"],"input_modalities":["text","image"],
		 "pricing":{"prompt":"0.00000055","completion":"0.000002","input_cache_read":"0.00000011"}},
		{"id":"no-tools","supported_features":["reasoning"],"input_modalities":["text"],"pricing":{}}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Baseten{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.Name != "GLM 5.3" || m.Pricing.Input != 0.55 || m.Pricing.Output != 2 || m.DefaultMaxTokens != 65536 || !m.Capabilities.Vision || m.Reasoning.Thinking != catwalk.ThinkingToggleable {
		t.Fatalf("bad mapping %+v", m)
	}
}

func TestBasetenEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Baseten{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

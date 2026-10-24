package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestAIHubMixMapsLLMs(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"model_id":"gpt-5","model_name":"GPT-5","features":"thinking,tools","input_modalities":"text,image","max_output":128000,"context_length":400000,"pricing":{"input":1.25,"output":10,"cache_read":0.125}},
		{"model_id":"tiny","model_name":"Tiny","input_modalities":"text","context_length":4096},
		{"model_id":"img","model_name":"Img","input_modalities":"image","context_length":100000}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/api/v1/models" || r.URL.Query().Get("type") != "llm" {
			t.Errorf("bad request %s", r.URL)
		}
	})
	got, err := AIHubMix{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "gpt-5" || m.Name != "GPT-5" || m.ContextWindow != 400000 || m.DefaultMaxTokens != 128000 ||
		m.Pricing.Input != 1.25 || m.Pricing.CacheHit != 0.125 || !m.Capabilities.Vision ||
		m.Reasoning.Thinking != catwalk.ThinkingToggleable {
		t.Fatalf("bad mapping %+v", m)
	}
}

func TestAIHubMixEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (AIHubMix{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestOpenRouterMapsAndFilters(t *testing.T) {
	body := `{"data":[
		{"id":"anthropic/claude-x","name":"Anthropic: X","context_length":200000,
		 "architecture":{"input_modalities":["text","image"],"output_modalities":["text"]},
		 "pricing":{"prompt":"0.000003","completion":"0.000015","input_cache_read":"0.0000003","input_cache_write":"0.00000375"},
		 "top_provider":{"context_length":200000,"max_completion_tokens":64000},
		 "supported_parameters":["tools","reasoning"]},
		{"id":"x/batch:batch","context_length":200000,
		 "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
		 "supported_parameters":["tools"]},
		{"id":"x/no-tools","context_length":200000,
		 "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
		 "supported_parameters":[]},
		{"id":"x/tiny","context_length":4000,
		 "architecture":{"input_modalities":["text"],"output_modalities":["text"]},
		 "supported_parameters":["tools"]},
		{"id":"x/image-out","context_length":200000,
		 "architecture":{"input_modalities":["text"],"output_modalities":["image"]},
		 "supported_parameters":["tools"]}
	]}`
	srv := serve(t, 200, body, func(r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := OpenRouter{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/api/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "anthropic/claude-x" || m.Name != "Anthropic: X" || m.ContextWindow != 200000 || m.DefaultMaxTokens != 32000 {
		t.Errorf("bad fields %+v", m)
	}
	if m.Pricing.Input != 3 || m.Pricing.Output != 15 || m.Pricing.CacheHit != 0.3 || m.Pricing.CacheCreate != 3.75 {
		t.Errorf("bad pricing %+v", m.Pricing)
	}
	if !m.Capabilities.Vision || m.Reasoning.Thinking != catwalk.ThinkingToggleable || m.Reasoning.DefaultEffortLevel != "medium" {
		t.Errorf("bad capabilities or reasoning %+v %+v", m.Capabilities, m.Reasoning)
	}
}

func TestOpenRouterEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (OpenRouter{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

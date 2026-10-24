package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestSyntheticKeepsToolTextModels(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"hf:a/b","name":"hf/Big-Model","input_modalities":["text","image"],"output_modalities":["text"],"context_length":100000,"max_output_length":8000,"supported_features":["tools","reasoning"],"pricing":{"prompt":"$0.00000055","completion":"$0.000002","input_cache_reads":"$0.0000001","input_cache_writes":"$0"}},
		{"id":"small","name":"small","input_modalities":["text"],"output_modalities":["text"],"context_length":8000,"supported_features":["tools"]},
		{"id":"notools","name":"notools","input_modalities":["text"],"output_modalities":["text"],"context_length":100000,"supported_features":[]},
		{"id":"img","name":"img","input_modalities":["image"],"output_modalities":["text"],"context_length":100000,"supported_features":["tools"]}]}`, func(r *http.Request) {
		if r.URL.Path != "/openai/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Synthetic{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/openai/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "hf:a/b" || m.Name != "Big Model" || m.ContextWindow != 100000 || m.DefaultMaxTokens != 4000 ||
		m.Pricing.Input != 0.55 || m.Pricing.Output != 2 || m.Pricing.CacheHit != 0.1 ||
		m.Reasoning.Thinking != catwalk.ThinkingToggleable || m.Reasoning.DefaultEffortLevel != "medium" || !m.Capabilities.Vision {
		t.Errorf("model = %+v", m)
	}
}

func TestSyntheticEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Synthetic{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

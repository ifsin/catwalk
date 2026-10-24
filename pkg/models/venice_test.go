package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestVeniceKeepsCallableTextModels(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"llama","type":"text","model_spec":{"name":"Llama","availableContextTokens":128000,"maxCompletionTokens":8000,"pricing":{"input":{"usd":0.7},"output":{"usd":2.8}},"capabilities":{"supportsFunctionCalling":true,"supportsReasoning":true,"supportsReasoningEffort":true,"supportsVision":true},"constraints":{"temperature":{"default":0.7},"top_p":{"default":0.95}}}},
		{"id":"offline","type":"text","model_spec":{"availableContextTokens":1000,"offline":true,"capabilities":{"supportsFunctionCalling":true}}},
		{"id":"beta","type":"text","model_spec":{"availableContextTokens":1000,"beta":true,"capabilities":{"supportsFunctionCalling":true}}},
		{"id":"nofn","type":"text","model_spec":{"availableContextTokens":1000,"capabilities":{}}},
		{"id":"image","type":"image","model_spec":{"availableContextTokens":1000,"capabilities":{"supportsFunctionCalling":true}}}]}`, func(r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Venice{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/api/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "llama" || m.Name != "Llama" || m.ContextWindow != 128000 || m.DefaultMaxTokens != 8000 || !m.Capabilities.Vision ||
		m.Pricing.Input != 0.7 || m.Pricing.Output != 2.8 || m.Reasoning.Thinking != catwalk.ThinkingToggleable ||
		len(m.Reasoning.EffortLevels) != 3 || m.Options.Temperature == nil || *m.Options.Temperature != 0.7 || m.Options.TopP == nil {
		t.Errorf("llama = %+v", m)
	}
}

func TestVeniceEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Venice{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

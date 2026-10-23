package models

import (
	"context"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestHyperReadsTheV2ProviderFromTheServiceRoot(t *testing.T) {
	srv := serve(t, 200, `{"id":"hyper","name":"Hyper","models":[
		{"id":"plain","name":"Plain","cost_per_1m_in":3.2,"cost_per_1m_out":16,"cost_per_1m_in_cached":0,"cost_per_1m_out_cached":0.3,"context_window":1000000,"default_max_tokens":16000,"can_reason":false,"supports_attachments":true},
		{"id":"effort","name":"Effort","context_window":1000,"can_reason":true,"reasoning_levels":["low","high"],"default_reasoning_effort":"high"},
		{"id":"always","name":"Always","can_reason":true}]}`, func(r *http.Request) {
		if r.URL.Path != "/api/v1/provider" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad request %s %v", r.URL, r.Header)
		}
	})
	got, err := Hyper{}.Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, BaseURL: srv.URL + "/v1", APIKey: "tok"})
	if err != nil || len(got) != 3 {
		t.Fatalf("got %+v, %v", got, err)
	}
	plain := got[0]
	if plain.Pricing.Input != 3.2 || plain.Pricing.Output != 16 || plain.Pricing.CacheHit != 0.3 || plain.ContextWindow != 1000000 ||
		plain.Reasoning.Thinking != catwalk.ThinkingNever || !plain.Capabilities.Vision {
		t.Errorf("plain = %+v", plain)
	}
	if e := got[1]; e.Reasoning.Thinking != catwalk.ThinkingToggleable || len(e.Reasoning.EffortLevels) != 2 || e.Reasoning.DefaultEffortLevel != "high" {
		t.Errorf("effort = %+v", e.Reasoning)
	}
	if got[2].Reasoning.Thinking != catwalk.ThinkingAlways {
		t.Errorf("always = %+v", got[2].Reasoning)
	}
}

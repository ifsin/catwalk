package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestVercelKeepsToolLanguageModels(t *testing.T) {
	srv := serve(t, 200, `{"object":"list","data":[
		{"id":"openai/gpt","name":"GPT","context_window":400000,"max_tokens":128000,"type":"language","tags":["tool-use","vision","reasoning"],"pricing":{"input":"0.00000125","output":"0.00001","input_cache_read":"0.000000125"}},
		{"id":"plain","name":"Plain","context_window":1000,"type":"language","tags":["tool-use"]},
		{"id":"notools","type":"language","tags":["vision"]},
		{"id":"embed","type":"embedding","tags":["tool-use"]}]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Vercel{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "openai/gpt" || m.Name != "GPT" || m.ContextWindow != 400000 || m.DefaultMaxTokens != 128000 || !m.Capabilities.Vision ||
		m.Pricing.Input != 1.25 || m.Pricing.Output != 10 || m.Pricing.CacheHit != 0.125 || m.Reasoning.Thinking != catwalk.ThinkingToggleable {
		t.Errorf("gpt = %+v", m)
	}
	if p := got[1]; p.DefaultMaxTokens != 100 || p.Reasoning.Thinking != catwalk.ThinkingNever {
		t.Errorf("plain = %+v", p)
	}
}

func TestVercelEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Vercel{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

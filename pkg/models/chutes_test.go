package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestChutesMapsAndFilters(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"moonshotai/Kimi-K2.6-TEE","context_length":262144,"max_output_length":65536,"input_modalities":["text","image"],"output_modalities":["text"],"supported_features":["tools","reasoning"],"pricing":{"prompt":0.45,"completion":2.2,"input_cache_read":0.09}},
		{"id":"image-gen","context_length":1000,"input_modalities":["text"],"output_modalities":["image"],"supported_features":["tools"]},
		{"id":"no-tools","context_length":1000,"input_modalities":["text"],"output_modalities":["text"]}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Chutes{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "moonshotai/Kimi-K2.6-TEE" || m.Name != m.ID || m.ContextWindow != 262144 || m.DefaultMaxTokens != 65536 ||
		m.Pricing.Output != 2.2 || !m.Capabilities.Vision || m.Reasoning.Thinking != catwalk.ThinkingToggleable {
		t.Fatalf("bad mapping %+v", m)
	}
}

func TestChutesEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Chutes{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

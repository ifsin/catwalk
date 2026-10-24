package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestPioneerKeepsInferenceChatModels(t *testing.T) {
	srv := serve(t, 200, `{"models":[
		{"id":"chat","label":"Chat","context_window":32000,"max_output_tokens":4000,"input_price_per_million":1.5,"output_price_per_million":2,"cache_read_price_per_million":0.1,"cache_write_price_per_million":0.2,"task_type":"decoder","is_chat_model":true,"supports_inference":true,"supports_image_input":true},
		{"id":"embed","task_type":"encoder","is_chat_model":true,"supports_inference":true},
		{"id":"old","task_type":"decoder","is_chat_model":true,"supports_inference":true,"deprecated":true},
		{"id":"","task_type":"decoder","is_chat_model":true,"supports_inference":true}]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/base-models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := Pioneer{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "chat" || m.Name != "Chat" || m.ContextWindow != 32000 || m.DefaultMaxTokens != 4000 || !m.Capabilities.Vision ||
		m.Pricing.Input != 1.5 || m.Pricing.Output != 2 || m.Pricing.CacheHit != 0.1 || m.Pricing.CacheCreate != 0.2 {
		t.Errorf("chat = %+v", m)
	}
}

func TestPioneerEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"models":[]}`, nil)
	if _, err := (Pioneer{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

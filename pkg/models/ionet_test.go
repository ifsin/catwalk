package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestIoNetMapsAndFilters(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"moonshotai/Kimi-K2.5","name":"Kimi K2.5","max_tokens":4000,"context_window":200000,"supports_images_input":true,"input_token_price":0.0000005,"output_token_price":0.000002},
		{"id":"moonshotai/Kimi-K2.5","name":"dup"},
		{"id":"meta-llama/Llama-3.2-3B","context_window":8000},
		{"id":"mistral-nemo-x","context_window":8000}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/api/v1/models" {
			t.Errorf("path %s", r.URL.Path)
		}
	})
	got, err := IoNet{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/api/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "moonshotai/Kimi-K2.5" || m.Name != "Kimi K2.5" || !m.Capabilities.Vision || m.ContextWindow != 200000 || m.DefaultMaxTokens != 400 || m.Pricing.Input != 0.5 || m.Pricing.Output != 2 {
		t.Errorf("got %+v", m)
	}
}

func TestIoNetEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (IoNet{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

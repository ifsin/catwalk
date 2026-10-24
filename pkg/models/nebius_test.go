package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestNebiusKeepsToolModels(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"moonshotai/Kimi-K2.5","name":"Kimi K2.5","context_length":131072,"supported_features":["tools","reasoning"],"pricing":{"prompt":"0.0000005","completion":"0.0000025"},"architecture":{"modality":"text+image->text"}},
		{"id":"embed","name":"Embed","context_length":8192,"supported_features":["embeddings"]}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" || r.URL.Query().Get("verbose") != "true" {
			t.Errorf("request %s", r.URL)
		}
	})
	got, err := Nebius{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "moonshotai/Kimi-K2.5" || m.Name != "Kimi K2.5" || !m.Capabilities.Vision || !m.CanReason() || m.ContextWindow != 131072 || m.Pricing.Input != 0.5 || m.Pricing.Output != 2.5 {
		t.Errorf("got %+v", m)
	}
}

func TestNebiusEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Nebius{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestNeuralwattKeepsLargeToolModels(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"glm-5.3","max_model_len":131072,"metadata":{"display_name":"GLM 5.3","pricing":{"input_per_million":0.6,"output_per_million":2.2},"capabilities":{"tools":true,"vision":true,"reasoning":true,"reasoning_effort":true},"limits":{"max_output_tokens":8192}}},
		{"id":"small","max_model_len":8192,"metadata":{"capabilities":{"tools":true}}},
		{"id":"old","max_model_len":131072,"metadata":{"deprecated":true,"capabilities":{"tools":true}}},
		{"id":"no-tools","max_model_len":131072,"metadata":{"capabilities":{"tools":false}}}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path %s", r.URL.Path)
		}
	})
	got, err := Neuralwatt{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "glm-5.3" || m.Name != "GLM 5.3" || !m.Capabilities.Vision || !m.CanReason() || m.DefaultMaxTokens != 8192 || m.Pricing.Output != 2.2 {
		t.Errorf("got %+v", m)
	}
}

func TestNeuralwattEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (Neuralwatt{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestAtlasCloudMapsAndFilters(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"zai/glm","name":"GLM","input_modalities":["text","image"],"output_modalities":["text"],"context_length":200000,"max_output_length":65536,"supported_features":["tools"],"discount_to_user":0.5,
		 "pricing":[{"prompt":"$0.000001","completion":"$0.000004","input_cache_read":"$0.0000002"}]},
		{"id":"not-ready","is_ready":false,"input_modalities":["text"],"output_modalities":["text"],"context_length":200000,"supported_features":["tools"],"pricing":{}},
		{"id":"no-tools","input_modalities":["text"],"output_modalities":["text"],"context_length":200000,"pricing":{}}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("bad path %s", r.URL.Path)
		}
	})
	got, err := AtlasCloud{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "zai/glm" || m.Name != "GLM" || m.DefaultMaxTokens != 20000 || m.Pricing.Input != 0.5 || m.Pricing.Output != 2 || !m.Capabilities.Vision {
		t.Fatalf("bad mapping %+v", m)
	}
}

func TestAtlasCloudEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (AtlasCloud{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

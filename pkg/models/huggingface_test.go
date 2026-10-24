package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestHuggingFaceOneEntryPerLiveToolBackend(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"org/m","providers":[
			{"provider":"groq","status":"live","context_length":8000,"supports_tools":true,"pricing":{"input":0.5,"output":1}},
			{"provider":"novita","status":"live","context_length":8000,"supports_tools":true},
			{"provider":"cerebras","status":"error","context_length":8000,"supports_tools":true},
			{"provider":"fireworks-ai","status":"live","supports_tools":false}
		]},
		{"id":"org/no-context","providers":[{"provider":"groq","status":"live","supports_tools":true}]}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path %s", r.URL.Path)
		}
	})
	got, err := HuggingFace{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "org/m:groq" || m.Name != "org/m (groq)" || m.ContextWindow != 8000 || m.DefaultMaxTokens != 2000 || m.Pricing.Input != 0.5 {
		t.Errorf("got %+v", m)
	}
}

func TestHuggingFaceEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[]}`, nil)
	if _, err := (HuggingFace{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

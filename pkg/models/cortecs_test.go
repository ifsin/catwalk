package models

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestCortecsKeepsToolModels(t *testing.T) {
	srv := serve(t, 200, `{"data":[
		{"id":"qwen","context_size":100000,"tags":["Tools","Reasoning"]},
		{"id":"vision","context_size":8000,"tags":["Tools","Image"]},
		{"id":"chat-only","context_size":8000,"tags":["Image"]}
	]}`, func(r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path %s", r.URL.Path)
		}
	})
	got, err := Cortecs{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	q := got[0]
	if q.ID != "qwen" || q.Name != "qwen" || q.ContextWindow != 100000 || q.DefaultMaxTokens != 10000 || !q.CanReason() {
		t.Errorf("qwen: %+v", q)
	}
	if v := got[1]; v.ID != "vision" || !v.Capabilities.Vision || v.CanReason() {
		t.Errorf("vision: %+v", v)
	}
}

func TestCortecsEmptyIsRejected(t *testing.T) {
	srv := serve(t, 200, `{"data":[{"id":"x","tags":[]}]}`, nil)
	if _, err := (Cortecs{}).Models(context.Background(), catwalk.Source{BaseURL: srv.URL + "/v1"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("got %v", err)
	}
}

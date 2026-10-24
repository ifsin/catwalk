package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestModelsKeepPickerChatModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad auth %v", r.Header)
		}
		_, _ = w.Write([]byte(`{"data":[
			{"id":"gpt-4.1","name":"GPT-4.1","model_picker_enabled":true,"capabilities":{"type":"chat","limits":{"max_context_window_tokens":128000,"max_output_tokens":16384},"supports":{"vision":true}},"supported_endpoints":["/chat/completions"]},
			{"id":"gpt-5","name":"GPT-5","model_picker_enabled":true,"capabilities":{"type":"chat"},"supported_endpoints":["/responses"]},
			{"id":"embed","model_picker_enabled":true,"capabilities":{"type":"embeddings"}},
			{"id":"off","model_picker_enabled":false,"capabilities":{"type":"chat"}}]}`))
	}))
	defer srv.Close()
	got, err := Copilot{}.Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, BaseURL: srv.URL, APIKey: "tok"})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if got[0].ContextWindow != 128000 || got[0].DefaultMaxTokens != 16384 || !got[0].Capabilities.Vision || got[0].Type != "" {
		t.Fatalf("bad %+v", got[0])
	}
	if got[1].Type != catwalk.TypeResponses {
		t.Fatalf("bad %+v", got[1])
	}
}

func TestModelsDropOnlyDisabledPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[
			{"id":"on","model_picker_enabled":true,"policy":{"state":"enabled"},"capabilities":{"type":"chat"}},
			{"id":"unset","model_picker_enabled":true,"policy":{"state":"unconfigured"},"capabilities":{"type":"chat"}},
			{"id":"blocked","model_picker_enabled":true,"policy":{"state":"disabled"},"capabilities":{"type":"chat"}},
			{"id":"nopolicy","model_picker_enabled":true,"capabilities":{"type":"chat"}}]}`))
	}))
	defer srv.Close()
	got, err := Copilot{}.Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, BaseURL: srv.URL, APIKey: "tok"})
	if err != nil || len(got) != 3 || got[0].ID != "on" || got[1].ID != "unset" || got[2].ID != "nopolicy" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

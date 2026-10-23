package models

import (
	"context"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

const codexBody = `{"models":[
	{"slug":"gpt-5.1-codex","display_name":"GPT-5.1 Codex","visibility":"list","context_window":272000,"default_reasoning_level":"medium","supported_reasoning_levels":[{"effort":"low"},{"effort":"medium"},{"effort":"high"}]},
	{"slug":"gpt-5.1-codex-mini","display_name":"GPT-5.1 Codex Mini","visibility":"list","context_window":272000},
	{"slug":"internal-model","display_name":"Internal","visibility":"hide","context_window":100}]}`

func TestOpenAISignInListsCodex(t *testing.T) {
	srv := serve(t, 200, codexBody, func(r *http.Request) {
		if r.URL.Path != "/models" || r.URL.Query().Get("client_version") != codexClientVersion ||
			r.Header.Get("Authorization") != "Bearer at" || r.Header.Get("chatgpt-account-id") != "acct" {
			t.Errorf("bad request %s %v", r.URL, r.Header)
		}
	})
	got, err := OpenAI{}.Models(context.Background(), catwalk.Source{
		Auth: catwalk.AuthSignIn, BaseURL: srv.URL, APIKey: "at",
		Headers: map[string]string{"chatgpt-account-id": "acct"},
	})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	m := got[0]
	if m.ID != "gpt-5.1-codex" || m.Name != "GPT-5.1 Codex" || m.ContextWindow != 272000 || m.Reasoning.Thinking != catwalk.ThinkingToggleable ||
		len(m.Reasoning.EffortLevels) != 3 || m.Reasoning.DefaultEffortLevel != "medium" {
		t.Fatalf("bad model %+v", m)
	}
	if got[1].Reasoning.Thinking != catwalk.ThinkingNever || len(got[1].Reasoning.EffortLevels) != 0 {
		t.Fatalf("mini should not reason: %+v", got[1])
	}
}

func TestOpenAIAPIKeyReturnsEmbedded(t *testing.T) {
	got, err := OpenAI{}.Models(context.Background(), catwalk.Source{Auth: catwalk.AuthAPIKey, BaseURL: "http://unused.invalid", APIKey: "k"})
	if err != nil || len(got) == 0 {
		t.Fatalf("got %d models, %v", len(got), err)
	}
}

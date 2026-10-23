package models

import (
	"context"
	"slices"

	"charm.land/catwalk/pkg/catwalk"
)

type copilotList struct {
	Data []struct {
		ID                 string   `json:"id"`
		Name               string   `json:"name"`
		ModelPickerEnabled bool     `json:"model_picker_enabled"`
		SupportedEndpoints []string `json:"supported_endpoints"`
		Capabilities       struct {
			Type   string `json:"type"`
			Limits struct {
				MaxContextWindowTokens int64 `json:"max_context_window_tokens"`
				MaxOutputTokens        int64 `json:"max_output_tokens"`
			} `json:"limits"`
			Supports struct {
				Vision bool `json:"vision"`
			} `json:"supports"`
		} `json:"capabilities"`
	} `json:"data"`
}

// Copilot lists the chat models enabled for a Copilot account.
type Copilot struct{}

var _ catwalk.Lister = Copilot{}

// Models implements Lister. Only chat models enabled in the model
// picker of the account are returned.
func (Copilot) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	if s.Auth != catwalk.AuthSignIn || s.APIKey == "" {
		return nil, ErrNotSignedIn
	}
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list copilotList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || !m.ModelPickerEnabled || m.Capabilities.Type != "chat" {
			continue
		}
		model := catwalk.Model{
			ID:               m.ID,
			Name:             FirstNonEmpty(m.Name, m.ID),
			ContextWindow:    m.Capabilities.Limits.MaxContextWindowTokens,
			DefaultMaxTokens: m.Capabilities.Limits.MaxOutputTokens,
			Reasoning:        catwalk.Reasoning{Thinking: catwalk.ThinkingNever},
			Capabilities:     catwalk.Capabilities{Vision: m.Capabilities.Supports.Vision},
		}
		if slices.Contains(m.SupportedEndpoints, "/responses") && !slices.Contains(m.SupportedEndpoints, "/chat/completions") {
			model.Type = catwalk.TypeResponses
		}
		out = append(out, model)
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderCopilot, Copilot{})
}

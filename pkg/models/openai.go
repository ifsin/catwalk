package models

import (
	"context"
	"errors"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/catwalk/pkg/embedded"
)

// codexClientVersion is the client version the Codex backend requires on its
// model list. The backend gates models by the features of the reported
// client, and a high version asks for the full list the plan grants.
const codexClientVersion = "999.0.0"

// OpenAI lists the models of an OpenAI account. A ChatGPT sign-in is listed
// from the Codex backend. An API key has no list endpoint here yet, so it
// returns the embedded catalog.
type OpenAI struct{}

var _ catwalk.Lister = OpenAI{}

type codexList struct {
	Models []struct {
		Slug                     string `json:"slug"`
		DisplayName              string `json:"display_name"`
		Visibility               string `json:"visibility"`
		ContextWindow            int64  `json:"context_window"`
		DefaultReasoningLevel    string `json:"default_reasoning_level"`
		SupportedReasoningLevels []struct {
			Effort string `json:"effort"`
		} `json:"supported_reasoning_levels"`
	} `json:"models"`
}

// Models implements Lister.
func (OpenAI) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	if s.Auth != catwalk.AuthSignIn {
		return embeddedModels(catwalk.InferenceProviderOpenAI)
	}
	url, err := Join(s, "/models?client_version="+codexClientVersion)
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list codexList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Models))
	for _, m := range list.Models {
		if m.Slug == "" || m.Visibility != "list" {
			continue
		}
		var levels []string
		for _, l := range m.SupportedReasoningLevels {
			if l.Effort != "" {
				levels = append(levels, l.Effort)
			}
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if len(levels) > 0 {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels(levels...),
				DefaultEffortLevel: m.DefaultReasoningLevel,
			}
		}
		out = append(out, catwalk.Model{
			ID:            m.Slug,
			Name:          FirstNonEmpty(m.DisplayName, m.Slug),
			ContextWindow: m.ContextWindow,
			Reasoning:     reasoning,
			Capabilities:  catwalk.Capabilities{Vision: true},
		})
	}
	return NonEmpty(out)
}

func embeddedModels(id catwalk.InferenceProvider) ([]catwalk.Model, error) {
	for _, p := range embedded.GetAll() {
		if p.ID == id {
			return NonEmpty(p.Models)
		}
	}
	return nil, errors.New("models: provider is not in the embedded catalog")
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderOpenAI, OpenAI{})
}

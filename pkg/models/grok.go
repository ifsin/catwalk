package models

import (
	"context"
	"slices"

	"charm.land/catwalk/pkg/catwalk"
)

type grokList struct {
	Models []struct {
		ID               string   `json:"id"`
		InputModalities  []string `json:"input_modalities"`
		OutputModalities []string `json:"output_modalities"`
	} `json:"models"`
}

// Grok lists the text models of an xAI account.
type Grok struct{}

var _ catwalk.Lister = Grok{}

// Models implements Lister. Only models that produce text are
// returned.
func (Grok) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/language-models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list grokList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Models))
	for _, m := range list.Models {
		if m.ID == "" {
			continue
		}
		if len(m.OutputModalities) > 0 && !slices.Contains(m.OutputModalities, "text") {
			continue
		}
		out = append(out, catwalk.Model{
			ID:           m.ID,
			Name:         m.ID,
			Reasoning:    catwalk.Reasoning{Thinking: catwalk.ThinkingNever},
			Capabilities: catwalk.Capabilities{Vision: slices.Contains(m.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderXAI, Grok{})
}

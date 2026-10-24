package models

import (
	"context"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type cortecsList struct {
	Data []struct {
		ID          string   `json:"id"`
		ContextSize int64    `json:"context_size"`
		Tags        []string `json:"tags"`
	} `json:"data"`
}

// Cortecs lists the tool-calling models of a Cortecs account.
type Cortecs struct{}

var _ catwalk.Lister = Cortecs{}

// Models implements Lister. Only models tagged Tools are returned. Pricing
// is not part of the list response, so it is left unset.
func (Cortecs) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list cortecsList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || !hasTag(m.Tags, "Tools") {
			continue
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if hasTag(m.Tags, "Reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}
		out = append(out, catwalk.Model{
			ID:               m.ID,
			Name:             m.ID,
			ContextWindow:    m.ContextSize,
			DefaultMaxTokens: m.ContextSize / 10,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: hasTag(m.Tags, "Image")},
		})
	}
	return NonEmpty(out)
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderCortecs, Cortecs{})
}

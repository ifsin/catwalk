package models

import (
	"context"
	"math"
	"slices"

	"charm.land/catwalk/pkg/catwalk"
)

type chutesList struct {
	Data []struct {
		ID                string   `json:"id"`
		ContextLength     int64    `json:"context_length"`
		MaxOutputLength   int64    `json:"max_output_length"`
		InputModalities   []string `json:"input_modalities"`
		OutputModalities  []string `json:"output_modalities"`
		SupportedFeatures []string `json:"supported_features"`
		Pricing           struct {
			Prompt         float64 `json:"prompt"`
			Completion     float64 `json:"completion"`
			InputCacheRead float64 `json:"input_cache_read"`
		} `json:"pricing"`
	} `json:"data"`
}

// Chutes lists the models of Chutes. Only text models that support tools are
// returned.
type Chutes struct{}

var _ catwalk.Lister = Chutes{}

// Models implements Lister.
func (Chutes) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list chutesList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || !slices.Contains(m.SupportedFeatures, "tools") {
			continue
		}
		if !slices.Contains(m.InputModalities, "text") || !slices.Contains(m.OutputModalities, "text") {
			continue
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if slices.Contains(m.SupportedFeatures, "reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: m.ID,
			Pricing: catwalk.Pricing{
				Input:    math.Round(m.Pricing.Prompt*1e5) / 1e5,
				Output:   math.Round(m.Pricing.Completion*1e5) / 1e5,
				CacheHit: math.Round(m.Pricing.InputCacheRead*1e5) / 1e5,
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: m.MaxOutputLength,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: slices.Contains(m.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderChutes, Chutes{})
}

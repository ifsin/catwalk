package models

import (
	"context"
	"math"

	"charm.land/catwalk/pkg/catwalk"
)

type avianList struct {
	Data []struct {
		ID            string `json:"id"`
		DisplayName   string `json:"display_name"`
		ContextLength int64  `json:"context_length"`
		MaxOutput     int64  `json:"max_output"`
		Reasoning     bool   `json:"reasoning"`
		Pricing       struct {
			Input     float64 `json:"input_per_million"`
			Output    float64 `json:"output_per_million"`
			CacheRead float64 `json:"cache_read_per_million"`
		} `json:"pricing"`
	} `json:"data"`
}

// Avian lists the models of Avian.
type Avian struct{}

var _ catwalk.Lister = Avian{}

// Models implements Lister.
func (Avian) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list avianList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" {
			continue
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if m.Reasoning {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(m.DisplayName, m.ID),
			Pricing: catwalk.Pricing{
				Input:    math.Round(m.Pricing.Input*1e5) / 1e5,
				Output:   math.Round(m.Pricing.Output*1e5) / 1e5,
				CacheHit: math.Round(m.Pricing.CacheRead*1e5) / 1e5,
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: m.MaxOutput,
			Reasoning:        reasoning,
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderAvian, Avian{})
}

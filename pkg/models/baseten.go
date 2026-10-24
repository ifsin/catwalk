package models

import (
	"context"
	"math"
	"slices"
	"strconv"

	"charm.land/catwalk/pkg/catwalk"
)

type basetenList struct {
	Data []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		ContextLength int64  `json:"context_length"`
		MaxCompletion int64  `json:"max_completion_tokens"`
		Pricing       struct {
			Prompt         string `json:"prompt"`
			Completion     string `json:"completion"`
			InputCacheRead string `json:"input_cache_read"`
		} `json:"pricing"`
		Features        []string `json:"supported_features"`
		InputModalities []string `json:"input_modalities"`
	} `json:"data"`
}

// Baseten lists the models of Baseten Model APIs. Only models that support
// tools and take text input are returned.
type Baseten struct{}

var _ catwalk.Lister = Baseten{}

// Models implements Lister.
func (Baseten) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list basetenList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || !slices.Contains(m.Features, "tools") || !slices.Contains(m.InputModalities, "text") {
			continue
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if slices.Contains(m.Features, "reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(m.Name, m.ID),
			Pricing: catwalk.Pricing{
				Input:    basetenPerMillion(m.Pricing.Prompt),
				Output:   basetenPerMillion(m.Pricing.Completion),
				CacheHit: basetenPerMillion(m.Pricing.InputCacheRead),
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: m.MaxCompletion,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: slices.Contains(m.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

// basetenPerMillion converts a per-token price string to a price per million
// tokens. An unreadable price is zero.
func basetenPerMillion(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return math.Round(v*1e6*1e5) / 1e5
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderBaseten, Baseten{})
}

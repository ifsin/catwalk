package models

import (
	"context"
	"math"
	"slices"
	"strconv"

	"charm.land/catwalk/pkg/catwalk"
)

type vercelList struct {
	Data []struct {
		ID            string   `json:"id"`
		Name          string   `json:"name"`
		ContextWindow int64    `json:"context_window"`
		MaxTokens     int64    `json:"max_tokens"`
		Type          string   `json:"type"`
		Tags          []string `json:"tags"`
		Pricing       struct {
			Input           string `json:"input"`
			Output          string `json:"output"`
			InputCacheRead  string `json:"input_cache_read"`
			InputCacheWrite string `json:"input_cache_write"`
		} `json:"pricing"`
	} `json:"data"`
}

// Vercel lists the language models of the AI Gateway that support tool use.
type Vercel struct{}

var _ catwalk.Lister = Vercel{}

// Models implements Lister.
func (Vercel) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list vercelList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || m.Type != "language" || !slices.Contains(m.Tags, "tool-use") {
			continue
		}

		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if slices.Contains(m.Tags, "reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}

		maxTokens := m.MaxTokens
		if maxTokens == 0 {
			maxTokens = m.ContextWindow / 10
		}

		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(m.Name, m.ID),
			Pricing: catwalk.Pricing{
				Input:       vercelPerMillion(m.Pricing.Input),
				Output:      vercelPerMillion(m.Pricing.Output),
				CacheCreate: vercelPerMillion(m.Pricing.InputCacheWrite),
				CacheHit:    vercelPerMillion(m.Pricing.InputCacheRead),
			},
			ContextWindow:    m.ContextWindow,
			DefaultMaxTokens: maxTokens,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: slices.Contains(m.Tags, "vision")},
		})
	}
	return NonEmpty(out)
}

// vercelPerMillion parses a per-token price string into dollars per million
// tokens. A missing or invalid price is zero.
func vercelPerMillion(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return math.Round(v*1e11) / 1e5
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderVercel, Vercel{})
}

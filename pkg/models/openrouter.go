package models

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type openRouterList struct {
	Data []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		ContextLength int64  `json:"context_length"`
		Architecture  struct {
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"architecture"`
		Pricing struct {
			Prompt          string `json:"prompt"`
			Completion      string `json:"completion"`
			InputCacheRead  string `json:"input_cache_read"`
			InputCacheWrite string `json:"input_cache_write"`
		} `json:"pricing"`
		TopProvider struct {
			ContextLength       int64  `json:"context_length"`
			MaxCompletionTokens *int64 `json:"max_completion_tokens"`
		} `json:"top_provider"`
		SupportedParams []string `json:"supported_parameters"`
	} `json:"data"`
}

// OpenRouter lists the models of OpenRouter. The list carries the pricing,
// context and modalities of each model, so no per-model request is made.
type OpenRouter struct{}

var _ catwalk.Lister = OpenRouter{}

// Models implements Lister. Only text models that take tools and have a
// context window of at least 20000 tokens are returned.
func (OpenRouter) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list openRouterList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || strings.HasSuffix(m.ID, ":batch") || m.ContextLength < 20000 {
			continue
		}
		if !slices.Contains(m.SupportedParams, "tools") ||
			!slices.Contains(m.Architecture.InputModalities, "text") ||
			!slices.Contains(m.Architecture.OutputModalities, "text") {
			continue
		}

		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if slices.Contains(m.SupportedParams, "reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}

		contextWindow := m.ContextLength
		if m.TopProvider.ContextLength > 0 {
			contextWindow = m.TopProvider.ContextLength
		}
		maxTokens := m.ContextLength / 10
		if m.TopProvider.MaxCompletionTokens != nil {
			maxTokens = *m.TopProvider.MaxCompletionTokens / 2
		}

		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(m.Name, m.ID),
			Pricing: catwalk.Pricing{
				Input:       perMillion(m.Pricing.Prompt),
				Output:      perMillion(m.Pricing.Completion),
				CacheCreate: perMillion(m.Pricing.InputCacheWrite),
				CacheHit:    perMillion(m.Pricing.InputCacheRead),
			},
			ContextWindow:    contextWindow,
			DefaultMaxTokens: maxTokens,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: slices.Contains(m.Architecture.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

// perMillion converts a per-token price string to dollars per 1M tokens,
// rounded to five decimals. A price that does not parse is zero.
func perMillion(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return math.Round(v*1e6*1e5) / 1e5
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderOpenRouter, OpenRouter{})
}

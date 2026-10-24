package models

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type syntheticList struct {
	Data []struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		InputModalities   []string `json:"input_modalities"`
		OutputModalities  []string `json:"output_modalities"`
		ContextLength     int64    `json:"context_length"`
		MaxOutputLength   int64    `json:"max_output_length"`
		SupportedFeatures []string `json:"supported_features"`
		Pricing           struct {
			Prompt           string `json:"prompt"`
			Completion       string `json:"completion"`
			InputCacheReads  string `json:"input_cache_reads"`
			InputCacheWrites string `json:"input_cache_writes"`
		} `json:"pricing"`
	} `json:"data"`
}

// Synthetic lists the tool-calling text models of Synthetic.
type Synthetic struct{}

var _ catwalk.Lister = Synthetic{}

// Models implements Lister. Models with a context window under 20k tokens,
// without text input and output, or without tool support are dropped.
func (Synthetic) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list syntheticList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || m.ContextLength < 20000 {
			continue
		}
		if !slices.Contains(m.InputModalities, "text") || !slices.Contains(m.OutputModalities, "text") {
			continue
		}
		if !slices.Contains(m.SupportedFeatures, "tools") {
			continue
		}

		// Drop the provider prefix and use spaces for hyphens, as the
		// generated catalog does.
		name := m.Name
		if idx := strings.Index(name, "/"); idx != -1 {
			name = name[idx+1:]
		}
		name = strings.ReplaceAll(name, "-", " ")

		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if slices.Contains(m.SupportedFeatures, "reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}

		// Cap the default output at 15% of the context, as the catalog does.
		maxTokens := m.ContextLength / 10
		if half := m.MaxOutputLength / 2; m.MaxOutputLength > 0 && half <= m.ContextLength*15/100 {
			maxTokens = half
		}

		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(name, m.ID),
			Pricing: catwalk.Pricing{
				Input:       syntheticPerMillion(m.Pricing.Prompt),
				Output:      syntheticPerMillion(m.Pricing.Completion),
				CacheCreate: syntheticPerMillion(m.Pricing.InputCacheWrites),
				CacheHit:    syntheticPerMillion(m.Pricing.InputCacheReads),
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: maxTokens,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: slices.Contains(m.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

// syntheticPerMillion parses a "$0.00000055" per-token price into dollars per
// million tokens.
func syntheticPerMillion(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimPrefix(s, "$"), 64)
	if err != nil {
		return 0
	}
	return math.Round(v*1e11) / 1e5
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderSynthetic, Synthetic{})
}

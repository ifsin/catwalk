package models

import (
	"context"
	"math"

	"charm.land/catwalk/pkg/catwalk"
)

type neuralwattList struct {
	Data []struct {
		ID          string `json:"id"`
		MaxModelLen int64  `json:"max_model_len"`
		Metadata    struct {
			DisplayName string `json:"display_name"`
			Pricing     struct {
				InputPerMillion        *float64 `json:"input_per_million"`
				OutputPerMillion       *float64 `json:"output_per_million"`
				CachedInputPerMillion  *float64 `json:"cached_input_per_million"`
				CachedOutputPerMillion *float64 `json:"cached_output_per_million"`
			} `json:"pricing"`
			Capabilities struct {
				Tools           bool `json:"tools"`
				Vision          bool `json:"vision"`
				Reasoning       bool `json:"reasoning"`
				ReasoningEffort bool `json:"reasoning_effort"`
			} `json:"capabilities"`
			Limits struct {
				MaxOutputTokens *int64 `json:"max_output_tokens"`
			} `json:"limits"`
			Deprecated bool `json:"deprecated"`
		} `json:"metadata"`
	} `json:"data"`
}

// Neuralwatt lists the tool-calling models of a Neuralwatt account.
type Neuralwatt struct{}

var _ catwalk.Lister = Neuralwatt{}

// Models implements Lister. Deprecated models, models without tool calling
// and models with a context window under 20k tokens are dropped.
func (Neuralwatt) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list neuralwattList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	round := func(v *float64) float64 {
		var f float64
		if v != nil {
			f = *v
		}
		return math.Round(f*1e5) / 1e5
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		meta := m.Metadata
		if m.ID == "" || meta.Deprecated || m.MaxModelLen < 20000 || !meta.Capabilities.Tools {
			continue
		}
		name := meta.DisplayName
		if name == "" {
			name = m.ID
		}
		maxTokens := m.MaxModelLen / 10
		if meta.Limits.MaxOutputTokens != nil {
			maxTokens = *meta.Limits.MaxOutputTokens
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if meta.Capabilities.Reasoning {
			reasoning.Thinking = catwalk.ThinkingToggleable
			if meta.Capabilities.ReasoningEffort {
				reasoning.EffortLevels = catwalk.NewEffortLevels("low", "medium", "high")
				reasoning.DefaultEffortLevel = "medium"
			}
		}
		p := meta.Pricing
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: name,
			Pricing: catwalk.Pricing{
				Input:       round(p.InputPerMillion),
				Output:      round(p.OutputPerMillion),
				CacheCreate: round(p.CachedInputPerMillion),
				CacheHit:    round(p.CachedOutputPerMillion),
			},
			ContextWindow:    m.MaxModelLen,
			DefaultMaxTokens: maxTokens,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: meta.Capabilities.Vision},
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderNeuralwatt, Neuralwatt{})
}

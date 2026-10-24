package models

import (
	"context"
	"math"
	"slices"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type aihubmixList struct {
	Data []struct {
		ModelID   string `json:"model_id"`
		ModelName string `json:"model_name"`
		Pricing   struct {
			Input      float64 `json:"input"`
			Output     float64 `json:"output"`
			CacheRead  float64 `json:"cache_read"`
			CacheWrite float64 `json:"cache_write"`
		} `json:"pricing"`
		Features        string `json:"features"`
		InputModalities string `json:"input_modalities"`
		MaxOutput       int64  `json:"max_output"`
		ContextLength   int64  `json:"context_length"`
	} `json:"data"`
}

// AIHubMix lists the LLMs of AIHubMix. The list is served from the API host,
// outside the /v1 base URL requests are sent to.
type AIHubMix struct{}

var _ catwalk.Lister = AIHubMix{}

// Models implements Lister. Models with a short context window or without
// text input are dropped.
func (AIHubMix) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	s.BaseURL = strings.TrimSuffix(strings.TrimRight(s.BaseURL, "/"), "/v1")
	url, err := Join(s, "/api/v1/models?type=llm")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list aihubmixList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	has := func(csv, field string) bool {
		return slices.ContainsFunc(strings.Split(csv, ","), func(f string) bool {
			return strings.TrimSpace(f) == field
		})
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ModelID == "" || m.ContextLength < 20000 || !has(m.InputModalities, "text") {
			continue
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if has(m.Features, "thinking") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}
		maxTokens := m.ContextLength / 10
		if m.MaxOutput != 0 && m.MaxOutput <= m.ContextLength/2 {
			maxTokens = m.MaxOutput
		}
		out = append(out, catwalk.Model{
			ID:   m.ModelID,
			Name: FirstNonEmpty(m.ModelName, m.ModelID),
			Pricing: catwalk.Pricing{
				Input:       math.Round(m.Pricing.Input*1e5) / 1e5,
				Output:      math.Round(m.Pricing.Output*1e5) / 1e5,
				CacheCreate: math.Round(m.Pricing.CacheWrite*1e5) / 1e5,
				CacheHit:    math.Round(m.Pricing.CacheRead*1e5) / 1e5,
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: maxTokens,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: has(m.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceAIHubMix, AIHubMix{})
}

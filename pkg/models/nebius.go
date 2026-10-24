package models

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type nebiusList struct {
	Data []struct {
		ID                string   `json:"id"`
		Name              string   `json:"name"`
		ContextLength     int64    `json:"context_length"`
		SupportedFeatures []string `json:"supported_features"`
		Pricing           struct {
			Prompt     string `json:"prompt"`
			Completion string `json:"completion"`
		} `json:"pricing"`
		Architecture struct {
			Modality string `json:"modality"`
		} `json:"architecture"`
	} `json:"data"`
}

// Nebius lists the tool-calling models of a Nebius Token Factory account.
type Nebius struct{}

var _ catwalk.Lister = Nebius{}

// Models implements Lister. Only models that support tools are returned.
func (Nebius) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models?verbose=true")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list nebiusList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	perMillion := func(price string) float64 {
		v, _ := strconv.ParseFloat(price, 64)
		return math.Round(v*1_000_000*100) / 100
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || !hasFeature(m.SupportedFeatures, "tools") {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if hasFeature(m.SupportedFeatures, "reasoning") {
			reasoning = catwalk.Reasoning{
				Thinking:           catwalk.ThinkingToggleable,
				EffortLevels:       catwalk.NewEffortLevels("low", "medium", "high"),
				DefaultEffortLevel: "medium",
			}
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: name,
			Pricing: catwalk.Pricing{
				Input:  perMillion(m.Pricing.Prompt),
				Output: perMillion(m.Pricing.Completion),
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: m.ContextLength / 10,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: strings.Contains(strings.ToLower(m.Architecture.Modality), "image")},
		})
	}
	return NonEmpty(out)
}

func hasFeature(features []string, feature string) bool {
	return slices.ContainsFunc(features, func(f string) bool { return strings.EqualFold(f, feature) })
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderNebius, Nebius{})
}

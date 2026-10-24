package models

import (
	"context"
	"math"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type veniceList struct {
	Data []struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		ModelSpec struct {
			Name                   string `json:"name"`
			AvailableContextTokens int64  `json:"availableContextTokens"`
			MaxCompletionTokens    int64  `json:"maxCompletionTokens"`
			Offline                bool   `json:"offline"`
			Beta                   bool   `json:"beta"`
			Capabilities           struct {
				SupportsFunctionCalling bool `json:"supportsFunctionCalling"`
				SupportsReasoning       bool `json:"supportsReasoning"`
				SupportsReasoningEffort bool `json:"supportsReasoningEffort"`
				SupportsVision          bool `json:"supportsVision"`
			} `json:"capabilities"`
			Constraints struct {
				Temperature *struct {
					Default float64 `json:"default"`
				} `json:"temperature"`
				TopP *struct {
					Default float64 `json:"default"`
				} `json:"top_p"`
			} `json:"constraints"`
			Pricing struct {
				Input struct {
					USD float64 `json:"usd"`
				} `json:"input"`
				Output struct {
					USD float64 `json:"usd"`
				} `json:"output"`
			} `json:"pricing"`
		} `json:"model_spec"`
	} `json:"data"`
}

// Venice lists the text models of Venice that can call functions and are
// not offline or in beta.
type Venice struct{}

var _ catwalk.Lister = Venice{}

// Models implements Lister.
func (Venice) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list veniceList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	round := func(v float64) float64 { return math.Round(v*1e5) / 1e5 }
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		spec := m.ModelSpec
		caps := spec.Capabilities
		if m.ID == "" || !strings.EqualFold(m.Type, "text") || spec.Offline || spec.Beta || !caps.SupportsFunctionCalling || spec.AvailableContextTokens <= 0 {
			continue
		}

		reasoning := catwalk.Reasoning{Thinking: catwalk.ThinkingNever}
		if caps.SupportsReasoning {
			reasoning.Thinking = catwalk.ThinkingToggleable
		}
		if caps.SupportsReasoning && caps.SupportsReasoningEffort {
			reasoning.EffortLevels = catwalk.NewEffortLevels("low", "medium", "high")
			reasoning.DefaultEffortLevel = "medium"
		}

		var options catwalk.ModelOptions
		if t := spec.Constraints.Temperature; t != nil && !math.IsNaN(t.Default) {
			v := t.Default
			options.Temperature = &v
		}
		if p := spec.Constraints.TopP; p != nil && !math.IsNaN(p.Default) {
			v := p.Default
			options.TopP = &v
		}

		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(spec.Name, m.ID),
			Pricing: catwalk.Pricing{
				Input:  round(spec.Pricing.Input.USD),
				Output: round(spec.Pricing.Output.USD),
			},
			ContextWindow:    spec.AvailableContextTokens,
			DefaultMaxTokens: spec.MaxCompletionTokens,
			Reasoning:        reasoning,
			Capabilities:     catwalk.Capabilities{Vision: caps.SupportsVision},
			Options:          options,
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderVenice, Venice{})
}

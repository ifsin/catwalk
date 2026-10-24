package models

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

// huggingFaceProviders are the router backends the generator keeps.
var huggingFaceProviders = []string{"fireworks-ai", "groq", "cerebras", "hf-inference"}

type huggingFaceList struct {
	Data []struct {
		ID        string `json:"id"`
		Providers []struct {
			Provider      string `json:"provider"`
			Status        string `json:"status"`
			ContextLength int64  `json:"context_length"`
			Pricing       *struct {
				Input  float64 `json:"input"`
				Output float64 `json:"output"`
			} `json:"pricing"`
			SupportsTools bool `json:"supports_tools"`
		} `json:"providers"`
	} `json:"data"`
}

// HuggingFace lists the models served by the Hugging Face router, one entry
// per live tool-calling backend.
type HuggingFace struct{}

var _ catwalk.Lister = HuggingFace{}

// Models implements Lister.
func (HuggingFace) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list huggingFaceList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	round := func(v float64) float64 { return math.Round(v*1e5) / 1e5 }
	out := make([]catwalk.Model, 0)
	for _, m := range list.Data {
		if m.ID == "" {
			continue
		}
		var fallback int64
		for _, p := range m.Providers {
			if p.ContextLength > 0 {
				fallback = p.ContextLength
				break
			}
		}
		if fallback == 0 {
			continue
		}
		for _, p := range m.Providers {
			if !slices.Contains(huggingFaceProviders, p.Provider) || !p.SupportsTools || p.Status != "live" {
				continue
			}
			ctxLen := cmp.Or(p.ContextLength, fallback)
			var pricing catwalk.Pricing
			if p.Pricing != nil {
				pricing = catwalk.Pricing{Input: round(p.Pricing.Input), Output: round(p.Pricing.Output)}
			}
			out = append(out, catwalk.Model{
				ID:               m.ID + ":" + p.Provider,
				Name:             fmt.Sprintf("%s (%s)", m.ID, p.Provider),
				Pricing:          pricing,
				ContextWindow:    ctxLen,
				DefaultMaxTokens: min(ctxLen/4, 8192),
				Reasoning:        catwalk.Reasoning{Thinking: catwalk.ThinkingNever},
			})
		}
	}
	slices.SortFunc(out, func(a, b catwalk.Model) int {
		if a.Name == b.Name {
			return strings.Compare(a.ID, b.ID)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderHuggingFace, HuggingFace{})
}

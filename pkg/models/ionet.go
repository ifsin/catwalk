package models

import (
	"cmp"
	"context"
	"math"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type ionetList struct {
	Data []struct {
		ID                   string  `json:"id"`
		Name                 string  `json:"name"`
		MaxTokens            int64   `json:"max_tokens"`
		ContextWindow        int64   `json:"context_window"`
		SupportsImagesInput  bool    `json:"supports_images_input"`
		InputTokenPrice      float64 `json:"input_token_price"`
		OutputTokenPrice     float64 `json:"output_token_price"`
		CacheWriteTokenPrice float64 `json:"cache_write_token_price"`
		CacheReadTokenPrice  float64 `json:"cache_read_token_price"`
	} `json:"data"`
}

// IoNet lists the models of an io.net Intelligence account.
type IoNet struct{}

var _ catwalk.Lister = IoNet{}

// Models implements Lister. Models the generator skips for coding use are
// dropped, and duplicate IDs are kept once.
func (IoNet) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list ionetList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	round := func(v float64) float64 { return math.Round(v*1e5) / 1e5 }
	seen := make(map[string]bool)
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || seen[m.ID] || ionetSkipped(m.ID) {
			continue
		}
		seen[m.ID] = true
		name := m.Name
		if name == "" {
			name = m.ID
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: name,
			Pricing: catwalk.Pricing{
				Input:       round(m.InputTokenPrice * 1_000_000),
				Output:      round(m.OutputTokenPrice * 1_000_000),
				CacheCreate: round(m.CacheWriteTokenPrice * 1_000_000),
				CacheHit:    round(m.CacheReadTokenPrice * 1_000_000),
			},
			ContextWindow:    m.ContextWindow,
			DefaultMaxTokens: cmp.Or(m.MaxTokens, m.ContextWindow) / 10,
			Reasoning:        catwalk.Reasoning{Thinking: catwalk.ThinkingNever},
			Capabilities:     catwalk.Capabilities{Vision: m.SupportsImagesInput},
		})
	}
	return NonEmpty(out)
}

// ionetSkipped reports whether the generator leaves a model out as not good
// enough for coding.
func ionetSkipped(id string) bool {
	id = strings.ToLower(id)
	for _, s := range []string{"deepseek-r1", "llama-3.2", "mistral-nemo"} {
		if strings.Contains(id, s) {
			return true
		}
	}
	return false
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderIoNet, IoNet{})
}

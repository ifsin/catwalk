package models

import (
	"context"
	"math"

	"charm.land/catwalk/pkg/catwalk"
)

type pioneerList struct {
	Models []struct {
		ID                 string  `json:"id"`
		Label              string  `json:"label"`
		ContextWindow      int64   `json:"context_window"`
		MaxOutputTokens    int64   `json:"max_output_tokens"`
		InputPrice         float64 `json:"input_price_per_million"`
		OutputPrice        float64 `json:"output_price_per_million"`
		CacheReadPrice     float64 `json:"cache_read_price_per_million"`
		CacheWritePrice    float64 `json:"cache_write_price_per_million"`
		TaskType           string  `json:"task_type"`
		IsChatModel        bool    `json:"is_chat_model"`
		SupportsInference  bool    `json:"supports_inference"`
		SupportsImageInput bool    `json:"supports_image_input"`
		Deprecated         bool    `json:"deprecated"`
	} `json:"models"`
}

// Pioneer lists the chat models that can run inference on Pioneer.
type Pioneer struct{}

var _ catwalk.Lister = Pioneer{}

// Models implements Lister.
func (Pioneer) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/base-models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list pioneerList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	round := func(v float64) float64 { return math.Round(v*1e5) / 1e5 }
	out := make([]catwalk.Model, 0, len(list.Models))
	for _, m := range list.Models {
		if m.ID == "" || !m.IsChatModel || m.TaskType != "decoder" || m.Deprecated || !m.SupportsInference {
			continue
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(m.Label, m.ID),
			Pricing: catwalk.Pricing{
				Input:       round(m.InputPrice),
				Output:      round(m.OutputPrice),
				CacheCreate: round(m.CacheWritePrice),
				CacheHit:    round(m.CacheReadPrice),
			},
			ContextWindow:    m.ContextWindow,
			DefaultMaxTokens: m.MaxOutputTokens,
			Capabilities:     catwalk.Capabilities{Vision: m.SupportsImageInput},
		})
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister("pioneer", Pioneer{})
}

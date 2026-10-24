package models

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type atlasCloudPrice struct {
	Prompt         string `json:"prompt"`
	Completion     string `json:"completion"`
	InputCacheRead string `json:"input_cache_read"`
}

type atlasCloudList struct {
	Data []struct {
		ID                string          `json:"id"`
		Name              string          `json:"name"`
		IsReady           *bool           `json:"is_ready"`
		InputModalities   []string        `json:"input_modalities"`
		OutputModalities  []string        `json:"output_modalities"`
		ContextLength     int64           `json:"context_length"`
		MaxOutputLength   int64           `json:"max_output_length"`
		SupportedFeatures []string        `json:"supported_features"`
		DiscountToUser    float64         `json:"discount_to_user"`
		Pricing           json.RawMessage `json:"pricing"`
	} `json:"data"`
}

// AtlasCloud lists the models of Atlas Cloud.
type AtlasCloud struct{}

var _ catwalk.Lister = AtlasCloud{}

// Models implements Lister. Models that are not ready, have a short context
// window, are not text models or lack tool support are dropped. Prices are
// the base tier, discounted for the account.
func (AtlasCloud) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list atlasCloudList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" || (m.IsReady != nil && !*m.IsReady) || m.ContextLength < 20000 {
			continue
		}
		if !slices.Contains(m.InputModalities, "text") || !slices.Contains(m.OutputModalities, "text") {
			continue
		}
		if !slices.Contains(m.SupportedFeatures, "tools") {
			continue
		}
		if m.DiscountToUser < 0 || m.DiscountToUser > 1 {
			continue
		}
		price, err := atlasCloudBasePrice(m.Pricing)
		if err != nil {
			continue
		}
		mult := 1 - m.DiscountToUser
		maxTokens := m.ContextLength / 10
		if m.MaxOutputLength > 0 {
			if half := m.MaxOutputLength / 2; half <= (m.ContextLength*15)/100 {
				maxTokens = half
			}
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: FirstNonEmpty(m.Name, m.ID),
			Pricing: catwalk.Pricing{
				Input:    math.Round(atlasCloudPerMillion(price.Prompt)*mult*1e5) / 1e5,
				Output:   math.Round(atlasCloudPerMillion(price.Completion)*mult*1e5) / 1e5,
				CacheHit: math.Round(atlasCloudPerMillion(price.InputCacheRead)*mult*1e5) / 1e5,
			},
			ContextWindow:    m.ContextLength,
			DefaultMaxTokens: maxTokens,
			Reasoning:        catwalk.Reasoning{Thinking: catwalk.ThinkingNever},
			Capabilities:     catwalk.Capabilities{Vision: slices.Contains(m.InputModalities, "image")},
		})
	}
	return NonEmpty(out)
}

// atlasCloudBasePrice reads pricing that is either one tier object or a list
// of tiers, and returns the first tier.
func atlasCloudBasePrice(raw json.RawMessage) (atlasCloudPrice, error) {
	var single atlasCloudPrice
	if err := json.Unmarshal(raw, &single); err == nil && single.Prompt != "" {
		return single, nil
	}
	var tiers []atlasCloudPrice
	if err := json.Unmarshal(raw, &tiers); err != nil {
		return atlasCloudPrice{}, err
	}
	if len(tiers) == 0 {
		return atlasCloudPrice{}, nil
	}
	return tiers[0], nil
}

// atlasCloudPerMillion converts a price string per token, with an optional
// "$" prefix, to a price per million tokens.
func atlasCloudPerMillion(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimPrefix(s, "$"), 64)
	if err != nil {
		return 0
	}
	return v * 1_000_000
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderAtlasCloud, AtlasCloud{})
}

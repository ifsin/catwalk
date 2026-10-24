package models

import (
	"context"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type zenList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// OpenCodeZen lists the models of OpenCode Zen. The endpoint only names the
// models, so the endpoint type of each one is read from its ID.
type OpenCodeZen struct{}

var _ catwalk.Lister = OpenCodeZen{}

// Models implements Lister.
func (OpenCodeZen) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list zenList
	if err := Decode(body, &list); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(list.Data))
	for _, m := range list.Data {
		if m.ID == "" {
			continue
		}
		out = append(out, catwalk.Model{
			ID:   m.ID,
			Name: m.ID,
			Type: zenEndpointType(m.ID),
		})
	}
	return NonEmpty(out)
}

// zenEndpointType returns the endpoint type of a model that does not use the
// provider's Chat Completions endpoint, or the empty string.
func zenEndpointType(id string) catwalk.Type {
	switch {
	case strings.HasPrefix(id, "claude-"),
		strings.HasPrefix(id, "qwen3.5-"),
		strings.HasPrefix(id, "qwen3.6-"),
		strings.HasPrefix(id, "qwen3.7-"),
		strings.HasPrefix(id, "qwen3.8-"):
		return catwalk.TypeMessages
	case strings.HasPrefix(id, "gpt-"),
		strings.HasPrefix(id, "grok-"),
		strings.HasPrefix(id, "muse-spark-"):
		return catwalk.TypeResponses
	default:
		return ""
	}
}

func init() {
	catwalk.RegisterLister(catwalk.InferenceProviderOpenCodeZen, OpenCodeZen{})
}

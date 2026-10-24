package models

import (
	"context"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

type goList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// OpenCodeGo lists the models of OpenCode Go. The endpoint only names the
// models, so the endpoint type of each one is read from its ID.
type OpenCodeGo struct{}

var _ catwalk.Lister = OpenCodeGo{}

// Models implements Lister.
func (OpenCodeGo) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	url, err := Join(s, "/models")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var list goList
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
			Type: goEndpointType(m.ID),
		})
	}
	return NonEmpty(out)
}

// goEndpointType returns the endpoint type of a model that does not use the
// provider's Chat Completions endpoint, or the empty string.
func goEndpointType(id string) catwalk.Type {
	switch {
	case strings.HasPrefix(id, "minimax-"),
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
	catwalk.RegisterLister(catwalk.InferenceProviderOpenCodeGo, OpenCodeGo{})
}

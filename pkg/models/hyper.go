package models

import (
	"context"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

// Hyper lists the models of the Hyper service. The service serves its whole
// provider description, models included, from its own API path, which is not
// under the base URL requests are sent to. It still answers in the v2 schema,
// which is read through catwalk.ProviderV2 until the service moves to v3.
type Hyper struct{}

var _ catwalk.Lister = Hyper{}

// Models implements Lister.
func (Hyper) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	s.BaseURL = strings.TrimSuffix(strings.TrimRight(s.BaseURL, "/"), "/v1")
	url, err := Join(s, "/api/v1/provider")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var provider catwalk.ProviderV2
	if err := Decode(body, &provider); err != nil {
		return nil, err
	}
	out := make([]catwalk.Model, 0, len(provider.Models))
	for _, m := range provider.ToProvider().Models {
		if m.ID != "" {
			out = append(out, m)
		}
	}
	return NonEmpty(out)
}

func init() {
	catwalk.RegisterLister("hyper", Hyper{})
}

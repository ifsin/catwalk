package models

import (
	"context"
	"strings"

	"charm.land/catwalk/pkg/catwalk"
)

// Hyper lists the models of the Hyper service. The service serves its whole
// provider description, models included, from its own API path, which is not
// under the base URL requests are sent to.
type Hyper struct{}

var _ catwalk.Lister = Hyper{}

// Models implements Lister.
func (Hyper) Models(ctx context.Context, s catwalk.Source) ([]catwalk.Model, error) {
	base := strings.TrimSuffix(strings.TrimRight(s.BaseURL, "/"), "/v1")
	s.BaseURL = base
	url, err := Join(s, "/api/v1/provider")
	if err != nil {
		return nil, err
	}
	body, err := Get(ctx, s, url)
	if err != nil {
		return nil, err
	}
	var provider catwalk.Provider
	if err := Decode(body, &provider); err != nil {
		return nil, err
	}
	return NonEmpty(provider.Models)
}

func init() {
	catwalk.RegisterLister("hyper", Hyper{})
}

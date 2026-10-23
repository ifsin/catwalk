// Package providers provides the authenticators of every provider that
// supports signing in.
package providers

import (
	"charm.land/catwalk/pkg/auth"
	"charm.land/catwalk/pkg/auth/copilot"
	"charm.land/catwalk/pkg/auth/grok"
	"charm.land/catwalk/pkg/auth/hyper"
	"charm.land/catwalk/pkg/auth/openai"
	"charm.land/catwalk/pkg/catwalk"
)

// All returns a fresh authenticator for every provider that supports signing
// in.
func All() []auth.Authenticator {
	return []auth.Authenticator{
		openai.New(),
		copilot.New(),
		grok.New(),
		hyper.New(),
	}
}

// Lookup returns the authenticator for a provider.
func Lookup(id catwalk.InferenceProvider) (auth.Authenticator, bool) {
	for _, a := range All() {
		if a.Provider() == id {
			return a, true
		}
	}
	return nil, false
}

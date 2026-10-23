// Package providers enables every provider implementation. Import it for its
// side effect: each provider registers its sign-in and its live model lister
// with package catwalk, and catwalk.Provider.Auth and Lister then find them.
package providers

import (
	_ "charm.land/catwalk/pkg/auth/copilot"
	_ "charm.land/catwalk/pkg/auth/grok"
	_ "charm.land/catwalk/pkg/auth/hyper"
	_ "charm.land/catwalk/pkg/auth/openai"
	_ "charm.land/catwalk/pkg/models"
)

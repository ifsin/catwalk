package catwalk

import (
	"context"
	"net/http"
)

// AuthMode is the kind of credential a Source carries. A provider can hold
// both a sign-in and an API key, but a request uses one of them.
type AuthMode string

// All the supported credential kinds.
const (
	// AuthAPIKey is a key the user entered.
	AuthAPIKey AuthMode = "apiKey"
	// AuthSignIn is the access token of a sign-in.
	AuthSignIn AuthMode = "signIn"
)

// Source describes where and how to ask a provider for its models. It is the
// same information a client uses to send requests.
type Source struct {
	// Auth is the kind of credential in APIKey. Empty means an API key.
	Auth AuthMode
	// BaseURL is the API base URL, for example https://api.openai.com/v1.
	BaseURL string
	// APIKey is the key or access token.
	APIKey string
	// Headers are extra headers to send.
	Headers map[string]string
	// Client sends the request. Nil means http.DefaultClient. Every request
	// is bounded by a timeout either way.
	Client *http.Client
}

// Source returns the request as the source of a model list.
func (r Request) Source() Source {
	return Source{Auth: AuthSignIn, BaseURL: r.BaseURL, APIKey: r.APIKey, Headers: r.Headers, Client: r.Client(nil)}
}

// Lister fetches the models an account can use. An implementation makes one
// request, does not retry and returns an error instead of an empty list.
type Lister interface {
	Models(ctx context.Context, s Source) ([]Model, error)
}

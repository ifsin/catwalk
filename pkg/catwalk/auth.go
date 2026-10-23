package catwalk

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Kind is the way a user completes a sign-in.
type Kind string

// All the supported sign-in kinds.
const (
	// KindBrowser opens an authorization page in a browser and receives the
	// result on a loopback redirect.
	KindBrowser Kind = "browser"
	// KindDevice shows a short code that the user enters on a verification
	// page, while the flow polls for the result.
	KindDevice Kind = "device"
)

// refreshSkew is how long before the real expiry a credential is considered
// due for a refresh.
const refreshSkew = time.Minute

// Credential is the result of a sign-in. Clients store it as an opaque JSON
// blob and hand it back to Refresh and Request.
type Credential struct {
	Provider     InferenceProvider `json:"provider"`
	AccessToken  string            `json:"access_token"`
	RefreshToken string            `json:"refresh_token,omitempty"`
	IDToken      string            `json:"id_token,omitempty"`
	// ExpiresAt is the access token expiry in unix seconds. Zero means
	// unknown, which counts as expired.
	ExpiresAt int64 `json:"expires_at,omitempty"`
	// Extra holds provider-specific values such as the ChatGPT account ID.
	Extra map[string]string `json:"extra,omitempty"`
}

// Expired reports whether the access token is expired or about to expire.
func (c *Credential) Expired() bool {
	return c.ExpiredAt(time.Now())
}

// ExpiredAt reports whether the access token is expired, or within the
// refresh window, at the given time.
func (c *Credential) ExpiredAt(now time.Time) bool {
	if c.ExpiresAt == 0 {
		return true
	}
	return !now.Add(refreshSkew).Before(time.Unix(c.ExpiresAt, 0))
}

// Clone returns a deep copy of the credential.
func (c *Credential) Clone() *Credential {
	out := *c
	if c.Extra != nil {
		out.Extra = make(map[string]string, len(c.Extra))
		for k, v := range c.Extra {
			out.Extra[k] = v
		}
	}
	return &out
}

// Marshal encodes the credential as the blob a client stores.
func (c *Credential) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// ParseCredential decodes a blob produced by Marshal.
func ParseCredential(data []byte) (*Credential, error) {
	var c Credential
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Prompt is what the client shows the user while a session is waiting.
type Prompt struct {
	Kind Kind
	// URL is the page to open: the authorization page for a browser sign-in,
	// the verification page for a device sign-in.
	URL string
	// UserCode is the code to enter on the verification page. Empty for a
	// browser sign-in.
	UserCode string
	// ExpiresAt is when the session gives up. Zero means no fixed deadline.
	ExpiresAt time.Time
}

// Session is a sign-in in progress.
type Session interface {
	// Prompt returns what to show or open for the user.
	Prompt() Prompt
	// Wait blocks until the sign-in completes, fails, or ctx is done.
	Wait(ctx context.Context) (*Credential, error)
	// Close releases any resources. It is safe to call more than once.
	Close() error
}

// Request describes how to talk to a provider with a credential.
type Request struct {
	// BaseURL replaces the provider's default API endpoint. Empty keeps it.
	BaseURL string
	// APIKey is the secret to send where an API key would go.
	APIKey string
	// Headers are extra headers to add to every request.
	Headers map[string]string
	// Wrap adapts the HTTP transport. It may be nil.
	Wrap func(http.RoundTripper) http.RoundTripper
}

// Client returns an HTTP client that applies Wrap to base. A nil base means
// http.DefaultTransport.
func (r Request) Client(base http.RoundTripper) *http.Client {
	if base == nil {
		base = http.DefaultTransport
	}
	if r.Wrap != nil {
		base = r.Wrap(base)
	}
	return &http.Client{Transport: base}
}

// Authenticator signs in to one provider.
type Authenticator interface {
	// Provider is the provider this authenticator signs in to.
	Provider() InferenceProvider
	// Kinds lists the supported sign-in kinds, preferred first.
	Kinds() []Kind
	// Start begins a sign-in of the given kind.
	Start(ctx context.Context, kind Kind) (Session, error)
	// Refresh returns an updated credential. The old one must be replaced by
	// the result, because refresh tokens can rotate.
	Refresh(ctx context.Context, c *Credential) (*Credential, error)
	// Request returns how to use the credential against the provider.
	Request(c *Credential) Request
}

// Supports reports whether a supports the given kind.
func Supports(a Authenticator, kind Kind) bool {
	for _, k := range a.Kinds() {
		if k == kind {
			return true
		}
	}
	return false
}

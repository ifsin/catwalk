package auth

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"crypto/subtle"
	"fmt"
	"net/url"
)

// BrowserSession is a catwalk.Session that waits for a browser redirect.
type BrowserSession struct {
	prompt   catwalk.Prompt
	state    string
	loop     *Loopback
	exchange func(ctx context.Context, code string) (*catwalk.Credential, error)
}

// NewBrowserSession returns a session that shows authURL and finishes when
// the loopback receives the redirect. exchange trades the authorization code
// for a credential.
func NewBrowserSession(authURL, state string, loop *Loopback, exchange func(ctx context.Context, code string) (*catwalk.Credential, error)) *BrowserSession {
	return &BrowserSession{
		prompt:   catwalk.Prompt{Kind: catwalk.KindBrowser, URL: authURL},
		state:    state,
		loop:     loop,
		exchange: exchange,
	}
}

// Prompt implements catwalk.Session.
func (s *BrowserSession) Prompt() catwalk.Prompt { return s.prompt }

// Wait implements catwalk.Session.
func (s *BrowserSession) Wait(ctx context.Context) (*catwalk.Credential, error) {
	defer s.Close()
	select {
	case q := <-s.loop.Results():
		return s.Complete(ctx, q)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Complete finishes the sign-in with the query of the redirect. Clients that
// cannot run a loopback listener can receive the redirect themselves and pass
// its query here.
func (s *BrowserSession) Complete(ctx context.Context, q url.Values) (*catwalk.Credential, error) {
	if code := q.Get("error"); code != "" {
		if code == "access_denied" {
			return nil, ErrDenied
		}
		return nil, fmt.Errorf("authorization failed: %s: %s", code, q.Get("error_description"))
	}
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(s.state)) != 1 {
		return nil, fmt.Errorf("authorization state mismatch")
	}
	code := q.Get("code")
	if code == "" {
		return nil, fmt.Errorf("authorization response contained no code")
	}
	return s.exchange(ctx, code)
}

// RedirectURI returns the redirect URI of the session.
func (s *BrowserSession) RedirectURI() string { return s.loop.RedirectURI() }

// Close implements catwalk.Session.
func (s *BrowserSession) Close() error { return s.loop.Close() }

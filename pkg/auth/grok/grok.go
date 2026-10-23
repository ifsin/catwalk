// Package grok provides sign-in with a Grok account for the xAI provider.
//
// The access token is a bearer credential for the xAI API. The sign-in is a
// browser authorization with PKCE and a loopback redirect, with the device
// flow as a fallback for machines where the redirect cannot be received.
package grok

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"charm.land/catwalk/pkg/auth"
	"charm.land/catwalk/pkg/catwalk"
)

const (
	// ClientID is the public OAuth client of the Grok CLI.
	ClientID = "b1a00492-073a-47ea-816f-4c329264a828"
	// Issuer is the authorization server.
	Issuer = "https://auth.x.ai"
	// APIBaseURL is the API the access token is valid for.
	APIBaseURL = "https://api.x.ai/v1"

	scope = "openid profile email offline_access grok-cli:access " +
		"api:access conversations:read conversations:write " +
		"workspaces:read workspaces:write"
	callbackPath = "/callback"
)

// Auth signs in to xAI with a Grok account.
type Auth struct {
	// HTTPClient is used for token requests. Nil uses a client with a
	// timeout.
	HTTPClient *http.Client
	// Referrer identifies the client to the authorization server.
	Referrer string

	issuer       string
	pollInterval time.Duration
}

// New returns an authenticator for xAI.
func New() *Auth {
	return &Auth{Referrer: "catwalk", issuer: Issuer}
}

// Provider implements catwalk.Authenticator.
func (a *Auth) Provider() catwalk.InferenceProvider { return catwalk.InferenceProviderXAI }

// Kinds implements catwalk.Authenticator.
func (a *Auth) Kinds() []catwalk.Kind { return []catwalk.Kind{catwalk.KindBrowser, catwalk.KindDevice} }

// Start implements catwalk.Authenticator.
func (a *Auth) Start(ctx context.Context, kind catwalk.Kind) (catwalk.Session, error) {
	switch kind {
	case catwalk.KindBrowser:
		return a.startBrowser()
	case catwalk.KindDevice:
		return a.startDevice(ctx)
	}
	return nil, auth.ErrUnsupportedKind
}

func (a *Auth) startBrowser() (catwalk.Session, error) {
	pkce, err := auth.NewPKCE()
	if err != nil {
		return nil, err
	}
	state, err := auth.RandomString(32)
	if err != nil {
		return nil, err
	}
	nonce, err := auth.RandomString(32)
	if err != nil {
		return nil, err
	}
	loop, err := auth.ListenLoopback("127.0.0.1", nil, callbackPath, "xAI")
	if err != nil {
		return nil, err
	}
	redirect := loop.RedirectURI()

	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirect},
		"scope":                 {scope},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"nonce":                 {nonce},
		"referrer":              {a.Referrer},
	}
	authURL := a.issuer + "/oauth2/authorize?" + q.Encode()

	return auth.NewBrowserSession(authURL, state, loop, func(ctx context.Context, code string) (*catwalk.Credential, error) {
		cred, err := a.token(ctx, url.Values{
			"grant_type":    {"authorization_code"},
			"code":          {code},
			"redirect_uri":  {redirect},
			"client_id":     {ClientID},
			"code_verifier": {pkce.Verifier},
		}, nil)
		if err != nil {
			return nil, fmt.Errorf("exchange authorization code: %w", err)
		}
		return cred, nil
	}), nil
}

func (a *Auth) startDevice(ctx context.Context) (catwalk.Session, error) {
	var dc auth.DeviceCode
	err := auth.PostForm(ctx, a.HTTPClient, a.issuer+"/oauth2/device/code", url.Values{
		"client_id": {ClientID},
		"scope":     {scope},
		"referrer":  {a.Referrer},
	}, nil, &dc)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	if !dc.Valid() {
		return nil, fmt.Errorf("device code response contained no device or user code")
	}

	return auth.NewDeviceSession(dc.Prompt(), func(ctx context.Context) (*catwalk.Credential, error) {
		return auth.PollToken(ctx, dc.PollInterval(a.pollInterval), dc.Lifetime(), func(ctx context.Context) (*catwalk.Credential, error) {
			return a.token(ctx, url.Values{
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": {dc.DeviceCode},
				"client_id":   {ClientID},
			}, nil)
		})
	}), nil
}

// Refresh implements catwalk.Authenticator.
func (a *Auth) Refresh(ctx context.Context, c *catwalk.Credential) (*catwalk.Credential, error) {
	if c.RefreshToken == "" {
		return nil, auth.ErrReauthRequired
	}
	cred, err := a.token(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {c.RefreshToken},
		"client_id":     {ClientID},
	}, c)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", auth.RefreshError(err))
	}
	return cred, nil
}

// Request implements catwalk.Authenticator.
func (a *Auth) Request(c *catwalk.Credential) catwalk.Request {
	return catwalk.Request{BaseURL: APIBaseURL, APIKey: c.AccessToken}
}

func (a *Auth) token(ctx context.Context, form url.Values, prev *catwalk.Credential) (*catwalk.Credential, error) {
	tr, err := auth.RequestToken(ctx, a.HTTPClient, a.issuer+"/oauth2/token", form)
	if err != nil {
		return nil, err
	}
	return tr.Credential(catwalk.InferenceProviderXAI, prev), nil
}

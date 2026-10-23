// Package openai provides sign-in with a ChatGPT account for the OpenAI
// provider.
//
// The flow is a browser authorization with PKCE and a loopback redirect. The
// resulting token is used against the ChatGPT Codex backend, which speaks the
// Responses API for ChatGPT plan accounts.
package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"charm.land/catwalk/pkg/auth"
	"charm.land/catwalk/pkg/catwalk"
)

const (
	// ClientID is the public OAuth client of the Codex desktop sign-in.
	ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	// Issuer is the authorization server.
	Issuer = "https://auth.openai.com"
	// CodexBaseURL is the backend that serves ChatGPT plan accounts.
	CodexBaseURL = "https://chatgpt.com/backend-api/codex"

	// AccountID is the key of the ChatGPT account ID in Credential.Extra.
	AccountID = "account_id"

	scope        = "openid profile email offline_access"
	callbackPath = "/auth/callback"
)

var callbackPorts = []int{1455, 1457}

// Auth signs in to OpenAI with a ChatGPT account.
type Auth struct {
	// HTTPClient is used for token requests. Nil uses a client with a
	// timeout.
	HTTPClient *http.Client
	// Originator identifies the client to the backend.
	Originator string

	issuer string
	ports  []int
}

// New returns an authenticator for OpenAI.
func New() *Auth {
	return &Auth{Originator: "catwalk", issuer: Issuer, ports: callbackPorts}
}

// Provider implements catwalk.Authenticator.
func (a *Auth) Provider() catwalk.InferenceProvider { return catwalk.InferenceProviderOpenAI }

// Kinds implements catwalk.Authenticator.
func (a *Auth) Kinds() []catwalk.Kind { return []catwalk.Kind{catwalk.KindBrowser} }

// Start implements catwalk.Authenticator.
func (a *Auth) Start(_ context.Context, kind catwalk.Kind) (catwalk.Session, error) {
	if kind != catwalk.KindBrowser {
		return nil, auth.ErrUnsupportedKind
	}
	pkce, err := auth.NewPKCE()
	if err != nil {
		return nil, err
	}
	state, err := auth.RandomString(32)
	if err != nil {
		return nil, err
	}
	loop, err := auth.ListenLoopback("localhost", a.ports, callbackPath, "OpenAI")
	if err != nil {
		return nil, err
	}
	redirect := loop.RedirectURI()

	q := url.Values{
		"response_type":              {"code"},
		"client_id":                  {ClientID},
		"redirect_uri":               {redirect},
		"scope":                      {scope},
		"code_challenge":             {pkce.Challenge},
		"code_challenge_method":      {"S256"},
		"id_token_add_organizations": {"true"},
		"codex_cli_simplified_flow":  {"true"},
		"state":                      {state},
		"originator":                 {a.Originator},
	}
	authURL := a.issuer + "/oauth/authorize?" + q.Encode()

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
	accountID := c.Extra[AccountID]
	headers := map[string]string{"originator": a.Originator}
	if accountID != "" {
		headers["chatgpt-account-id"] = accountID
	}
	return catwalk.Request{
		BaseURL: CodexBaseURL,
		APIKey:  c.AccessToken,
		Headers: headers,
		Wrap: func(base http.RoundTripper) http.RoundTripper {
			return &transport{base: base, originator: a.Originator, accountID: accountID}
		},
	}
}

func (a *Auth) token(ctx context.Context, form url.Values, prev *catwalk.Credential) (*catwalk.Credential, error) {
	tr, err := auth.RequestToken(ctx, a.HTTPClient, a.issuer+"/oauth/token", form)
	if err != nil {
		return nil, err
	}
	cred := tr.Credential(catwalk.InferenceProviderOpenAI, prev)
	id := accountID(tr.IDToken)
	if id == "" {
		id = accountID(tr.AccessToken)
	}
	if id != "" {
		if cred.Extra == nil {
			cred.Extra = map[string]string{}
		}
		cred.Extra[AccountID] = id
	}
	return cred, nil
}

func accountID(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var claims struct {
		AccountID string `json:"chatgpt_account_id"`
		Auth      struct {
			AccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	if claims.AccountID != "" {
		return claims.AccountID
	}
	return claims.Auth.AccountID
}

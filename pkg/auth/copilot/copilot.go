// Package copilot provides sign-in with a GitHub account for the Copilot
// provider.
//
// The sign-in is a device flow against GitHub. The GitHub token is then
// exchanged for a short-lived Copilot token. The credential keeps the Copilot
// token as its access token and the GitHub token as its refresh token, so a
// refresh mints a new Copilot token.
package copilot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"charm.land/catwalk/pkg/auth"
	"charm.land/catwalk/pkg/catwalk"
)

const (
	// ClientID is the public OAuth client of the Copilot editor plugins.
	ClientID = "Iv1.b507a08c87ecfe98"
	// APIBaseURL is the Copilot API when the token response names none.
	APIBaseURL = "https://api.githubcopilot.com"
	// APIEndpoint is the Credential.Extra key of the API endpoint that the
	// token response assigned to the account.
	APIEndpoint = "api_endpoint"

	deviceCodeURL   = "https://github.com/login/device/code"
	accessTokenURL  = "https://github.com/login/oauth/access_token"
	copilotTokenURL = "https://api.github.com/copilot_internal/v2/token"

	userAgent           = "GitHubCopilotChat/0.32.4"
	editorVersion       = "vscode/1.105.1"
	editorPluginVersion = "copilot-chat/0.32.4"
	integrationID       = "vscode-chat"
)

// Auth signs in to Copilot with a GitHub account.
type Auth struct {
	// HTTPClient is used for requests. Nil uses a client with a timeout.
	HTTPClient *http.Client

	deviceCodeURL   string
	accessTokenURL  string
	copilotTokenURL string
	pollInterval    time.Duration
}

// New returns an authenticator for Copilot.
func New() *Auth {
	return &Auth{
		deviceCodeURL:   deviceCodeURL,
		accessTokenURL:  accessTokenURL,
		copilotTokenURL: copilotTokenURL,
	}
}

// Provider implements catwalk.Authenticator.
func (a *Auth) Provider() catwalk.InferenceProvider { return catwalk.InferenceProviderCopilot }

// Kinds implements catwalk.Authenticator.
func (a *Auth) Kinds() []catwalk.Kind { return []catwalk.Kind{catwalk.KindDevice} }

// Start implements catwalk.Authenticator.
func (a *Auth) Start(ctx context.Context, kind catwalk.Kind) (catwalk.Session, error) {
	if kind != catwalk.KindDevice {
		return nil, auth.ErrUnsupportedKind
	}
	var dc auth.DeviceCode
	err := auth.PostForm(ctx, a.HTTPClient, a.deviceCodeURL, url.Values{
		"client_id": {ClientID},
		"scope":     {"read:user"},
	}, headers(), &dc)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	if !dc.Valid() {
		return nil, fmt.Errorf("device code response contained no device or user code")
	}

	return auth.NewDeviceSession(dc.Prompt(), func(ctx context.Context) (*catwalk.Credential, error) {
		return auth.PollToken(ctx, dc.PollInterval(a.pollInterval), dc.Lifetime(), func(ctx context.Context) (*catwalk.Credential, error) {
			var resp struct {
				AccessToken string `json:"access_token"`
				Error       string `json:"error"`
			}
			err := auth.PostForm(ctx, a.HTTPClient, a.accessTokenURL, url.Values{
				"client_id":   {ClientID},
				"device_code": {dc.DeviceCode},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			}, headers(), &resp)
			if err != nil {
				return nil, err
			}
			switch resp.Error {
			case "":
			case "authorization_pending":
				return nil, auth.ErrPending
			case "slow_down":
				return nil, auth.ErrSlowDown
			case "expired_token":
				return nil, auth.ErrExpired
			case "access_denied":
				return nil, auth.ErrDenied
			default:
				return nil, fmt.Errorf("authorization failed: %s", resp.Error)
			}
			if resp.AccessToken == "" {
				return nil, auth.ErrPending
			}
			return a.copilotToken(ctx, resp.AccessToken)
		})
	}), nil
}

// Refresh implements catwalk.Authenticator.
func (a *Auth) Refresh(ctx context.Context, c *catwalk.Credential) (*catwalk.Credential, error) {
	if c.RefreshToken == "" {
		return nil, auth.ErrReauthRequired
	}
	cred, err := a.copilotToken(ctx, c.RefreshToken)
	if err != nil {
		var te *auth.TokenError
		if errors.As(err, &te) && (te.Status == http.StatusUnauthorized || te.Status == http.StatusForbidden) {
			return nil, fmt.Errorf("%w: %w", auth.ErrReauthRequired, err)
		}
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	return cred, nil
}

// Request implements catwalk.Authenticator.
func (a *Auth) Request(c *catwalk.Credential) catwalk.Request {
	base := APIBaseURL
	if endpoint := c.Extra[APIEndpoint]; endpoint != "" {
		base = endpoint
	}
	return catwalk.Request{
		BaseURL: base,
		APIKey:  c.AccessToken,
		Headers: map[string]string{
			"User-Agent":             userAgent,
			"Editor-Version":         editorVersion,
			"Editor-Plugin-Version":  editorPluginVersion,
			"Copilot-Integration-Id": integrationID,
		},
		Wrap: func(base http.RoundTripper) http.RoundTripper {
			return &initiator{base: base}
		},
	}
}

func (a *Auth) copilotToken(ctx context.Context, githubToken string) (*catwalk.Credential, error) {
	h := headers()
	h.Set("Authorization", "Bearer "+githubToken)
	var resp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
		Endpoints struct {
			API string `json:"api"`
		} `json:"endpoints"`
	}
	if err := auth.GetJSON(ctx, a.HTTPClient, a.copilotTokenURL, &resp, h); err != nil {
		var te *auth.TokenError
		if errors.As(err, &te) && te.Status == http.StatusForbidden && te.Code == "" {
			return nil, fmt.Errorf("%w: %w", auth.ErrNotAvailable, err)
		}
		return nil, err
	}
	if resp.Token == "" {
		return nil, fmt.Errorf("copilot token response contained no token")
	}
	cred := &catwalk.Credential{
		Provider:     catwalk.InferenceProviderCopilot,
		AccessToken:  resp.Token,
		RefreshToken: githubToken,
		ExpiresAt:    resp.ExpiresAt,
	}
	if resp.Endpoints.API != "" {
		cred.Extra = map[string]string{APIEndpoint: resp.Endpoints.API}
	}
	return cred, nil
}

func headers() http.Header {
	return http.Header{
		"User-Agent":             {userAgent},
		"Editor-Version":         {editorVersion},
		"Editor-Plugin-Version":  {editorPluginVersion},
		"Copilot-Integration-Id": {integrationID},
	}
}

var assistantRole = regexp.MustCompile(`"role"\s*:\s*"assistant"`)

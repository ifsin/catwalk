// Package hyper provides sign-in with a Hyper account.
//
// The sign-in is a device flow against the Hyper service. The service hands
// out a long-lived refresh token, which is exchanged for short-lived access
// tokens.
package hyper

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"charm.land/catwalk/pkg/auth"
	"charm.land/catwalk/pkg/catwalk"
)

// DefaultBaseURL is the Hyper service.
const DefaultBaseURL = "https://hyper.charm.land"

// Auth signs in to Hyper.
type Auth struct {
	// HTTPClient is used for requests. Nil uses a client with a timeout.
	HTTPClient *http.Client
	// BaseURL is the Hyper service. Empty uses the HYPER_URL environment
	// variable, then DefaultBaseURL.
	BaseURL string
	// DeviceName names this device in the account's device list.
	DeviceName string

	pollInterval time.Duration
}

// New returns an authenticator for Hyper.
func New() *Auth {
	return &Auth{DeviceName: deviceName()}
}

// Provider implements catwalk.Authenticator.
func (a *Auth) Provider() catwalk.InferenceProvider { return "hyper" }

// Kinds implements catwalk.Authenticator.
func (a *Auth) Kinds() []catwalk.Kind { return []catwalk.Kind{catwalk.KindDevice} }

type deviceResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
}

type pollResponse struct {
	RefreshToken     string `json:"refresh_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type exchangeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    int64  `json:"expires_at"`
}

// Start implements catwalk.Authenticator.
func (a *Auth) Start(ctx context.Context, kind catwalk.Kind) (catwalk.Session, error) {
	if kind != catwalk.KindDevice {
		return nil, auth.ErrUnsupportedKind
	}
	var dr deviceResponse
	err := auth.PostJSON(ctx, a.HTTPClient, a.base()+"/device/auth", map[string]string{"device_name": a.DeviceName}, &dr, nil)
	if err != nil {
		return nil, fmt.Errorf("start device sign-in: %w", err)
	}
	dc := auth.DeviceCode{
		DeviceCode:      dr.DeviceCode,
		UserCode:        dr.UserCode,
		VerificationURI: dr.VerificationURL,
		ExpiresIn:       dr.ExpiresIn,
	}
	if !dc.Valid() {
		return nil, fmt.Errorf("device sign-in response contained no device or user code")
	}

	return auth.NewDeviceSession(dc.Prompt(), func(ctx context.Context) (*catwalk.Credential, error) {
		return auth.PollToken(ctx, dc.PollInterval(a.pollInterval), dc.Lifetime(), func(ctx context.Context) (*catwalk.Credential, error) {
			var pr pollResponse
			if err := auth.GetJSON(ctx, a.HTTPClient, a.base()+"/device/auth/"+url.PathEscape(dc.DeviceCode), &pr, nil); err != nil {
				var te *auth.TokenError
				if !errors.As(err, &te) || te.Code == "" {
					return nil, err
				}
				pr.Error, pr.ErrorDescription = te.Code, te.Description
			}
			if pr.RefreshToken != "" {
				return a.exchange(ctx, pr.RefreshToken)
			}
			switch pr.Error {
			case "authorization_pending", "":
				return nil, auth.ErrPending
			case "slow_down":
				return nil, auth.ErrSlowDown
			case "expired_token":
				return nil, auth.ErrExpired
			case "access_denied":
				return nil, auth.ErrDenied
			}
			if pr.ErrorDescription != "" {
				return nil, errors.New(pr.ErrorDescription)
			}
			return nil, fmt.Errorf("device sign-in failed: %s", pr.Error)
		})
	}), nil
}

// Refresh implements catwalk.Authenticator.
func (a *Auth) Refresh(ctx context.Context, c *catwalk.Credential) (*catwalk.Credential, error) {
	if c.RefreshToken == "" {
		return nil, auth.ErrReauthRequired
	}
	cred, err := a.exchange(ctx, c.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", auth.RefreshError(err))
	}
	return cred, nil
}

// Request implements catwalk.Authenticator.
func (a *Auth) Request(c *catwalk.Credential) catwalk.Request {
	return catwalk.Request{BaseURL: a.base() + "/v1", APIKey: c.AccessToken}
}

func (a *Auth) exchange(ctx context.Context, refreshToken string) (*catwalk.Credential, error) {
	var er exchangeResponse
	err := auth.PostJSON(ctx, a.HTTPClient, a.base()+"/token/exchange", map[string]string{"refresh_token": refreshToken}, &er, nil)
	if err != nil {
		return nil, err
	}
	if er.AccessToken == "" {
		return nil, fmt.Errorf("token response contained no access token")
	}
	cred := &catwalk.Credential{
		Provider:     "hyper",
		AccessToken:  er.AccessToken,
		RefreshToken: er.RefreshToken,
		ExpiresAt:    er.ExpiresAt,
	}
	if cred.RefreshToken == "" {
		cred.RefreshToken = refreshToken
	}
	if er.ExpiresIn > 0 {
		cred.ExpiresAt = time.Now().Add(time.Duration(er.ExpiresIn) * time.Second).Unix()
	}
	return cred, nil
}

func (a *Auth) base() string {
	if a.BaseURL != "" {
		return strings.TrimRight(a.BaseURL, "/")
	}
	if v := os.Getenv("HYPER_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return DefaultBaseURL
}

func deviceName() string {
	if host, err := os.Hostname(); err == nil && host != "" {
		return "Catwalk (" + host + ")"
	}
	return "Catwalk"
}

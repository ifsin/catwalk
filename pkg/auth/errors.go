package auth

import (
	"errors"
	"fmt"
	"strings"
)

// Errors returned by sign-in and refresh.
var (
	// ErrReauthRequired means the stored credential can no longer be
	// refreshed and the user must sign in again.
	ErrReauthRequired = errors.New("auth: sign in again")
	// ErrNotAvailable means the account has no access to the provider.
	ErrNotAvailable = errors.New("auth: not available for this account")
	// ErrDenied means the user declined the authorization.
	ErrDenied = errors.New("auth: authorization denied")
	// ErrExpired means the device code or session expired.
	ErrExpired = errors.New("auth: authorization expired")
	// ErrUnsupportedKind means the provider does not offer that sign-in kind.
	ErrUnsupportedKind = errors.New("auth: unsupported sign-in kind")
	// ErrPending is returned by a poll attempt when the user has not finished.
	ErrPending = errors.New("auth: authorization pending")
	// ErrSlowDown is returned by a poll attempt when the server asks to poll
	// less often.
	ErrSlowDown = errors.New("auth: slow down")
)

// TokenError is a failed response from a token or device endpoint.
type TokenError struct {
	Status      int
	Code        string
	Description string
	Body        string
}

// Error implements error.
func (e *TokenError) Error() string {
	switch {
	case e.Code != "" && e.Description != "":
		return fmt.Sprintf("auth: token endpoint returned %d: %s: %s", e.Status, e.Code, e.Description)
	case e.Code != "":
		return fmt.Sprintf("auth: token endpoint returned %d: %s", e.Status, e.Code)
	default:
		return fmt.Sprintf("auth: token endpoint returned %d: %s", e.Status, e.Body)
	}
}

// Revoked reports whether the refresh token was rejected for good.
func (e *TokenError) Revoked() bool {
	return e.Code == "invalid_grant" || strings.Contains(e.Body, "revoked")
}

// RefreshError turns a failed refresh into ErrReauthRequired when the refresh
// token was rejected for good, and returns err unchanged otherwise.
func RefreshError(err error) error {
	var te *TokenError
	if errors.As(err, &te) && te.Revoked() {
		return fmt.Errorf("%w: %w", ErrReauthRequired, err)
	}
	return err
}

func deviceError(err error) error {
	var te *TokenError
	if !errors.As(err, &te) {
		return err
	}
	switch te.Code {
	case "authorization_pending":
		return ErrPending
	case "slow_down":
		return ErrSlowDown
	case "expired_token":
		return ErrExpired
	case "access_denied":
		return ErrDenied
	}
	return err
}

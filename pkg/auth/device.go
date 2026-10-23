package auth

import (
	"charm.land/catwalk/pkg/catwalk"
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultPollInterval = 5 * time.Second
	defaultDeviceExpiry = 15 * time.Minute
)

// DeviceCode is a device authorization grant (RFC 8628).
type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// Valid reports whether the grant has what a device sign-in needs.
func (d *DeviceCode) Valid() bool {
	return d.DeviceCode != "" && d.UserCode != "" && d.VerificationURI != ""
}

// Prompt builds the prompt for the grant.
func (d *DeviceCode) Prompt() catwalk.Prompt {
	p := catwalk.Prompt{Kind: catwalk.KindDevice, URL: d.VerificationURI, UserCode: d.UserCode}
	if d.VerificationURIComplete != "" {
		p.URL = d.VerificationURIComplete
	}
	if d.ExpiresIn > 0 {
		p.ExpiresAt = time.Now().Add(time.Duration(d.ExpiresIn) * time.Second)
	}
	return p
}

// PollInterval returns the polling interval the server asked for, or fallback
// when it did not ask for one.
func (d *DeviceCode) PollInterval(fallback time.Duration) time.Duration {
	if d.Interval > 0 {
		return time.Duration(d.Interval) * time.Second
	}
	if fallback > 0 {
		return fallback
	}
	return defaultPollInterval
}

// Lifetime returns how long the grant is valid.
func (d *DeviceCode) Lifetime() time.Duration {
	if d.ExpiresIn > 0 {
		return time.Duration(d.ExpiresIn) * time.Second
	}
	return defaultDeviceExpiry
}

// PollToken calls attempt every interval until it returns a credential, a
// terminal error, or the lifetime runs out. An attempt reports that the user
// has not finished with ErrPending, or that it should be called less often
// with ErrSlowDown. Token endpoint errors with the standard device codes are
// understood as well.
func PollToken(ctx context.Context, interval, lifetime time.Duration, attempt func(context.Context) (*catwalk.Credential, error)) (*catwalk.Credential, error) {
	if interval <= 0 {
		interval = defaultPollInterval
	}
	if lifetime <= 0 {
		lifetime = defaultDeviceExpiry
	}
	deadline := time.Now().Add(lifetime)
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}

		cred, err := attempt(ctx)
		if err == nil {
			return cred, nil
		}
		switch err = deviceError(err); {
		case errors.Is(err, ErrPending):
		case errors.Is(err, ErrSlowDown):
			interval += defaultPollInterval
		default:
			return nil, err
		}

		if !time.Now().Before(deadline) {
			return nil, ErrExpired
		}
		timer.Reset(interval)
	}
}

// DeviceSession is a catwalk.Session that waits with a poll function.
type DeviceSession struct {
	prompt catwalk.Prompt
	poll   func(context.Context) (*catwalk.Credential, error)

	mu     sync.Mutex
	cancel context.CancelFunc
	closed bool
}

// NewDeviceSession returns a session that shows prompt and finishes when poll
// returns.
func NewDeviceSession(prompt catwalk.Prompt, poll func(context.Context) (*catwalk.Credential, error)) *DeviceSession {
	return &DeviceSession{prompt: prompt, poll: poll}
}

// Prompt implements catwalk.Session.
func (s *DeviceSession) Prompt() catwalk.Prompt { return s.prompt }

// Wait implements catwalk.Session.
func (s *DeviceSession) Wait(ctx context.Context) (*catwalk.Credential, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, context.Canceled
	}
	ctx, s.cancel = context.WithCancel(ctx)
	cancel := s.cancel
	s.mu.Unlock()
	defer cancel()
	return s.poll(ctx)
}

// Close implements catwalk.Session.
func (s *DeviceSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

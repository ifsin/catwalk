// Package models holds the shared parts of live model lists: the request
// helper, the OpenAI-compatible and Anthropic listers, and the providers
// that have no list.
//
// The embedded catalog describes what a provider offers in general. What an
// account can actually use can differ: a ChatGPT sign-in only reaches the
// Codex models and a Copilot plan enables a subset. The client passes the
// endpoint and credentials it already uses for requests and gets back catwalk
// models with only the fields the endpoint reports.
package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

const (
	maxBody = 8 << 20
	timeout = 20 * time.Second
)

// ErrNotSignedIn is returned by a lister that only works with a sign-in when
// the source carries none.
var ErrNotSignedIn = errors.New("models: this provider needs a sign-in")

// ErrEmpty is returned when an endpoint answers with no usable models. A
// client should keep its previous list instead of replacing it.
var ErrEmpty = errors.New("models: the provider returned no models")

// Join appends path to the source's base URL. A lister uses it when its list
// is served by the API host, and builds the URL itself when it is not.
func Join(s catwalk.Source, path string) (string, error) {
	base := strings.TrimRight(s.BaseURL, "/")
	if base == "" {
		return "", fmt.Errorf("models: no base URL")
	}
	return base + path, nil
}

// Get sends an authenticated GET to url and returns the body of a 200
// answer.
func Get(ctx context.Context, s catwalk.Source, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if s.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("models: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &StatusError{Status: resp.StatusCode, Body: snippet(data)}
	}
	return data, nil
}

// Decode unmarshals a response body.
func Decode(body []byte, v any) error {
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("models: decode response: %w", err)
	}
	return nil
}

// NonEmpty returns list, or ErrEmpty when it has no models.
func NonEmpty(list []catwalk.Model) ([]catwalk.Model, error) {
	if len(list) == 0 {
		return nil, ErrEmpty
	}
	return list, nil
}

// FirstNonEmpty returns the first value that is not empty.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// StatusError is a non-200 answer from a model list endpoint.
type StatusError struct {
	Status int
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("models: status %d: %s", e.Status, e.Body)
}

func snippet(data []byte) string {
	const n = 200
	s := strings.TrimSpace(string(data))
	if len(s) > n {
		s = s[:n]
	}
	return s
}

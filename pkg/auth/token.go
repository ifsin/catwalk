package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"charm.land/catwalk/pkg/catwalk"
)

const maxBody = 1 << 20

// HTTPClient returns a client with a timeout, for flows that were not given
// one.
func HTTPClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// TokenResponse is the standard OAuth 2.0 token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Credential converts the response into a credential for provider. When the
// response carries no refresh token, the one from prev is kept.
func (r *TokenResponse) Credential(provider catwalk.InferenceProvider, prev *catwalk.Credential) *catwalk.Credential {
	c := &catwalk.Credential{
		Provider:     provider,
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		IDToken:      r.IDToken,
	}
	if r.ExpiresIn > 0 {
		c.ExpiresAt = time.Now().Add(time.Duration(r.ExpiresIn) * time.Second).Unix()
	}
	if prev != nil {
		if c.RefreshToken == "" {
			c.RefreshToken = prev.RefreshToken
		}
		if len(prev.Extra) > 0 {
			c.Extra = make(map[string]string, len(prev.Extra))
			for k, v := range prev.Extra {
				c.Extra[k] = v
			}
		}
	}
	return c
}

// RequestToken posts a form to a token endpoint. A non-200 answer is
// returned as a *TokenError.
func RequestToken(ctx context.Context, client *http.Client, endpoint string, form url.Values) (*TokenResponse, error) {
	body, err := post(ctx, client, endpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), nil)
	if err != nil {
		return nil, err
	}
	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token response contained no access token")
	}
	return &tr, nil
}

// PostForm posts a form and decodes a JSON answer into out. A non-200 answer
// is returned as a *TokenError.
func PostForm(ctx context.Context, client *http.Client, endpoint string, form url.Values, header http.Header, out any) error {
	body, err := post(ctx, client, endpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), header)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// PostJSON posts a JSON body and decodes a JSON answer into out. A non-200
// answer is returned as a *TokenError.
func PostJSON(ctx context.Context, client *http.Client, endpoint string, in, out any, header http.Header) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}
	body, err := post(ctx, client, endpoint, "application/json", strings.NewReader(string(data)), header)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// GetJSON fetches a JSON answer into out. A non-200 answer is returned as a
// *TokenError.
func GetJSON(ctx context.Context, client *http.Client, endpoint string, out any, header http.Header) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	body, err := do(client, req, header)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func post(ctx context.Context, client *http.Client, endpoint, contentType string, body io.Reader, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return do(client, req, header)
}

func do(client *http.Client, req *http.Request, header http.Header) ([]byte, error) {
	if client == nil {
		client = HTTPClient()
	}
	req.Header.Set("Accept", "application/json")
	for k, vs := range header {
		req.Header[k] = vs
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newTokenError(resp.StatusCode, data)
	}
	return data, nil
}

func newTokenError(status int, body []byte) *TokenError {
	te := &TokenError{Status: status, Body: string(body)}
	var parsed struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		te.Code = parsed.Error
		te.Description = parsed.ErrorDescription
	}
	return te
}

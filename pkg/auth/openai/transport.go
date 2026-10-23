package openai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// transport adapts requests to what the Codex backend accepts and keeps the
// credentials from reaching other hosts.
type transport struct {
	base       http.RoundTripper
	originator string
	accountID  string
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	switch strings.ToLower(clone.URL.Hostname()) {
	case "chatgpt.com":
		clone.Header.Set("originator", t.originator)
		if t.accountID != "" {
			clone.Header.Set("chatgpt-account-id", t.accountID)
		}
		clone.Body, clone.ContentLength = dropMaxOutputTokens(clone.Body, clone.ContentLength)
	case "api.openai.com", "auth.openai.com":
	default:
		clone.Header.Del("Authorization")
		clone.Header.Del("chatgpt-account-id")
	}
	return t.base.RoundTrip(clone)
}

// dropMaxOutputTokens removes the field the Codex backend rejects. Bodies
// that are not JSON objects pass through unchanged.
func dropMaxOutputTokens(body io.ReadCloser, length int64) (io.ReadCloser, int64) {
	if body == nil || body == http.NoBody || length == 0 {
		return body, length
	}
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data))
	}

	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data))
	}
	if _, ok := fields["max_output_tokens"]; !ok {
		return io.NopCloser(bytes.NewReader(data)), int64(len(data))
	}
	delete(fields, "max_output_tokens")

	cleaned, err := json.Marshal(fields)
	if err != nil {
		cleaned = data
	}
	return io.NopCloser(bytes.NewReader(cleaned)), int64(len(cleaned))
}

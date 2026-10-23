package copilot

import (
	"bytes"
	"io"
	"net/http"
)

// initiator tells Copilot whether a request was started by the user or by the
// agent, based on whether the history already holds assistant messages.
type initiator struct {
	base http.RoundTripper
}

func (t *initiator) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	who := "user"
	if clone.Body != nil && clone.Body != http.NoBody {
		data, err := io.ReadAll(clone.Body)
		_ = clone.Body.Close()
		if err != nil {
			return nil, err
		}
		clone.Body = io.NopCloser(bytes.NewReader(data))
		if assistantRole.Match(data) {
			who = "agent"
		}
	}
	clone.Header.Set("X-Initiator", who)
	return t.base.RoundTrip(clone)
}

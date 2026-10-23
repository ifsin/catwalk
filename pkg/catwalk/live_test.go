package catwalk

import "testing"

func TestRequestSourceIsSignIn(t *testing.T) {
	src := Request{BaseURL: "https://x", APIKey: "tok", Headers: map[string]string{"a": "b"}}.Source()
	if src.Auth != AuthSignIn || src.BaseURL != "https://x" || src.APIKey != "tok" || src.Headers["a"] != "b" || src.Client == nil {
		t.Fatalf("got %+v", src)
	}
}

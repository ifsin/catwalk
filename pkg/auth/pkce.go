package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// PKCE is a proof key for code exchange pair (RFC 7636, method S256).
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE generates a fresh verifier and its challenge.
func NewPKCE() (PKCE, error) {
	verifier, err := RandomString(48)
	if err != nil {
		return PKCE{}, fmt.Errorf("generate code verifier: %w", err)
	}
	return PKCE{Verifier: verifier, Challenge: Challenge(verifier)}, nil
}

// Challenge returns the S256 code challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// RandomString returns n random bytes encoded as unpadded base64url.
func RandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

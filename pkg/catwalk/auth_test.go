package catwalk

import (
	"testing"
	"time"
)

func TestCredentialExpired(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	tests := []struct {
		name      string
		expiresAt int64
		want      bool
	}{
		{"unknown", 0, true},
		{"past", now.Unix() - 10, true},
		{"inside skew", now.Add(30 * time.Second).Unix(), true},
		{"fresh", now.Add(time.Hour).Unix(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Credential{ExpiresAt: tt.expiresAt}
			if got := c.ExpiredAt(now); got != tt.want {
				t.Errorf("ExpiredAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentialRoundTrip(t *testing.T) {
	c := &Credential{Provider: "openai", AccessToken: "a", RefreshToken: "r", ExpiresAt: 5, Extra: map[string]string{"k": "v"}}
	data, err := c.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseCredential(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "a" || got.RefreshToken != "r" || got.Extra["k"] != "v" || got.Provider != "openai" {
		t.Errorf("round trip lost data: %+v", got)
	}
	clone := c.Clone()
	clone.Extra["k"] = "changed"
	if c.Extra["k"] != "v" {
		t.Error("Clone shares Extra with the original")
	}
}

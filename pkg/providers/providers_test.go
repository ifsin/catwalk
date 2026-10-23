package providers

import (
	"testing"

	"charm.land/catwalk/pkg/auth"
	"charm.land/catwalk/pkg/catwalk"
)

func TestLookup(t *testing.T) {
	tests := []struct {
		id    catwalk.InferenceProvider
		kinds []auth.Kind
	}{
		{catwalk.InferenceProviderOpenAI, []auth.Kind{auth.KindBrowser}},
		{catwalk.InferenceProviderCopilot, []auth.Kind{auth.KindDevice}},
		{catwalk.InferenceProviderXAI, []auth.Kind{auth.KindBrowser, auth.KindDevice}},
		{"hyper", []auth.Kind{auth.KindDevice}},
	}
	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			a, ok := Lookup(tt.id)
			if !ok {
				t.Fatal("no authenticator")
			}
			if a.Provider() != tt.id {
				t.Errorf("Provider() = %q", a.Provider())
			}
			for _, k := range tt.kinds {
				if !auth.Supports(a, k) {
					t.Errorf("does not support %s", k)
				}
			}
		})
	}

	if _, ok := Lookup(catwalk.InferenceProviderAnthropic); ok {
		t.Error("anthropic has no sign-in")
	}
}

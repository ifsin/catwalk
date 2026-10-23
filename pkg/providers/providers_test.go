package providers

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestAuth(t *testing.T) {
	tests := []struct {
		id    catwalk.InferenceProvider
		kinds []catwalk.Kind
	}{
		{catwalk.InferenceProviderOpenAI, []catwalk.Kind{catwalk.KindBrowser}},
		{catwalk.InferenceProviderCopilot, []catwalk.Kind{catwalk.KindDevice}},
		{catwalk.InferenceProviderXAI, []catwalk.Kind{catwalk.KindBrowser, catwalk.KindDevice}},
		{"hyper", []catwalk.Kind{catwalk.KindDevice}},
	}
	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			a := (catwalk.Provider{ID: tt.id}).Auth()
			if a == nil {
				t.Fatal("no authenticator")
			}
			if a.Provider() != tt.id {
				t.Errorf("Provider() = %q", a.Provider())
			}
			for _, k := range tt.kinds {
				if !catwalk.Supports(a, k) {
					t.Errorf("does not support %s", k)
				}
			}
		})
	}

	if (catwalk.Provider{ID: catwalk.InferenceProviderAnthropic}).Auth() != nil {
		t.Error("anthropic has no sign-in")
	}
}

func TestLister(t *testing.T) {
	for _, id := range []catwalk.InferenceProvider{catwalk.InferenceProviderOpenAI, catwalk.InferenceProviderCopilot, catwalk.InferenceProviderXAI, "hyper"} {
		if (catwalk.Provider{ID: id}).Lister() == nil {
			t.Errorf("%s has no lister", id)
		}
	}
	for _, id := range []catwalk.InferenceProvider{catwalk.InferenceProviderGemini, catwalk.InferenceProviderBedrock} {
		if (catwalk.Provider{ID: id, Type: catwalk.TypeCompletions}).Lister() != nil {
			t.Errorf("%s should have no lister", id)
		}
	}
}

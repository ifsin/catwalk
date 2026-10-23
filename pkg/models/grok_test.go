package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestModelsKeepTextModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"id":"grok-4","input_modalities":["text","image"],"output_modalities":["text"]},{"id":"grok-image","output_modalities":["image"]}]}`))
	}))
	defer srv.Close()
	got, err := Grok{}.Models(context.Background(), catwalk.Source{BaseURL: srv.URL, APIKey: "k"})
	if err != nil || len(got) != 1 || got[0].ID != "grok-4" || !got[0].Capabilities.Vision {
		t.Fatalf("got %+v, %v", got, err)
	}
}

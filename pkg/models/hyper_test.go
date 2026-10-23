package models

import (
	"context"
	"net/http"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
)

func TestHyperReadsProviderFromServiceRoot(t *testing.T) {
	srv := serve(t, 200, `{"id":"hyper","name":"Hyper","models":[{"id":"m1","name":"M1","context_window":1000},{"id":"m2","name":"M2"}]}`, func(r *http.Request) {
		if r.URL.Path != "/api/v1/provider" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("bad request %s %v", r.URL, r.Header)
		}
	})
	got, err := Hyper{}.Models(context.Background(), catwalk.Source{Auth: catwalk.AuthSignIn, BaseURL: srv.URL + "/v1", APIKey: "tok"})
	if err != nil || len(got) != 2 || got[0].ID != "m1" || got[0].ContextWindow != 1000 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

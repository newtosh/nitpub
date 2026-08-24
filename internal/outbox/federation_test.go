package outbox

import (
	"testing"

	"github.com/newtosh/nitpub/internal/store"
)

func TestSetFederation(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	created, _, err := svc.CreatePost(KindNote, "hello")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(created.ID)

	updated, err := svc.SetFederation(slug, FederationState{Shared: false})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Federation == nil || updated.Federation.Shared {
		t.Fatalf("federation = %+v, want shared=false", updated.Federation)
	}

	got, err := svc.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Federation == nil || got.Federation.Shared {
		t.Fatalf("stored federation = %+v", got.Federation)
	}
}

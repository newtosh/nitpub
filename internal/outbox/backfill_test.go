package outbox

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/newtosh/nitpub/internal/store"
)

func TestOutboxCollectionOmitsSiteOnlyPosts(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	if _, _, err := svc.CreatePost(KindNote, "site only"); err != nil {
		t.Fatal(err)
	}
	shared, _, err := svc.CreatePost(KindNote, "federated")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := svc.SetFederation(PostSlug(shared.ID), FederationState{
		Shared:   true,
		SharedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}

	col, err := svc.OutboxCollection()
	if err != nil {
		t.Fatal(err)
	}
	if col.TotalItems != 1 {
		t.Fatalf("totalItems = %d, want 1", col.TotalItems)
	}
}

func TestBackfillFederationSkipsDeliveredPosts(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	delivered, _, err := svc.CreatePost(KindNote, "already sent")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := svc.SetFederation(PostSlug(delivered.ID), FederationState{
		Shared:   true,
		SharedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreatePost(KindNote, "needs send"); err != nil {
		t.Fatal(err)
	}

	deliveries := 0
	result, err := svc.BackfillFederation(func(activity any) error {
		deliveries++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 || result.Skipped != 1 || deliveries != 1 {
		t.Fatalf("result = %+v deliveries = %d", result, deliveries)
	}
}

func TestBackfillFederationSkipsDrafts(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	if _, _, err := svc.CreatePost(KindNote, "needs send"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveDraft(KindNote, "", "a draft, never published", uuid.NewString()); err != nil {
		t.Fatal(err)
	}

	deliveries := 0
	result, err := svc.BackfillFederation(func(activity any) error {
		deliveries++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 || deliveries != 1 {
		t.Fatalf("expected exactly one delivery (draft skipped), result = %+v deliveries = %d", result, deliveries)
	}
}

func TestBackfillFederationRetriesFailedDelivery(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, _, err := svc.CreatePost(KindNote, "retry me")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetFederation(PostSlug(post.ID), FederationState{
		Shared: true,
		Error:  "delivery failed",
	}); err != nil {
		t.Fatal(err)
	}

	result, err := svc.BackfillFederation(func(activity any) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 {
		t.Fatalf("sent = %d, want 1", result.Sent)
	}
	got, err := svc.GetPost(PostSlug(post.ID))
	if err != nil {
		t.Fatal(err)
	}
	if got.Federation == nil || got.Federation.Error != "" {
		t.Fatalf("federation = %+v", got.Federation)
	}
}

func TestRedeliverSharedPosts(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, _, err := svc.CreatePost(KindNote, "already sent")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := svc.SetFederation(PostSlug(post.ID), FederationState{
		Shared:   true,
		SharedAt: &now,
	}); err != nil {
		t.Fatal(err)
	}

	deliveries := 0
	result, err := svc.RedeliverSharedPosts(func(activity any) error {
		deliveries++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 || deliveries != 1 {
		t.Fatalf("result = %+v deliveries = %d", result, deliveries)
	}
}

func TestPrepareFederatedDeliveryIncludesContext(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, _, err := svc.CreatePost(KindNote, "hello fediverse")
	if err != nil {
		t.Fatal(err)
	}

	create, err := svc.PrepareFederatedDelivery(*post)
	if err != nil {
		t.Fatal(err)
	}
	if create.Context == nil {
		t.Fatal("expected ActivityBaseURI context")
	}
}

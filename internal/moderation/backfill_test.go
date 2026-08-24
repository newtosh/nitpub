package moderation

import (
	"encoding/json"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

const backfillBaseURL = "https://nitpub.example"

// allPostsExist is a postExists stub for tests that don't care about slug
// validation — every candidate slug is treated as a known local post.
func allPostsExist(string) bool { return true }

func seedRawInboxActivity(t *testing.T, st *store.Store, id, actor, postSlug, content string) {
	t.Helper()
	activity := map[string]any{
		"id":    id,
		"type":  "Create",
		"actor": actor,
		"object": map[string]any{
			"id":        id + "/object",
			"type":      "Note",
			"inReplyTo": backfillBaseURL + "/posts/" + postSlug,
			"content":   content,
		},
	}
	raw, err := json.Marshal(activity)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = st.DB().Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketInbox)).Put([]byte(id), raw)
	})
	if err != nil {
		t.Fatalf("seed inbox bucket: %v", err)
	}
}

func TestRunBackfillOnceMigratesRawReplies(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	seedRawInboxActivity(t, st, "act-1", "actor-a", "post-a", "first")
	seedRawInboxActivity(t, st, "act-2", "actor-b", "post-a", "second")
	seedRawInboxActivity(t, st, "act-3", "actor-c", "post-b", "third")

	svc := New(st)
	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("RunBackfillOnce: %v", err)
	}

	postA, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost post-a: %v", err)
	}
	if len(postA) != 2 {
		t.Fatalf("expected 2 migrated replies for post-a, got %d: %+v", len(postA), postA)
	}
	for _, r := range postA {
		if r.Status != StatusPending {
			t.Fatalf("backfilled entries must land as pending, got %+v", r)
		}
		if r.Verified {
			t.Fatalf("backfilled entries must be marked unverified (no signature survives for historical data), got %+v", r)
		}
	}

	postB, err := svc.RepliesForPost("post-b")
	if err != nil || len(postB) != 1 {
		t.Fatalf("expected 1 migrated reply for post-b, got %v %+v", err, postB)
	}
}

func TestRunBackfillOnceIsIdempotent(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	seedRawInboxActivity(t, st, "act-1", "actor-a", "post-a", "first")

	svc := New(st)
	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("first RunBackfillOnce: %v", err)
	}
	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("second RunBackfillOnce: %v", err)
	}

	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 reply after two backfill runs, got %d", len(got))
	}
}

func TestRunBackfillOnceSkipsWhenMarkerAlreadySet(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := New(st)
	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("first RunBackfillOnce (nothing to migrate): %v", err)
	}

	// Seed an entry *after* the marker is already set — a real backfill
	// candidate that should be left untouched by a second run.
	seedRawInboxActivity(t, st, "act-late", "actor-a", "post-a", "late")

	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("second RunBackfillOnce: %v", err)
	}

	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected marker-gated backfill to skip entirely on second run, got %+v", got)
	}
}

func TestRunBackfillOnceRecoversFromPartialPriorRun(t *testing.T) {
	// Simulates a crash mid-backfill: some entries were already migrated
	// (via the per-record transaction SaveReply already committed) but the
	// completion marker was never written because the pass didn't finish.
	// A fresh RunBackfillOnce call must pick up the remaining entries — the
	// already-migrated one is a no-op via SaveReply's activity-ID dedup, not
	// a duplicate.
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	seedRawInboxActivity(t, st, "act-1", "actor-a", "post-a", "first")
	seedRawInboxActivity(t, st, "act-2", "actor-b", "post-a", "second")

	svc := New(st)
	// Simulate the partial prior run: act-1 already migrated, marker unset.
	if err := svc.SaveReply(Reply{ActivityID: "act-1", PostSlug: "post-a", Actor: "actor-a", Content: "first", Status: StatusPending}); err != nil {
		t.Fatalf("seed partial migration: %v", err)
	}

	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("recovery RunBackfillOnce: %v", err)
	}

	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected both entries present after recovery (no loss, no duplication), got %d: %+v", len(got), got)
	}

	done, err := svc.backfillMarkerSet()
	if err != nil {
		t.Fatalf("backfillMarkerSet: %v", err)
	}
	if !done {
		t.Fatalf("expected the completion marker to be set after a successful recovery pass")
	}
}

func TestRunBackfillOnceSkipsUnknownPostSlugs(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	seedRawInboxActivity(t, st, "act-1", "actor-a", "no-such-post", "reply")

	svc := New(st)
	noPostsExist := func(string) bool { return false }
	if err := svc.RunBackfillOnce(noPostsExist); err != nil {
		t.Fatalf("RunBackfillOnce: %v", err)
	}

	got, err := svc.RepliesForPost("no-such-post")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected a reply to an unknown slug to be skipped, got %+v", got)
	}
}

func TestRunBackfillOnceToleratesLegacyBaseURL(t *testing.T) {
	// Backfilled activities can predate a domain migration: their raw
	// inReplyTo may carry a base URL that no longer matches the current
	// config, even though the post itself still exists (its stored IRI
	// already gets rewritten on migration). Backfill must recognize the
	// reply by slug + postExists, not by an exact current-baseURL prefix.
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	activity := map[string]any{
		"id":    "act-1",
		"type":  "Create",
		"actor": "actor-a",
		"object": map[string]any{
			"id":        "act-1/object",
			"type":      "Note",
			"inReplyTo": "https://old-domain.example/posts/post-a",
			"content":   "reply under a retired domain",
		},
	}
	raw, err := json.Marshal(activity)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = st.DB().Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketInbox)).Put([]byte("act-1"), raw)
	})
	if err != nil {
		t.Fatalf("seed inbox bucket: %v", err)
	}

	svc := New(st)
	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("RunBackfillOnce: %v", err)
	}

	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected the legacy-domain reply to migrate under its post slug, got %+v", got)
	}
}

func TestRunBackfillOnceIgnoresNonReplyAndForeignPostActivities(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// A Follow-shaped activity sharing the inbox bucket (no inReplyTo at all).
	follow := map[string]any{"id": "follow-1", "type": "Follow", "actor": "actor-a"}
	raw, _ := json.Marshal(follow)
	err = st.DB().Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketInbox)).Put([]byte("follow-1"), raw)
	})
	if err != nil {
		t.Fatalf("seed follow activity: %v", err)
	}
	seedRawInboxActivity(t, st, "act-1", "actor-a", "post-a", "reply")

	svc := New(st)
	if err := svc.RunBackfillOnce(allPostsExist); err != nil {
		t.Fatalf("RunBackfillOnce: %v", err)
	}

	pending, err := svc.PendingReplies()
	if err != nil {
		t.Fatalf("PendingReplies: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected only the reply activity to be migrated, got %d: %+v", len(pending), pending)
	}
}

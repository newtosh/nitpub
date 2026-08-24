package moderation

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func TestSaveReplyRoundTrips(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{
		ActivityID: "https://remote.example/activities/1",
		PostSlug:   "post-a",
		Actor:      "https://remote.example/users/alice",
		Content:    "hello world",
		AuthorName: "Alice",
		Status:     StatusPending,
	}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(got))
	}
	if got[0].Actor != reply.Actor || got[0].Content != reply.Content {
		t.Fatalf("fields did not round-trip: %+v", got[0])
	}
}

func TestSaveReplyDuplicateActivityIsNoOp(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{ActivityID: "act-1", PostSlug: "post-a", Actor: "actor-a", Content: "hi", Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("first SaveReply: %v", err)
	}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("duplicate SaveReply: %v", err)
	}
	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 reply after duplicate save, got %d", len(got))
	}
}

func TestSaveReplyConcurrentDuplicateActivityID(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{ActivityID: "act-race", PostSlug: "post-a", Actor: "actor-a", Content: "hi", Status: StatusPending}

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = svc.SaveReply(reply)
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent SaveReply returned error: %v", err)
		}
	}

	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 reply after concurrent duplicate saves, got %d", len(got))
	}
}

func TestSaveReplyRefusesToOverwriteOccupiedKey(t *testing.T) {
	// A genuine cross-activity hash collision is cryptographically infeasible
	// with full-length SHA-256 (KTD2), so this exercises the defensive branch
	// directly: the replies bucket already holds an entry at the key a new
	// save would compute, but (simulating an inconsistent/crash-recovered
	// index) the activity-ID index does not yet know about it. SaveReply must
	// refuse to overwrite rather than silently clobber the existing entry.
	r := Reply{ActivityID: "act-1", PostSlug: "post-a", Actor: "actor-a", Content: "first", Status: StatusPending, orderingValue: "0000000000000000001"}
	key := compositeKey(r.PostSlug, r.orderingValue, r.ActivityID)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := New(st)

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	err = st.DB().Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketReplies)).Put([]byte(key), raw)
	})
	if err != nil {
		t.Fatalf("seed replies bucket: %v", err)
	}

	second := Reply{ActivityID: "act-1", PostSlug: "post-a", Actor: "actor-b", Content: "second", Status: StatusPending, orderingValue: "0000000000000000001"}
	if err := svc.saveReplyWithOrdering(second); err == nil {
		t.Fatalf("expected refusal to overwrite an occupied key, got nil error")
	}

	got, err := svc.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 || got[0].Content != "first" {
		t.Fatalf("existing entry must not be overwritten, got %+v", got)
	}
}

func TestRepliesForPostEmptyReturnsEmptySlice(t *testing.T) {
	svc := newTestService(t)
	got, err := svc.RepliesForPost("no-such-post")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %+v", got)
	}
}

func TestApprovedRepliesForPostExcludesPendingAndRejected(t *testing.T) {
	svc := newTestService(t)
	must := func(r Reply) {
		t.Helper()
		if err := svc.SaveReply(r); err != nil {
			t.Fatalf("SaveReply: %v", err)
		}
	}
	must(Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "pending", Status: StatusPending})
	must(Reply{ActivityID: "a2", PostSlug: "p", Actor: "x", Content: "approved", Status: StatusApproved})
	must(Reply{ActivityID: "a3", PostSlug: "p", Actor: "x", Content: "rejected", Status: StatusRejected})

	got, err := svc.ApprovedRepliesForPost("p")
	if err != nil {
		t.Fatalf("ApprovedRepliesForPost: %v", err)
	}
	if len(got) != 1 || got[0].Content != "approved" {
		t.Fatalf("expected only the approved reply, got %+v", got)
	}
}

func TestApprovedReplyCountExcludesPendingAndRejected(t *testing.T) {
	svc := newTestService(t)
	must := func(r Reply) {
		t.Helper()
		if err := svc.SaveReply(r); err != nil {
			t.Fatalf("SaveReply: %v", err)
		}
	}
	must(Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "pending", Status: StatusPending})
	must(Reply{ActivityID: "a2", PostSlug: "p", Actor: "x", Content: "approved-1", Status: StatusApproved})
	must(Reply{ActivityID: "a3", PostSlug: "p", Actor: "x", Content: "approved-2", Status: StatusApproved})
	must(Reply{ActivityID: "a4", PostSlug: "p", Actor: "x", Content: "rejected", Status: StatusRejected})

	n, err := svc.ApprovedReplyCount("p")
	if err != nil {
		t.Fatalf("ApprovedReplyCount: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected count 2, got %d", n)
	}
}

func TestApprovedReplyCountZeroForPostWithNoReplies(t *testing.T) {
	svc := newTestService(t)
	n, err := svc.ApprovedReplyCount("no-such-post")
	if err != nil {
		t.Fatalf("ApprovedReplyCount: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected count 0, got %d", n)
	}
}

func TestPendingRepliesAcrossPosts(t *testing.T) {
	svc := newTestService(t)
	must := func(r Reply) {
		t.Helper()
		if err := svc.SaveReply(r); err != nil {
			t.Fatalf("SaveReply: %v", err)
		}
	}
	must(Reply{ActivityID: "a1", PostSlug: "p1", Actor: "x", Content: "pending-1", Status: StatusPending})
	must(Reply{ActivityID: "a2", PostSlug: "p2", Actor: "x", Content: "pending-2", Status: StatusPending})
	must(Reply{ActivityID: "a3", PostSlug: "p1", Actor: "x", Content: "approved", Status: StatusApproved})

	got, err := svc.PendingReplies()
	if err != nil {
		t.Fatalf("PendingReplies: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 pending replies, got %d: %+v", len(got), got)
	}
}

func TestSetReplyStatusApproveTransition(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "c", Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	all, err := svc.RepliesForPost("p")
	if err != nil || len(all) != 1 {
		t.Fatalf("RepliesForPost: %v %+v", err, all)
	}
	key := all[0].Key

	if err := svc.SetReplyStatus(key, StatusApproved); err != nil {
		t.Fatalf("SetReplyStatus approve: %v", err)
	}
	approved, err := svc.ApprovedRepliesForPost("p")
	if err != nil || len(approved) != 1 {
		t.Fatalf("expected approved reply, got %v %+v", err, approved)
	}
}

func TestSetReplyStatusRejectTransition(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "c", Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	all, _ := svc.RepliesForPost("p")
	key := all[0].Key

	if err := svc.SetReplyStatus(key, StatusRejected); err != nil {
		t.Fatalf("SetReplyStatus reject: %v", err)
	}
	approved, err := svc.ApprovedRepliesForPost("p")
	if err != nil || len(approved) != 0 {
		t.Fatalf("expected no approved replies after reject, got %v %+v", err, approved)
	}
}

func TestSetReplyStatusRejectsLateralActionedTransition(t *testing.T) {
	// Moving directly between two actioned states (e.g. approved straight to
	// rejected) must fail -- a revert always goes through pending first, so
	// "revert" means the same thing everywhere the UI offers it.
	svc := newTestService(t)
	reply := Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "c", Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	all, _ := svc.RepliesForPost("p")
	key := all[0].Key

	if err := svc.SetReplyStatus(key, StatusApproved); err != nil {
		t.Fatalf("first SetReplyStatus (approve): %v", err)
	}
	err := svc.SetReplyStatus(key, StatusRejected)
	if !errors.Is(err, ErrInvalidStatusTransition) {
		t.Fatalf("expected ErrInvalidStatusTransition moving approved straight to rejected, got %v", err)
	}

	approved, err := svc.ApprovedRepliesForPost("p")
	if err != nil || len(approved) != 1 {
		t.Fatalf("rejected lateral-transition attempt must not change the status, got %v %+v", err, approved)
	}
}

func TestSetReplyStatusRevertsToPending(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "c", Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	all, _ := svc.RepliesForPost("p")
	key := all[0].Key

	if err := svc.SetReplyStatus(key, StatusSkipped); err != nil {
		t.Fatalf("SetReplyStatus skip: %v", err)
	}
	if err := svc.SetReplyStatus(key, StatusPending); err != nil {
		t.Fatalf("SetReplyStatus revert to pending: %v", err)
	}

	pending, err := svc.PendingReplies()
	if err != nil || len(pending) != 1 {
		t.Fatalf("expected reverted reply back in the pending queue, got %v %+v", err, pending)
	}
}

func TestReviewedRepliesExcludesPending(t *testing.T) {
	svc := newTestService(t)
	for _, r := range []Reply{
		{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: "c1", Status: StatusPending},
		{ActivityID: "a2", PostSlug: "p", Actor: "x", Content: "c2", Status: StatusApproved},
		{ActivityID: "a3", PostSlug: "p", Actor: "x", Content: "c3", Status: StatusRejected},
		{ActivityID: "a4", PostSlug: "p", Actor: "x", Content: "c4", Status: StatusSkipped},
	} {
		if err := svc.SaveReply(r); err != nil {
			t.Fatalf("SaveReply: %v", err)
		}
	}

	reviewed, err := svc.ReviewedReplies()
	if err != nil {
		t.Fatalf("ReviewedReplies: %v", err)
	}
	if len(reviewed) != 3 {
		t.Fatalf("expected 3 reviewed replies (approved+rejected+skipped), got %d: %+v", len(reviewed), reviewed)
	}
	for _, r := range reviewed {
		if r.Status == StatusPending {
			t.Fatalf("ReviewedReplies must not include pending entries, got %+v", r)
		}
	}
}

func TestSetReplyStatusUnknownKeyReturnsNotFound(t *testing.T) {
	svc := newTestService(t)
	err := svc.SetReplyStatus("no-such-key", StatusApproved)
	if !errors.Is(err, ErrReplyNotFound) {
		t.Fatalf("expected ErrReplyNotFound, got %v", err)
	}
}

func TestTrustedAndBlockedActorsRoundTrip(t *testing.T) {
	svc := newTestService(t)
	actor := "https://remote.example/users/alice"

	trusted, err := svc.IsTrusted(actor)
	if err != nil || trusted {
		t.Fatalf("expected not trusted initially, got %v %v", trusted, err)
	}
	if err := svc.AddTrusted(actor); err != nil {
		t.Fatalf("AddTrusted: %v", err)
	}
	trusted, err = svc.IsTrusted(actor)
	if err != nil || !trusted {
		t.Fatalf("expected trusted after AddTrusted, got %v %v", trusted, err)
	}
	if err := svc.RemoveTrusted(actor); err != nil {
		t.Fatalf("RemoveTrusted: %v", err)
	}
	trusted, err = svc.IsTrusted(actor)
	if err != nil || trusted {
		t.Fatalf("expected not trusted after RemoveTrusted, got %v %v", trusted, err)
	}

	blocked, err := svc.IsBlocked(actor)
	if err != nil || blocked {
		t.Fatalf("expected not blocked initially, got %v %v", blocked, err)
	}
	if err := svc.AddBlocked(actor); err != nil {
		t.Fatalf("AddBlocked: %v", err)
	}
	blocked, err = svc.IsBlocked(actor)
	if err != nil || !blocked {
		t.Fatalf("expected blocked after AddBlocked, got %v %v", blocked, err)
	}
	if err := svc.RemoveBlocked(actor); err != nil {
		t.Fatalf("RemoveBlocked: %v", err)
	}
	blocked, err = svc.IsBlocked(actor)
	if err != nil || blocked {
		t.Fatalf("expected not blocked after RemoveBlocked, got %v %v", blocked, err)
	}
}

func TestSaveReplySanitizesScriptPayloads(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{
		ActivityID: "a1",
		PostSlug:   "p",
		Actor:      "x",
		Content:    `<p>hi</p><script>alert(1)</script><img src=x onerror=alert(2)><a href="javascript:alert(3)">click</a>`,
		AuthorName: `<script>alert(4)</script>Mallory`,
		Status:     StatusPending,
	}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	got, err := svc.RepliesForPost("p")
	if err != nil || len(got) != 1 {
		t.Fatalf("RepliesForPost: %v %+v", err, got)
	}
	stored := got[0]
	for _, bad := range []string{"<script", "onerror=", "javascript:"} {
		if strings.Contains(strings.ToLower(stored.Content), bad) {
			t.Fatalf("stored content still contains %q: %q", bad, stored.Content)
		}
		if strings.Contains(strings.ToLower(stored.AuthorName), bad) {
			t.Fatalf("stored author name still contains %q: %q", bad, stored.AuthorName)
		}
	}
	if strings.Contains(stored.AuthorName, "<") {
		t.Fatalf("author name must be plain text with no HTML retained, got %q", stored.AuthorName)
	}
}

func TestSaveReplyTruncatesOversizedContent(t *testing.T) {
	svc := newTestService(t)
	longContent := strings.Repeat("a", maxContentBytes+500)
	longName := strings.Repeat("b", maxAuthorNameBytes+50)
	reply := Reply{ActivityID: "a1", PostSlug: "p", Actor: "x", Content: longContent, AuthorName: longName, Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	got, err := svc.RepliesForPost("p")
	if err != nil || len(got) != 1 {
		t.Fatalf("RepliesForPost: %v %+v", err, got)
	}
	if len(got[0].Content) > maxContentBytes {
		t.Fatalf("content not truncated: %d bytes", len(got[0].Content))
	}
	if len(got[0].AuthorName) > maxAuthorNameBytes {
		t.Fatalf("author name not truncated: %d bytes", len(got[0].AuthorName))
	}
}

func TestCompositeKeySlugContainingColon(t *testing.T) {
	svc := newTestService(t)
	// Slugs are UUIDs in this codebase and never contain ':', but verify the
	// key scheme doesn't silently corrupt if one did.
	reply := Reply{ActivityID: "a1", PostSlug: "weird:slug", Actor: "x", Content: "c", Status: StatusPending}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}
	got, err := svc.RepliesForPost("weird:slug")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(got))
	}
}

func TestFindByObjectIDResolvesParent(t *testing.T) {
	svc := newTestService(t)
	reply := Reply{
		ActivityID: "https://remote.example/activities/1",
		PostSlug:   "post-a",
		Actor:      "https://remote.example/users/alice",
		Content:    "hello",
		ObjectID:   "https://remote.example/statuses/1",
		Status:     StatusPending,
	}
	if err := svc.SaveReply(reply); err != nil {
		t.Fatalf("SaveReply: %v", err)
	}

	found, err := svc.FindByObjectID("https://remote.example/statuses/1")
	if err != nil {
		t.Fatalf("FindByObjectID: %v", err)
	}
	if found == nil || found.PostSlug != "post-a" {
		t.Fatalf("expected to resolve the parent reply's PostSlug, got %+v", found)
	}
}

func TestFindByObjectIDUnknownReturnsNilNotError(t *testing.T) {
	svc := newTestService(t)
	found, err := svc.FindByObjectID("https://remote.example/statuses/does-not-exist")
	if err != nil {
		t.Fatalf("FindByObjectID: %v", err)
	}
	if found != nil {
		t.Fatalf("expected nil for an unknown object id, got %+v", found)
	}
}

func TestFindByObjectIDEmptyReturnsNilNotError(t *testing.T) {
	svc := newTestService(t)
	found, err := svc.FindByObjectID("")
	if err != nil {
		t.Fatalf("FindByObjectID: %v", err)
	}
	if found != nil {
		t.Fatalf("expected nil for an empty object id, got %+v", found)
	}
}

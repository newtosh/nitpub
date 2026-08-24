package inbox

import (
	"net/http/httptest"
	"testing"

	vocab "github.com/go-ap/activitypub"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/federation"
	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

const testBaseURL = "https://nitpub.example"

func newTestHandler(t *testing.T) (*Handler, *moderation.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	actorIRI := vocab.IRI(testBaseURL + "/actor")
	ap := apstore.New(st, actorIRI)
	ob := outbox.New(st, testBaseURL, string(actorIRI))
	verify := federation.NewVerifier(ap, nil)
	mod := moderation.New(st)

	h := NewHandler(verify, ap, ob, nil, string(actorIRI), testBaseURL, mod, nil)
	return h, mod
}

func replyActivity(id, actor, inReplyTo, content string) map[string]any {
	return map[string]any{
		"id":    id,
		"type":  "Create",
		"actor": actor,
		"object": map[string]any{
			"id":        id + "/object",
			"type":      "Note",
			"inReplyTo": inReplyTo,
			"content":   content,
		},
	}
}

func TestHandleCreateReplyDefaultsToPending(t *testing.T) {
	h, mod := newTestHandler(t)
	postIRI := testBaseURL + "/posts/post-a"
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity("https://remote.example/activities/1", "https://remote.example/users/alice", postIRI, "hello"))

	replies, err := mod.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(replies) != 1 || replies[0].Status != moderation.StatusPending {
		t.Fatalf("expected 1 pending reply, got %+v", replies)
	}
	if replies[0].Actor != string(remote.ID) {
		t.Fatalf("expected reply actor to be the verified remoteActor %q, got %q", remote.ID, replies[0].Actor)
	}
}

func TestHandleCreateReplyFromTrustedActorAutoApproves(t *testing.T) {
	h, mod := newTestHandler(t)
	postIRI := testBaseURL + "/posts/post-a"
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}
	if err := mod.AddTrusted(string(remote.ID)); err != nil {
		t.Fatalf("AddTrusted: %v", err)
	}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity("https://remote.example/activities/2", string(remote.ID), postIRI, "hello"))

	replies, err := mod.ApprovedRepliesForPost("post-a")
	if err != nil {
		t.Fatalf("ApprovedRepliesForPost: %v", err)
	}
	if len(replies) != 1 {
		t.Fatalf("expected 1 auto-approved reply, got %+v", replies)
	}
}

func TestHandleCreateReplyFromBlockedActorAutoRejects(t *testing.T) {
	h, mod := newTestHandler(t)
	postIRI := testBaseURL + "/posts/post-a"
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/mallory")}
	if err := mod.AddBlocked(string(remote.ID)); err != nil {
		t.Fatalf("AddBlocked: %v", err)
	}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity("https://remote.example/activities/3", string(remote.ID), postIRI, "spam"))

	approved, err := mod.ApprovedRepliesForPost("post-a")
	if err != nil {
		t.Fatalf("ApprovedRepliesForPost: %v", err)
	}
	if len(approved) != 0 {
		t.Fatalf("expected no approved replies for a blocked actor, got %+v", approved)
	}
	all, err := mod.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(all) != 1 || all[0].Status != moderation.StatusRejected {
		t.Fatalf("expected 1 rejected reply, got %+v", all)
	}
}

func TestHandleCreateReplyIgnoresBodyClaimedActor(t *testing.T) {
	// A signed request cannot claim a different (trusted) actor's identity in
	// the activity body — trust/block lookups and the persisted Reply.Actor
	// must use the HTTP-signature-verified remoteActor (R11), not
	// activity["actor"].
	h, mod := newTestHandler(t)
	postIRI := testBaseURL + "/posts/post-a"
	trustedURI := "https://remote.example/users/trusted-alice"
	if err := mod.AddTrusted(trustedURI); err != nil {
		t.Fatalf("AddTrusted: %v", err)
	}
	// The verified signer is a different, non-trusted actor, but the JSON
	// body claims to be the trusted one.
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/attacker")}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity("https://remote.example/activities/4", trustedURI, postIRI, "spoofed"))

	approved, err := mod.ApprovedRepliesForPost("post-a")
	if err != nil {
		t.Fatalf("ApprovedRepliesForPost: %v", err)
	}
	if len(approved) != 0 {
		t.Fatalf("body-claimed trusted actor must not auto-approve; got %+v", approved)
	}
	all, err := mod.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(all) != 1 || all[0].Actor != string(remote.ID) {
		t.Fatalf("expected reply stored under the verified actor %q, got %+v", remote.ID, all)
	}
}

func TestHandleCreateNonReplyActivityUnaffected(t *testing.T) {
	h, mod := newTestHandler(t)
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}
	activity := map[string]any{
		"id":    "https://remote.example/activities/5",
		"type":  "Create",
		"actor": string(remote.ID),
		"object": map[string]any{
			"id":      "https://remote.example/activities/5/object",
			"type":    "Note",
			"content": "not a reply",
		},
	}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, activity)

	// No inReplyTo targeting a local post: nothing should land in moderation storage.
	all, err := mod.PendingReplies()
	if err != nil {
		t.Fatalf("PendingReplies: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected no moderation entries for a non-reply activity, got %+v", all)
	}
}

func TestHandleCreateDuplicateDeliveryIsIdempotent(t *testing.T) {
	h, mod := newTestHandler(t)
	postIRI := testBaseURL + "/posts/post-a"
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}
	activity := replyActivity("https://remote.example/activities/6", string(remote.ID), postIRI, "hello")

	h.handleCreate(httptest.NewRecorder(), remote, activity)
	h.handleCreate(httptest.NewRecorder(), remote, activity)

	all, err := mod.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 reply after duplicate delivery, got %d", len(all))
	}
}

func TestHandleCreateNestedReplyThreadsOntoSamePost(t *testing.T) {
	h, mod := newTestHandler(t)
	postIRI := testBaseURL + "/posts/post-a"
	alice := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}
	bob := vocab.Actor{ID: vocab.ID("https://other.example/users/bob")}

	// Alice replies directly to the post.
	parentActivityID := "https://remote.example/activities/10"
	parentObjectID := parentActivityID + "/object"
	h.handleCreate(httptest.NewRecorder(), alice, replyActivity(parentActivityID, string(alice.ID), postIRI, "top-level"))

	// Bob replies to Alice's reply, not to the post — inReplyTo targets
	// Alice's reply's own object id.
	childActivityID := "https://other.example/activities/11"
	w := httptest.NewRecorder()
	h.handleCreate(w, bob, replyActivity(childActivityID, string(bob.ID), parentObjectID, "nested reply"))

	all, err := mod.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both the top-level and nested reply threaded onto post-a, got %+v", all)
	}

	var parent, child *moderation.Reply
	for i := range all {
		switch all[i].Actor {
		case string(alice.ID):
			parent = &all[i]
		case string(bob.ID):
			child = &all[i]
		}
	}
	if parent == nil {
		t.Fatalf("expected to find Alice's top-level reply among %+v", all)
	}
	if parent.Nested {
		t.Fatalf("expected top-level reply Nested=false, got true")
	}
	if child == nil {
		t.Fatalf("expected to find Bob's nested reply among %+v", all)
	}
	if !child.Nested {
		t.Fatalf("expected nested reply Nested=true, got false")
	}
	if child.InReplyTo != parentObjectID {
		t.Fatalf("expected nested reply's InReplyTo %q to be Alice's object id %q", child.InReplyTo, parentObjectID)
	}
	if child.ObjectID != childActivityID+"/object" {
		t.Fatalf("expected nested reply's own ObjectID to be set, got %q", child.ObjectID)
	}
	if child.ParentActor != string(alice.ID) {
		t.Fatalf("expected nested reply's ParentActor to be Alice's actor URI %q, got %q", alice.ID, child.ParentActor)
	}
}

func TestHandleCreateReplyToUnknownParentIsDropped(t *testing.T) {
	// A reply whose inReplyTo neither matches a local post nor any reply we
	// already track (an unrelated remote conversation, or a reply to a
	// parent we never saw) must be silently dropped, not stored orphaned.
	h, mod := newTestHandler(t)
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity(
		"https://remote.example/activities/12",
		string(remote.ID),
		"https://unrelated.example/statuses/999",
		"replying to something we've never heard of",
	))

	all, err := mod.PendingReplies()
	if err != nil {
		t.Fatalf("PendingReplies: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected no moderation entries for a reply to an unknown parent, got %+v", all)
	}
}

func TestHandleCreateModerationDisabledAutoApproves(t *testing.T) {
	h, mod := newTestHandler(t)
	h.moderationEnabled = func() bool { return false }
	postIRI := testBaseURL + "/posts/post-a"
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/alice")}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity("https://remote.example/activities/13", string(remote.ID), postIRI, "hello"))

	approved, err := mod.ApprovedRepliesForPost("post-a")
	if err != nil {
		t.Fatalf("ApprovedRepliesForPost: %v", err)
	}
	if len(approved) != 1 {
		t.Fatalf("expected reply auto-approved with moderation disabled, got %+v", approved)
	}
}

func TestHandleCreateModerationDisabledStillBlocksBlockedActor(t *testing.T) {
	h, mod := newTestHandler(t)
	h.moderationEnabled = func() bool { return false }
	postIRI := testBaseURL + "/posts/post-a"
	remote := vocab.Actor{ID: vocab.ID("https://remote.example/users/mallory")}
	if err := mod.AddBlocked(string(remote.ID)); err != nil {
		t.Fatalf("AddBlocked: %v", err)
	}

	w := httptest.NewRecorder()
	h.handleCreate(w, remote, replyActivity("https://remote.example/activities/14", string(remote.ID), postIRI, "spam"))

	approved, err := mod.ApprovedRepliesForPost("post-a")
	if err != nil {
		t.Fatalf("ApprovedRepliesForPost: %v", err)
	}
	if len(approved) != 0 {
		t.Fatalf("expected blocked actor still rejected with moderation disabled, got %+v", approved)
	}
	all, err := mod.RepliesForPost("post-a")
	if err != nil {
		t.Fatalf("RepliesForPost: %v", err)
	}
	if len(all) != 1 || all[0].Status != moderation.StatusRejected {
		t.Fatalf("expected 1 rejected reply, got %+v", all)
	}
}

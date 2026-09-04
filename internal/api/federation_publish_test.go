package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/newtosh/nitpub/internal/bluesky"
	"github.com/newtosh/nitpub/internal/outbox"
)

// blueskyPDSStub is a minimal fake PDS: refreshSession always succeeds
// unless refreshErr is set, and createRecord's behavior is driven by
// createRecordFn. reachedRefresh fires (non-blocking) each time
// refreshSession is hit, letting a test observe that a background delivery
// actually reached the network without racing on completion.
type blueskyPDSStub struct {
	t              *testing.T
	createRecordFn func(w http.ResponseWriter, r *http.Request)
	reachedRefresh chan struct{}
	blockOnRefresh chan struct{} // closed to unblock, nil to not block
}

func newBlueskyPDSStub(t *testing.T) *blueskyPDSStub {
	t.Helper()
	return &blueskyPDSStub{t: t, reachedRefresh: make(chan struct{}, 8)}
}

func (s *blueskyPDSStub) server() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			select {
			case s.reachedRefresh <- struct{}{}:
			default:
			}
			if s.blockOnRefresh != nil {
				<-s.blockOnRefresh
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"did": "did:plc:test", "handle": "alice.bsky.social",
				"accessJwt": "access-1", "refreshJwt": "refresh-2",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			if s.createRecordFn != nil {
				s.createRecordFn(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/1", "cid": "cid1"})
		default:
			s.t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	s.t.Cleanup(srv.Close)
	return srv
}

func connectFakeBluesky(t *testing.T, h *Handler) {
	t.Helper()
	if err := h.blueskyAuth.Put(bluesky.Auth{DID: "did:plc:test", Handle: "alice.bsky.social", RefreshJWT: "refresh-1"}); err != nil {
		t.Fatal(err)
	}
}

func createPostReq(t *testing.T, sid, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/posts", strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	return req
}

// TestCreatePost_BlueskyAsync_ReturnsBeforeDeliveryCompletes proves publish
// doesn't block on Bluesky delivery (R3): the handler must return its HTTP
// response while the background delivery is still parked mid-network-call.
func TestCreatePost_BlueskyAsync_ReturnsBeforeDeliveryCompletes(t *testing.T) {
	stub := newBlueskyPDSStub(t)
	stub.blockOnRefresh = make(chan struct{})
	var closeOnce sync.Once
	t.Cleanup(func() { closeOnce.Do(func() { close(stub.blockOnRefresh) }) })
	client := bluesky.NewClient(stub.server().URL)
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	rec := httptest.NewRecorder()
	h.ServePosts(rec, createPostReq(t, sid, `{"kind":"note","content":"hello there","federate":false,"bluesky":true}`))

	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	if post.Bluesky == nil || post.Bluesky.Status != "pending" {
		t.Fatalf("expected pending bluesky state in the publish response, got %+v", post.Bluesky)
	}

	// The response above returned while the fake PDS is still parked on
	// refreshSession (blockOnRefresh not yet closed) — proof the handler
	// didn't wait on delivery. Now confirm the background goroutine really
	// did get there before releasing it, so the test isn't vacuous.
	select {
	case <-stub.reachedRefresh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected background delivery to reach refreshSession")
	}
	closeOnce.Do(func() { close(stub.blockOnRefresh) })

	// Let the now-unblocked goroutine actually finish before the test's
	// db closes out from under it (t.Cleanup order), rather than leaving
	// it to race the teardown.
	slug := outbox.PostSlug(post.ID)
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := h.outbox.GetPost(slug)
		if err != nil {
			t.Fatal(err)
		}
		if got.Bluesky != nil && got.Bluesky.Status == "posted" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for delivery to finish, last seen %+v", got.Bluesky)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestCreatePost_BlueskySuccess_UpdatesStateAfterGoroutine confirms a
// successful background delivery is reflected in stored post state once it
// finishes.
func TestCreatePost_BlueskySuccess_UpdatesStateAfterGoroutine(t *testing.T) {
	stub := newBlueskyPDSStub(t)
	client := bluesky.NewClient(stub.server().URL)
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	rec := httptest.NewRecorder()
	h.ServePosts(rec, createPostReq(t, sid, `{"kind":"note","content":"hello there","federate":false,"bluesky":true}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := h.outbox.GetPost(slug)
		if err != nil {
			t.Fatal(err)
		}
		if got.Bluesky != nil && got.Bluesky.Status == "posted" {
			if got.Bluesky.URI == "" {
				t.Fatalf("expected a URI on success, got %+v", got.Bluesky)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for posted state, last seen %+v", got.Bluesky)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestCreatePost_BlueskyFailure_UpdatesStateWithError mirrors the success
// case for a delivery that fails.
func TestCreatePost_BlueskyFailure_UpdatesStateWithError(t *testing.T) {
	stub := newBlueskyPDSStub(t)
	stub.createRecordFn = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "InternalServerError", "message": "boom"})
	}
	client := bluesky.NewClient(stub.server().URL)
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	rec := httptest.NewRecorder()
	h.ServePosts(rec, createPostReq(t, sid, `{"kind":"note","content":"hello there","federate":false,"bluesky":true}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := h.outbox.GetPost(slug)
		if err != nil {
			t.Fatal(err)
		}
		if got.Bluesky != nil && got.Bluesky.Status == "error" {
			if got.Bluesky.Error == "" {
				t.Fatalf("expected an error message, got %+v", got.Bluesky)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for error state, last seen %+v", got.Bluesky)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestCreatePost_BlueskyRequestedButNotConnected_SilentSkip covers KTD2:
// bluesky:true with no connected account must not error, and must leave no
// Bluesky state behind.
func TestCreatePost_BlueskyRequestedButNotConnected_SilentSkip(t *testing.T) {
	client := bluesky.NewClient("http://127.0.0.1:0") // never dialed
	h, sid := testBlueskyHandler(t, client)
	// Deliberately not calling connectFakeBluesky — no stored Auth.

	rec := httptest.NewRecorder()
	h.ServePosts(rec, createPostReq(t, sid, `{"kind":"note","content":"hello there","federate":false,"bluesky":true}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	if post.Bluesky != nil {
		t.Fatalf("expected no bluesky state when not connected, got %+v", post.Bluesky)
	}
}

func retryReq(t *testing.T, sid, slug string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/posts/"+slug+"/bluesky/retry", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	req.SetPathValue("slug", slug)
	return req
}

// TestAdminRetryBlueskyPost_NoPriorAttempt_Errors covers the retry
// endpoint's own validation: a post with no Bluesky attempt at all
// (post.Bluesky == nil) must not be silently no-op'd.
func TestAdminRetryBlueskyPost_NoPriorAttempt_Errors(t *testing.T) {
	client := bluesky.NewClient("http://127.0.0.1:0")
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	post, _, err := h.outbox.CreatePost(outbox.KindNote, "a note with no bluesky attempt")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	rec := httptest.NewRecorder()
	h.AdminRetryBlueskyPost(rec, retryReq(t, sid, slug))
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("expected 4xx, got %d body = %s", rec.Code, rec.Body.String())
	}

	got, err := h.outbox.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bluesky != nil {
		t.Fatalf("expected retry-with-no-attempt to leave state untouched, got %+v", got.Bluesky)
	}
}

// TestAdminRetryBlueskyPost_SucceedsSynchronously confirms retry re-runs
// Deliver synchronously (the caller sees the result, not a background
// goroutine).
func TestAdminRetryBlueskyPost_SucceedsSynchronously(t *testing.T) {
	stub := newBlueskyPDSStub(t)
	client := bluesky.NewClient(stub.server().URL)
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	post, _, err := h.outbox.CreatePost(outbox.KindNote, "a note to retry")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)
	if _, err := h.outbox.SetBluesky(slug, outbox.BlueskyState{Status: "error", Error: "boom"}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.AdminRetryBlueskyPost(rec, retryReq(t, sid, slug))
	if rec.Code != http.StatusOK {
		t.Fatalf("retry status = %d body = %s", rec.Code, rec.Body.String())
	}

	var got outbox.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Bluesky == nil || got.Bluesky.Status != "posted" {
		t.Fatalf("expected posted state in the retry response itself (synchronous), got %+v", got.Bluesky)
	}
}

// TestAdminRetryBlueskyPost_FailureReturnsErrorAndUpdatesState covers the
// retry failure path.
func TestAdminRetryBlueskyPost_FailureReturnsErrorAndUpdatesState(t *testing.T) {
	stub := newBlueskyPDSStub(t)
	stub.createRecordFn = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "InternalServerError", "message": "boom"})
	}
	client := bluesky.NewClient(stub.server().URL)
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	post, _, err := h.outbox.CreatePost(outbox.KindNote, "a note to retry and fail")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)
	if _, err := h.outbox.SetBluesky(slug, outbox.BlueskyState{Status: "error", Error: "prior failure"}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.AdminRetryBlueskyPost(rec, retryReq(t, sid, slug))
	if rec.Code < 400 {
		t.Fatalf("expected an error status, got %d body = %s", rec.Code, rec.Body.String())
	}

	got, err := h.outbox.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Bluesky == nil || got.Bluesky.Status != "error" || got.Bluesky.Error == "" {
		t.Fatalf("expected error state after failed retry, got %+v", got.Bluesky)
	}
}

// TestAdminRetryBlueskyPost_FreshPendingRefused proves a still-plausibly-
// in-flight pending state (younger than the staleness bound) blocks a
// concurrent retry rather than racing bluesky.Deliver against itself.
func TestAdminRetryBlueskyPost_FreshPendingRefused(t *testing.T) {
	client := bluesky.NewClient("http://127.0.0.1:0")
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	post, _, err := h.outbox.CreatePost(outbox.KindNote, "a note stuck pending")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)
	now := time.Now().UTC()
	if _, err := h.outbox.SetBluesky(slug, outbox.BlueskyState{Status: "pending", PendingSince: &now}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.AdminRetryBlueskyPost(rec, retryReq(t, sid, slug))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a fresh pending state, got %d body = %s", rec.Code, rec.Body.String())
	}
}

// TestAdminRetryBlueskyPost_StalePendingRetryable proves a pending state
// older than the 10-minute staleness bound (KTD5 — e.g. a process restart
// stranded it) is retried rather than refused as still-in-progress.
func TestAdminRetryBlueskyPost_StalePendingRetryable(t *testing.T) {
	stub := newBlueskyPDSStub(t)
	client := bluesky.NewClient(stub.server().URL)
	h, sid := testBlueskyHandler(t, client)
	connectFakeBluesky(t, h)

	post, _, err := h.outbox.CreatePost(outbox.KindNote, "a note stranded pending")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)
	stale := time.Now().UTC().Add(-11 * time.Minute)
	if _, err := h.outbox.SetBluesky(slug, outbox.BlueskyState{Status: "pending", PendingSince: &stale}); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.AdminRetryBlueskyPost(rec, retryReq(t, sid, slug))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected stale pending to be retried, got %d body = %s", rec.Code, rec.Body.String())
	}

	var got outbox.Post
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Bluesky == nil || got.Bluesky.Status != "posted" {
		t.Fatalf("expected posted after retrying a stale pending state, got %+v", got.Bluesky)
	}
}

func TestAdminRetryBlueskyPost_RequiresAuth(t *testing.T) {
	client := bluesky.NewClient("http://127.0.0.1:0")
	h, _ := testBlueskyHandler(t, client)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/some-slug/bluesky/retry", nil)
	req.SetPathValue("slug", "some-slug")
	rec := httptest.NewRecorder()
	h.AdminRetryBlueskyPost(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}
}

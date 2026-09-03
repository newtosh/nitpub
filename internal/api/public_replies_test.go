package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func TestGetPostRepliesReturnsOnlyApproved(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, mod, "", false, false)

	for _, r := range []moderation.Reply{
		{ActivityID: "a1", PostSlug: "post-a", Actor: "x", Content: "pending", Status: moderation.StatusPending},
		{ActivityID: "a2", PostSlug: "post-a", Actor: "x", Content: "approved-1", Status: moderation.StatusApproved},
		{ActivityID: "a3", PostSlug: "post-a", Actor: "x", Content: "rejected", Status: moderation.StatusRejected},
	} {
		if err := mod.SaveReply(r); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts/post-a/replies", nil)
	req.SetPathValue("id", "post-a")
	rec := httptest.NewRecorder()
	h.GetPostReplies(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got []publicReply
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Content != "approved-1" {
		t.Fatalf("expected only the approved reply, got %+v", got)
	}
}

func TestGetPostRepliesEmptyPostReturns200EmptyArray(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, mod, "", false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/no-such-post/replies", nil)
	req.SetPathValue("id", "no-such-post")
	rec := httptest.NewRecorder()
	h.GetPostReplies(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a post with zero replies (not 404), got %d body=%s", rec.Code, rec.Body.String())
	}
	var got []publicReply
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty array, got %+v", got)
	}
}

func TestGetPostRepliesResponseOmitsInternalFields(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, mod, "", false, false)
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: "post-a", Actor: "x", Content: "hi", Status: moderation.StatusApproved}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts/post-a/replies", nil)
	req.SetPathValue("id", "post-a")
	rec := httptest.NewRecorder()
	h.GetPostReplies(rec, req)

	body := rec.Body.String()
	for _, forbidden := range []string{`"status"`, `"key"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaks internal field %s: %s", forbidden, body)
		}
	}
}

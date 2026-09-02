package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func TestGetPostIncludesApprovedReplyCount(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, mod, "", false, false)

	created, _, err := ob.CreatePost(outbox.KindNote, "public read")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(created.ID)

	for _, r := range []moderation.Reply{
		{ActivityID: "a1", PostSlug: slug, Actor: "x", Content: "approved-1", Status: moderation.StatusApproved},
		{ActivityID: "a2", PostSlug: slug, Actor: "x", Content: "approved-2", Status: moderation.StatusApproved},
		{ActivityID: "a3", PostSlug: slug, Actor: "x", Content: "pending", Status: moderation.StatusPending},
	} {
		if err := mod.SaveReply(r); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts/"+slug, nil)
	req.SetPathValue("id", slug)
	rec := httptest.NewRecorder()
	h.GetPost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ReplyCount int `json:"reply_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ReplyCount != 2 {
		t.Fatalf("expected reply_count 2, got %d", resp.ReplyCount)
	}
}

func TestGetPostReplyCountZeroWhenNoModeration(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	// mod intentionally omitted (nil) -- must not error, just reply_count=0.
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false, false)

	created, _, err := ob.CreatePost(outbox.KindNote, "no moderation configured")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(created.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/"+slug, nil)
	req.SetPathValue("id", slug)
	rec := httptest.NewRecorder()
	h.GetPost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ReplyCount int `json:"reply_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ReplyCount != 0 {
		t.Fatalf("expected reply_count 0, got %d", resp.ReplyCount)
	}
}

func TestListPostsIncludesApprovedReplyCount(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, mod, "", false, false)

	created, _, err := ob.CreatePost(outbox.KindNote, "list me")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(created.ID)
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: slug, Actor: "x", Content: "hi", Status: moderation.StatusApproved}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp []struct {
		ReplyCount int `json:"reply_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) != 1 || resp[0].ReplyCount != 1 {
		t.Fatalf("expected one post with reply_count 1, got %+v", resp)
	}
}

func TestListPostsPaginatedIncludesApprovedReplyCount(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	mod := moderation.New(st)
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, mod, "", false, false)

	created, _, err := ob.CreatePost(outbox.KindNote, "list me paginated")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(created.ID)
	if err := mod.SaveReply(moderation.Reply{ActivityID: "a1", PostSlug: slug, Actor: "x", Content: "hi", Status: moderation.StatusApproved}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts?limit=10&offset=0", nil)
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Posts []struct {
			ReplyCount int `json:"reply_count"`
		} `json:"posts"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Posts) != 1 || resp.Posts[0].ReplyCount != 1 {
		t.Fatalf("expected one post with reply_count 1, got %+v", resp.Posts)
	}
}

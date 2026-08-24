package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/search"
	"github.com/newtosh/nitpub/internal/store"
)

func TestCreatePostRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	body := bytes.NewBufferString(`{"kind":"note","content":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", body)
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCreatePostAuthenticated(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	body := bytes.NewBufferString(`{"kind":"note","content":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", body)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.NewDecoder(rec.Body).Decode(&post); err != nil {
		t.Fatal(err)
	}
	if post.Content != "hello" {
		t.Fatalf("content = %q", post.Content)
	}
}

func TestCreatePostSkipsFederationWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, func(activity any) error {
		t.Fatal("deliver should not run when federate=false")
		return nil
	}, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	federate := false
	body, _ := json.Marshal(map[string]any{
		"kind":     "note",
		"content":  "site only",
		"federate": federate,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.NewDecoder(rec.Body).Decode(&post); err != nil {
		t.Fatal(err)
	}
	if post.Federation == nil || post.Federation.Shared {
		t.Fatalf("federation = %+v, want shared=false", post.Federation)
	}
}

func TestMalformedBody(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString("not json"))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestGetPostPublic(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	created, _, err := ob.CreatePost(outbox.KindNote, "public read")
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
}

func TestServePostObjectReturnsActivityJSON(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	created, _, err := ob.CreatePost(outbox.KindNote, "dereference me")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(created.ID)
	req := httptest.NewRequest(http.MethodGet, "/posts/"+slug, nil)
	req.SetPathValue("id", slug)
	rec := httptest.NewRecorder()
	h.ServePostObject(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/activity+json" {
		t.Fatalf("content-type = %q", ct)
	}
	var obj map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &obj); err != nil {
		t.Fatalf("response not valid JSON: %v (body=%s)", err, rec.Body.String())
	}
	if obj["id"] != created.ID {
		t.Fatalf("id = %v, want %q", obj["id"], created.ID)
	}
}

func TestServePostObjectDraftNotFound(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	draft, err := ob.SaveDraft(outbox.KindNote, "", "still drafting", "")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(draft.ID)
	req := httptest.NewRequest(http.MethodGet, "/posts/"+slug, nil)
	req.SetPathValue("id", slug)
	rec := httptest.NewRecorder()
	h.ServePostObject(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a draft's object", rec.Code)
	}
}

func TestGetPostMissing(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/nope", nil)
	req.SetPathValue("id", "nope")
	rec := httptest.NewRecorder()
	h.GetPost(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUpdatePostAuthenticated(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	created, _, err := ob.CreatePost(outbox.KindNote, "before")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(created.ID)

	body := bytes.NewBufferString(`{"kind":"note","content":"after"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/posts/"+slug, body)
	req.SetPathValue("id", slug)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.GetPost(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var post outbox.Post
	if err := json.NewDecoder(rec.Body).Decode(&post); err != nil {
		t.Fatal(err)
	}
	if post.Content != "after" {
		t.Fatalf("content = %q", post.Content)
	}
}

func TestUpdatePostRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodPut, "/api/posts/abc", bytes.NewBufferString(`{"kind":"note","content":"x"}`))
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()
	h.GetPost(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServeFeed(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)
	if _, _, err := ob.CreatePost(outbox.KindNote, "feed item"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	rec := httptest.NewRecorder()
	h.ServeFeed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/rss+xml; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<rss") || !strings.Contains(body, "feed item") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestSearchIndexExcludesDraft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	if _, _, err := ob.CreatePost(outbox.KindNote, "uniquekeyword published"); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.SaveDraft(outbox.KindNote, "", "uniquekeyword draft never published", ""); err != nil {
		t.Fatal(err)
	}

	// Mirror internal/server's rebuildSearch closure: it feeds the index
	// from ob.ListPosts(), which excludes drafts by default (KTD4).
	searchIdx := search.NewIndex()
	posts, err := ob.ListPosts()
	if err != nil {
		t.Fatal(err)
	}
	searchIdx.Rebuild(posts, nil)

	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, searchIdx, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/search?q=uniquekeyword", nil)
	rec := httptest.NewRecorder()
	h.ServeSearch(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "draft never published") {
		t.Fatalf("draft leaked into search results: %s", body)
	}
	if !strings.Contains(body, "published") {
		t.Fatalf("expected published post in search results: %s", body)
	}
}

func TestServeFeedExcludesDraft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)
	if _, _, err := ob.CreatePost(outbox.KindNote, "feed item"); err != nil {
		t.Fatal(err)
	}
	if _, err := ob.SaveDraft(outbox.KindNote, "", "a draft, never published", ""); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/feed.xml", nil)
	rec := httptest.NewRecorder()
	h.ServeFeed(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "a draft, never published") {
		t.Fatalf("draft leaked into feed: %s", body)
	}
	if !strings.Contains(body, "feed item") {
		t.Fatalf("expected published post in feed: %s", body)
	}
}

func TestListPostsExcludesDraftUnauthenticatedIncludesAuthenticated(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	if _, err := ob.SaveDraft(outbox.KindNote, "", "a draft", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ob.CreatePost(outbox.KindNote, "a published post"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	var publicPosts []outbox.Post
	if err := json.NewDecoder(rec.Body).Decode(&publicPosts); err != nil {
		t.Fatal(err)
	}
	if len(publicPosts) != 1 {
		t.Fatalf("expected 1 public post, got %d", len(publicPosts))
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	authedReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	authedRec := httptest.NewRecorder()
	h.ServePosts(authedRec, authedReq)
	var authedPosts []outbox.Post
	if err := json.NewDecoder(authedRec.Body).Decode(&authedPosts); err != nil {
		t.Fatal(err)
	}
	if len(authedPosts) != 2 {
		t.Fatalf("expected 2 posts for an authenticated caller, got %d", len(authedPosts))
	}
}

func TestGetPostDraftUnauthenticated404AuthenticatedFound(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	draft, err := ob.SaveDraft(outbox.KindNote, "", "shh, still drafting", "")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(draft.ID)

	req := httptest.NewRequest(http.MethodGet, "/api/posts/"+slug, nil)
	req.SetPathValue("id", slug)
	rec := httptest.NewRecorder()
	h.GetPost(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unauthenticated status = %d", rec.Code)
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/api/posts/"+slug, nil)
	authedReq.SetPathValue("id", slug)
	authedReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	authedRec := httptest.NewRecorder()
	h.GetPost(authedRec, authedReq)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d body=%s", authedRec.Code, authedRec.Body.String())
	}
}

func TestSaveDraftRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	body := bytes.NewBufferString(`{"kind":"note","content":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/posts/drafts", body)
	rec := httptest.NewRecorder()
	h.SaveDraft(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestSaveDraftAuthenticatedCreatesThenUpdatesInPlace(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	body := bytes.NewBufferString(`{"kind":"note","content":"first pass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/posts/drafts", body)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.SaveDraft(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var first outbox.Post
	if err := json.NewDecoder(rec.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(first.ID)

	body2 := bytes.NewBufferString(`{"kind":"note","content":"second pass","slug":"` + slug + `"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/posts/drafts", body2)
	req2.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec2 := httptest.NewRecorder()
	h.SaveDraft(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec2.Code, rec2.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/posts/"+slug, nil)
	getReq.SetPathValue("id", slug)
	getReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	getRec := httptest.NewRecorder()
	h.GetPost(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d", getRec.Code)
	}

	posts, err := ob.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected exactly one post at that slug, got %d", len(posts))
	}
}

func TestPublishDraftEndpointTransitionsAndReindexes(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	reindexed := false
	h := NewHandler(ob, auth, nil, nil, nil, func() { reindexed = true }, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	draft, err := ob.SaveDraft(outbox.KindNote, "", "ready to publish", "")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(draft.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/"+slug+"/publish", nil)
	req.SetPathValue("id", slug)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.PublishDraft(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !reindexed {
		t.Fatal("expected search index rebuild to be triggered on publish")
	}

	publicReq := httptest.NewRequest(http.MethodGet, "/api/posts/"+slug, nil)
	publicReq.SetPathValue("id", slug)
	publicRec := httptest.NewRecorder()
	h.GetPost(publicRec, publicReq)
	if publicRec.Code != http.StatusOK {
		t.Fatalf("expected published post visible unauthenticated, status = %d", publicRec.Code)
	}
}

func TestPublishDraftEndpointRejectsEmptyContent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false)

	draft, err := ob.SaveDraft(outbox.KindNote, "title-only note draft", "", "")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(draft.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/"+slug+"/publish", nil)
	req.SetPathValue("id", slug)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.PublishDraft(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 matching CreatePost's empty-content error shape", rec.Code)
	}
}

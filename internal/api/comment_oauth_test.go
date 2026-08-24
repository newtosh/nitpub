package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/newtosh/nitpub/internal/commentauth"
	"github.com/newtosh/nitpub/internal/mastodon"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

// fakeInstance stands in for a visitor's Mastodon-API-compatible instance.
type fakeInstance struct {
	srv            *httptest.Server
	registerCalls  int
	postReplyCalls int
	revokeCalls    int
	resolveStatus  func(w http.ResponseWriter, r *http.Request)
	postReply      func(w http.ResponseWriter, r *http.Request)
	revoke         func(w http.ResponseWriter, r *http.Request)
	verifyCreds    func(w http.ResponseWriter, r *http.Request)
	failToken      bool
}

func newFakeInstance(t *testing.T) *fakeInstance {
	t.Helper()
	f := &fakeInstance{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		f.registerCalls++
		_ = json.NewEncoder(w).Encode(map[string]string{"client_id": "cid", "client_secret": "csecret"})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if f.failToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/api/v2/search", func(w http.ResponseWriter, r *http.Request) {
		if f.resolveStatus != nil {
			f.resolveStatus(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"statuses": []map[string]string{{"id": "555", "url": r.URL.Query().Get("q")}},
		})
	})
	mux.HandleFunc("/api/v1/statuses", func(w http.ResponseWriter, r *http.Request) {
		f.postReplyCalls++
		if f.postReply != nil {
			f.postReply(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/accounts/verify_credentials", func(w http.ResponseWriter, r *http.Request) {
		if f.verifyCreds != nil {
			f.verifyCreds(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"username": "visitor", "display_name": "Visitor", "avatar": "https://x/a.png"})
	})
	mux.HandleFunc("/oauth/revoke", func(w http.ResponseWriter, r *http.Request) {
		f.revokeCalls++
		if f.revoke != nil {
			f.revoke(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeInstance) domain(t *testing.T) string {
	t.Helper()
	u, err := url.Parse(f.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func testCommentHandler(t *testing.T, client *mastodon.Client) (*CommentHandler, *outbox.Service, *commentauth.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	sessions := commentauth.NewStore(st)
	apps := mastodon.NewAppRegistrar(client, mastodon.NewAppStore(st))
	h := NewCommentHandler(ob, sessions, apps, client, "http://example.test")
	// Tests point at loopback-hosted httptest fakes, which the real KTD7
	// check correctly rejects — TestCommentAuthInvalidInstanceAtStart
	// exercises the real validator directly instead.
	h.validateInstance = func(string) error { return nil }
	return h, ob, sessions
}

func startAndFollowCallback(t *testing.T, h *CommentHandler, domain, postSlug string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"post_slug": postSlug, "instance": domain, "draft_text": "great post!"})
	req := httptest.NewRequest(http.MethodPost, "/api/comments/oauth/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.StartCommentAuth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp startCommentAuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	redirect, err := url.Parse(resp.RedirectURL)
	if err != nil {
		t.Fatal(err)
	}
	state := redirect.Query().Get("state")

	var bindingCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == commentauth.BindingCookie {
			bindingCookie = c
		}
	}
	if bindingCookie == nil {
		t.Fatal("expected binding cookie to be set")
	}

	cbReq := httptest.NewRequest(http.MethodGet, "/comment/callback?code=authcode&state="+state, nil)
	cbReq.AddCookie(bindingCookie)
	cbRec := httptest.NewRecorder()
	h.CommentAuthCallback(cbRec, cbReq)
	return cbRec
}

func TestInstanceFromInput(t *testing.T) {
	cases := map[string]string{
		"mastodon.social":        "mastodon.social",
		"alice@mastodon.social":  "mastodon.social",
		"@alice@mastodon.social": "mastodon.social",
		" mastodon.social ":      "mastodon.social",
		"@mastodon.social":       "mastodon.social",
	}
	for in, want := range cases {
		if got := instanceFromInput(in); got != want {
			t.Errorf("instanceFromInput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommentAuthStartAcceptsFullHandle(t *testing.T) {
	inst := newFakeInstance(t)
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hi")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	body, _ := json.Marshal(map[string]string{"post_slug": slug, "instance": "someone@" + inst.domain(t), "draft_text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/comments/oauth/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.StartCommentAuth(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected a full handle to be accepted, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCommentAuthFlowHappyPath(t *testing.T) {
	inst := newFakeInstance(t)
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, sessions := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	rec := startAndFollowCallback(t, h, inst.domain(t), slug)
	if rec.Code != http.StatusFound {
		t.Fatalf("callback status = %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "comment=success") {
		t.Fatalf("expected success redirect, got %s", loc)
	}
	if inst.registerCalls != 1 {
		t.Fatalf("expected 1 app registration, got %d", inst.registerCalls)
	}

	var sessCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == commentauth.SessionCookie {
			sessCookie = c
		}
	}
	if sessCookie == nil {
		t.Fatal("expected comment session cookie to be set")
	}
	sess, err := sessions.GetSession(sessCookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Handle != "visitor" || sess.Instance != inst.domain(t) {
		t.Fatalf("unexpected session: %+v", sess)
	}
}

func TestCommentAuthCallbackRejectsMissingState(t *testing.T) {
	inst := newFakeInstance(t)
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, _, _ := testCommentHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/comment/callback?code=x&state=doesnotexist", nil)
	rec := httptest.NewRecorder()
	h.CommentAuthCallback(rec, req)
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("should not 500, got %d", rec.Code)
	}
	if rec.Code < 400 {
		t.Fatalf("expected an error response, got %d", rec.Code)
	}
}

func TestCommentAuthCallbackRejectsMismatchedBindingCookie(t *testing.T) {
	inst := newFakeInstance(t)
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hi")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	body, _ := json.Marshal(map[string]string{"post_slug": slug, "instance": inst.domain(t), "draft_text": "x"})
	startReq := httptest.NewRequest(http.MethodPost, "/api/comments/oauth/start", bytes.NewReader(body))
	startRec := httptest.NewRecorder()
	h.StartCommentAuth(startRec, startReq)
	var resp startCommentAuthResponse
	_ = json.Unmarshal(startRec.Body.Bytes(), &resp)
	redirect, _ := url.Parse(resp.RedirectURL)
	state := redirect.Query().Get("state")

	cbReq := httptest.NewRequest(http.MethodGet, "/comment/callback?code=authcode&state="+state, nil)
	cbReq.AddCookie(&http.Cookie{Name: commentauth.BindingCookie, Value: "wrong-value"})
	cbRec := httptest.NewRecorder()
	h.CommentAuthCallback(cbRec, cbReq)
	if cbRec.Code < 400 {
		t.Fatalf("expected rejection on binding-cookie mismatch, got %d", cbRec.Code)
	}
}

func TestCommentAuthTokenExchangeFailurePreservesDraft(t *testing.T) {
	inst := newFakeInstance(t)
	inst.failToken = true
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hi")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	rec := startAndFollowCallback(t, h, inst.domain(t), slug)
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "comment=error_auth") {
		t.Fatalf("expected error_auth redirect, got %s", loc)
	}
	if !strings.Contains(loc, url.QueryEscape("great post!")) {
		t.Fatalf("expected draft text preserved in redirect, got %s", loc)
	}
}

func TestCommentAuthResolveFailureStillRevokes(t *testing.T) {
	inst := newFakeInstance(t)
	inst.resolveStatus = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}
	revoked := false
	inst.revoke = func(w http.ResponseWriter, r *http.Request) {
		revoked = true
		w.WriteHeader(http.StatusOK)
	}
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hi")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	startAndFollowCallback(t, h, inst.domain(t), slug)
	if !revoked {
		t.Fatal("expected token to be revoked even though ResolveStatus failed")
	}
}

func TestCommentAuthInvalidInstanceAtStart(t *testing.T) {
	client := mastodon.NewClient()
	h, ob, _ := testCommentHandler(t, client)
	h.validateInstance = mastodon.ValidateInstanceHost // exercise the real KTD7 check

	post, _, err := ob.CreatePost(outbox.KindNote, "hi")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	body, _ := json.Marshal(map[string]string{"post_slug": slug, "instance": "127.0.0.1", "draft_text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/comments/oauth/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.StartCommentAuth(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for IP-literal instance, got %d", rec.Code)
	}
}

func TestCommentAuthSkipsSessionWhenAccountLookupFails(t *testing.T) {
	inst := newFakeInstance(t)
	inst.verifyCreds = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hi")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	rec := startAndFollowCallback(t, h, inst.domain(t), slug)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected the comment to still post successfully, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "comment=success") {
		t.Fatalf("expected success redirect even without a handle, got %s", loc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == commentauth.SessionCookie {
			t.Fatalf("expected no comment-session cookie when the handle couldn't be resolved, got one: %+v", c)
		}
	}
}

func TestCommentAuthStartRejectsDraftPost(t *testing.T) {
	client := mastodon.NewClient()
	h, ob, _ := testCommentHandler(t, client)

	draft, err := ob.SaveDraft(outbox.KindNote, "", "not published yet", "")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(draft.ID)

	body, _ := json.Marshal(map[string]string{"post_slug": slug, "instance": "mastodon.social", "draft_text": "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/comments/oauth/start", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.StartCommentAuth(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a draft post (must not leak via GetPost), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCommentSessionStatusAndLogout(t *testing.T) {
	client := mastodon.NewClient()
	h, _, sessions := testCommentHandler(t, client)

	req := httptest.NewRequest(http.MethodGet, "/api/comments/session", nil)
	rec := httptest.NewRecorder()
	h.CommentSessionStatus(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 with no session, got %d", rec.Code)
	}

	id, _ := commentauth.NewSessionID()
	sess := commentauth.CreateSessionRecord(id, "mastodon.social", "alice", "Alice", "", time.Now())
	if err := sessions.PutSession(sess); err != nil {
		t.Fatal(err)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/api/comments/session", nil)
	req2.AddCookie(&http.Cookie{Name: commentauth.SessionCookie, Value: id})
	rec2 := httptest.NewRecorder()
	h.CommentSessionStatus(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	var body commentSessionResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Handle != "alice" || body.Instance != "mastodon.social" {
		t.Fatalf("unexpected session response: %+v", body)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/comments/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: commentauth.SessionCookie, Value: id})
	logoutRec := httptest.NewRecorder()
	h.CommentLogout(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on logout, got %d", logoutRec.Code)
	}
	if _, err := sessions.GetSession(id); err == nil {
		t.Fatal("expected session to be deleted after logout")
	}

	req3 := httptest.NewRequest(http.MethodGet, "/api/comments/session", nil)
	req3.AddCookie(&http.Cookie{Name: commentauth.SessionCookie, Value: id})
	rec3 := httptest.NewRecorder()
	h.CommentSessionStatus(rec3, req3)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("expected 204 after logout, got %d", rec3.Code)
	}
}

// cookieByName finds a cookie by name from a set of Set-Cookie responses,
// failing the test if it's missing.
func cookieByName(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("expected %s cookie to be set", name)
	return nil
}

func TestCommentAuthFastPathReusesCachedToken(t *testing.T) {
	inst := newFakeInstance(t)
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	cbRec := startAndFollowCallback(t, h, inst.domain(t), slug)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback status = %d", cbRec.Code)
	}
	sessCookie := cookieByName(t, cbRec.Result().Cookies(), commentauth.SessionCookie)
	tokenCookie := cookieByName(t, cbRec.Result().Cookies(), commentauth.TokenCookie)
	if inst.postReplyCalls != 1 {
		t.Fatalf("expected 1 post-reply call after first comment, got %d", inst.postReplyCalls)
	}
	registerCallsAfterFirst := inst.registerCalls

	// Second comment: carries the session + cached token cookies, no
	// binding cookie (no OAuth round-trip should happen at all).
	body, _ := json.Marshal(map[string]string{"post_slug": slug, "instance": inst.domain(t), "draft_text": "second comment"})
	req := httptest.NewRequest(http.MethodPost, "/api/comments/oauth/start", bytes.NewReader(body))
	req.AddCookie(sessCookie)
	req.AddCookie(tokenCookie)
	rec := httptest.NewRecorder()
	h.StartCommentAuth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("start status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp startCommentAuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Posted {
		t.Fatalf("expected Posted=true, got %+v", resp)
	}
	if resp.RedirectURL != "" {
		t.Fatalf("expected no redirect on the fast path, got %q", resp.RedirectURL)
	}
	if inst.postReplyCalls != 2 {
		t.Fatalf("expected 2 post-reply calls total, got %d", inst.postReplyCalls)
	}
	if inst.registerCalls != registerCallsAfterFirst {
		t.Fatalf("fast path should not re-register the app: before=%d after=%d", registerCallsAfterFirst, inst.registerCalls)
	}
}

func TestCommentLogoutRevokesCachedToken(t *testing.T) {
	inst := newFakeInstance(t)
	client := mastodon.NewClientWithHTTP(inst.srv.Client())
	h, ob, _ := testCommentHandler(t, client)

	post, _, err := ob.CreatePost(outbox.KindNote, "hello world")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)

	cbRec := startAndFollowCallback(t, h, inst.domain(t), slug)
	sessCookie := cookieByName(t, cbRec.Result().Cookies(), commentauth.SessionCookie)
	tokenCookie := cookieByName(t, cbRec.Result().Cookies(), commentauth.TokenCookie)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/comments/logout", nil)
	logoutReq.AddCookie(sessCookie)
	logoutReq.AddCookie(tokenCookie)
	logoutRec := httptest.NewRecorder()
	h.CommentLogout(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on logout, got %d", logoutRec.Code)
	}
	if inst.revokeCalls != 1 {
		t.Fatalf("expected logout to revoke the cached token, revokeCalls = %d", inst.revokeCalls)
	}

	var clearedToken *http.Cookie
	for _, c := range logoutRec.Result().Cookies() {
		if c.Name == commentauth.TokenCookie {
			clearedToken = c
		}
	}
	if clearedToken == nil || clearedToken.MaxAge >= 0 {
		t.Fatalf("expected token cookie to be cleared on logout, got %+v", clearedToken)
	}
}

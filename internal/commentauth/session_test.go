package commentauth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/newtosh/nitpub/internal/store"
)

func testStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(st), func() { _ = st.Close() }
}

func TestSessionRoundTrip(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sess := CreateSessionRecord("sid", "mastodon.social", "alice", "Alice", "https://example.com/a.png", now)
	if err := s.PutSession(sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSession("sid")
	if err != nil {
		t.Fatal(err)
	}
	if got.Instance != "mastodon.social" || got.Handle != "alice" || got.DisplayName != "Alice" {
		t.Fatalf("unexpected session: %+v", got)
	}
}

func TestSessionTouchAndExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sess := CreateSessionRecord("abc", "i", "h", "", "", now)
	if sess.ExpiresAt != now.Add(sessionMaxAge) {
		t.Fatalf("unexpected expiry: %v", sess.ExpiresAt)
	}
	before := sess.ExpiresAt
	soon := now.Add(time.Hour)
	if !TouchSession(sess, soon) {
		t.Fatal("expected touch ok")
	}
	if sess.ExpiresAt != before {
		t.Fatal("expiry should not extend before sessionTouchAfter has elapsed")
	}
	later := now.Add(25 * time.Hour)
	if !TouchSession(sess, later) {
		t.Fatal("expected touch ok")
	}
	if sess.LastSeen != later {
		t.Fatal("last seen not updated")
	}
	if sess.ExpiresAt != before {
		t.Fatalf("expiry capped at created_at+30d should be unchanged: %v", sess.ExpiresAt)
	}
	expired := sess.CreatedAt.Add(sessionMaxAge + time.Second)
	if TouchSession(sess, expired) {
		t.Fatal("expected expired")
	}
}

func TestSessionValidate(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sess := CreateSessionRecord("sid", "i", "h", "", "", now)
	if err := s.PutSession(sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.ValidateSession("sid", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "sid" {
		t.Fatal("id mismatch")
	}
	_, err = s.ValidateSession("sid", now.Add(sessionMaxAge+time.Second))
	if err == nil {
		t.Fatal("expected expired")
	}
	if _, err := s.GetSession("sid"); err == nil {
		t.Fatal("expired session should be deleted")
	}
}

func TestSessionCookieHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://nitpub.example/p/x", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	SetSessionCookie(rec, req, "sid123", time.Now().Add(time.Hour))
	set := rec.Header().Get("Set-Cookie")
	if !strings.Contains(set, "nitpub_comment=sid123") {
		t.Fatalf("cookie value missing: %s", set)
	}
	if !strings.Contains(set, "HttpOnly") || !strings.Contains(set, "Secure") || !strings.Contains(set, "SameSite=Lax") {
		t.Fatalf("cookie flags missing: %s", set)
	}

	rec2 := httptest.NewRecorder()
	ClearSessionCookie(rec2, req)
	cleared := rec2.Header().Get("Set-Cookie")
	if !strings.Contains(cleared, "nitpub_comment=") || !strings.Contains(cleared, "Max-Age=0") {
		t.Fatalf("expected cookie clear: %s", cleared)
	}
}

func TestPendingCommentAuthExpiry(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p, err := NewPendingCommentAuth("my-post", "hello world", "mastodon.social", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutPending(p); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPending(p.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.PostSlug != "my-post" || got.DraftText != "hello world" {
		t.Fatalf("unexpected pending record: %+v", got)
	}
	if got.ExpiresAt != now.Add(pendingAuthTTL) {
		t.Fatalf("unexpected TTL: %v", got.ExpiresAt)
	}
}

func TestPendingCommentAuthCapsDraftText(t *testing.T) {
	now := time.Now()
	long := strings.Repeat("a", MaxDraftTextBytes+500)
	p, err := NewPendingCommentAuth("slug", long, "mastodon.social", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.DraftText) != MaxDraftTextBytes {
		t.Fatalf("expected draft text capped at %d bytes, got %d", MaxDraftTextBytes, len(p.DraftText))
	}
}

func TestPendingCommentAuthCapsDraftTextWithoutSplittingRune(t *testing.T) {
	now := time.Now()
	// A multi-byte rune ("é", 2 bytes) sitting exactly on the truncation
	// boundary — a raw byte-index slice would cut it in half.
	long := strings.Repeat("a", MaxDraftTextBytes-1) + "éé"
	p, err := NewPendingCommentAuth("slug", long, "mastodon.social", now)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(p.DraftText) {
		t.Fatalf("truncated draft text is not valid UTF-8: %q", p.DraftText)
	}
	if len(p.DraftText) > MaxDraftTextBytes {
		t.Fatalf("truncated draft text exceeds cap: %d bytes", len(p.DraftText))
	}
}

func TestBindingCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://nitpub.example/comment/callback?state=tok123", nil)
	rec := httptest.NewRecorder()
	SetBindingCookie(rec, req, "tok123", time.Now().Add(10*time.Minute))
	set := rec.Header().Get("Set-Cookie")
	if !strings.Contains(set, "nitpub_comment_binding=tok123") {
		t.Fatalf("binding cookie value missing: %s", set)
	}

	req2 := httptest.NewRequest(http.MethodGet, "https://nitpub.example/comment/callback", nil)
	req2.AddCookie(&http.Cookie{Name: BindingCookie, Value: "tok123"})
	if !VerifyBindingCookie(req2, "tok123") {
		t.Fatal("expected binding cookie to verify against matching state")
	}
	if VerifyBindingCookie(req2, "wrong") {
		t.Fatal("expected binding cookie to reject mismatched state")
	}

	req3 := httptest.NewRequest(http.MethodGet, "https://nitpub.example/comment/callback", nil)
	if VerifyBindingCookie(req3, "tok123") {
		t.Fatal("expected binding cookie to reject when cookie is absent")
	}
}

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionTouchAndExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	sess := CreateSessionRecord("abc", now, true)
	if sess.ExpiresAt != now.Add(sessionMaxAge) {
		t.Fatalf("unexpected expiry: %v", sess.ExpiresAt)
	}
	browser := CreateSessionRecord("abc", now, false)
	if browser.ExpiresAt != now.Add(sessionBrowserMaxAge) {
		t.Fatalf("browser expiry: %v", browser.ExpiresAt)
	}
	later := now.Add(25 * time.Hour)
	if !TouchSession(sess, later) {
		t.Fatal("expected touch ok")
	}
	if sess.LastSeen != later {
		t.Fatal("last seen not updated")
	}
	expired := sess.CreatedAt.Add(sessionMaxAge + time.Second)
	if TouchSession(sess, expired) {
		t.Fatal("expected expired")
	}
}

func TestSetSessionCookieDomain(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)

	rec := httptest.NewRecorder()
	SetSessionCookie(rec, req, "sid", now.Add(time.Hour), true, "")
	cookies := cookiesNamed(rec, SessionCookie)
	if len(cookies) != 1 {
		t.Fatalf("host-only set: got %d Set-Cookie lines, want 1", len(cookies))
	}
	if cookies[0].Domain != "" {
		t.Fatalf("Domain = %q, want empty when cookieDomain unset", cookies[0].Domain)
	}

	// Go's net/http normalizes away a leading dot when writing the
	// Set-Cookie header (RFC 6265 makes Domain=example.com and the
	// legacy Domain=.example.com equivalent) — the leading dot in the
	// input is cosmetic, not required.
	// Widening to a Domain cookie must also expire the host-only twin so
	// browsers don't keep both under the same name.
	rec = httptest.NewRecorder()
	SetSessionCookie(rec, req, "sid", now.Add(time.Hour), true, ".example.com")
	cookies = cookiesNamed(rec, SessionCookie)
	if len(cookies) != 2 {
		t.Fatalf("domain set: got %d Set-Cookie lines, want 2 (expire host-only + set domain)", len(cookies))
	}
	if cookies[0].MaxAge != -1 || cookies[0].Domain != "" {
		t.Fatalf("first cookie should expire host-only, got Domain=%q MaxAge=%d", cookies[0].Domain, cookies[0].MaxAge)
	}
	if cookies[1].Domain != "example.com" || cookies[1].Value != "sid" {
		t.Fatalf("second cookie = Domain:%q Value:%q, want Domain:example.com Value:sid", cookies[1].Domain, cookies[1].Value)
	}
}

func TestClearSessionCookieExpiresHostOnlyAndDomain(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rec := httptest.NewRecorder()
	ClearSessionCookie(rec, req, ".example.com")
	cookies := cookiesNamed(rec, SessionCookie)
	if len(cookies) != 2 {
		t.Fatalf("got %d Set-Cookie lines, want 2", len(cookies))
	}
	if cookies[0].Domain != "" || cookies[0].MaxAge != -1 {
		t.Fatalf("host-only clear = Domain:%q MaxAge:%d", cookies[0].Domain, cookies[0].MaxAge)
	}
	if cookies[1].Domain != "example.com" || cookies[1].MaxAge != -1 {
		t.Fatalf("domain clear = Domain:%q MaxAge:%d", cookies[1].Domain, cookies[1].MaxAge)
	}
}

func cookiesNamed(rec *httptest.ResponseRecorder, name string) []*http.Cookie {
	var out []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

func TestSessionValidate(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sess := CreateSessionRecord("sid", now, true)
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
}

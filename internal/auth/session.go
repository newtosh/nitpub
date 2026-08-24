package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

const (
	SessionCookie        = "nitpub_session"
	sessionMaxAge        = 30 * 24 * time.Hour
	sessionBrowserMaxAge = 24 * time.Hour
	sessionTouchAfter    = 24 * time.Hour
	pendingAuthTTL       = 5 * time.Minute
	enrollTokenTTL       = 10 * time.Minute
)

// NewSessionID returns a random opaque session identifier.
func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSessionRecord builds a new session. Persistent sessions use the 30-day cap.
func CreateSessionRecord(id string, now time.Time, persistent bool) *Session {
	expires := now.Add(sessionMaxAge)
	if !persistent {
		expires = now.Add(sessionBrowserMaxAge)
	}
	return &Session{
		ID:        id,
		CreatedAt: now,
		ExpiresAt: expires,
		LastSeen:  now,
	}
}

// TouchSession updates last_seen and may extend expiry up to created_at+30d.
func TouchSession(sess *Session, now time.Time) bool {
	if now.After(sess.ExpiresAt) {
		return false
	}
	cap := sess.CreatedAt.Add(sessionMaxAge)
	if now.Sub(sess.LastSeen) >= sessionTouchAfter && now.Before(cap) {
		next := now.Add(sessionMaxAge)
		if next.After(cap) {
			next = cap
		}
		sess.ExpiresAt = next
	}
	sess.LastSeen = now
	return true
}

func (s *Store) ValidateSession(id string, now time.Time) (*Session, error) {
	sess, err := s.GetSession(id)
	if err != nil {
		return nil, err
	}
	if now.After(sess.ExpiresAt) {
		_ = s.DeleteSession(id)
		return nil, fmt.Errorf("session expired")
	}
	if !TouchSession(sess, now) {
		_ = s.DeleteSession(id)
		return nil, fmt.Errorf("session expired")
	}
	if err := s.PutSession(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string, expires time.Time, persistent bool, cookieDomain string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	// When widening to a Domain cookie, expire any prior host-only cookie
	// with the same name first. Browsers can keep both; Request.Cookie then
	// returns an arbitrary one, and a stale host-only value makes a fresh
	// login look like it "succeeded then bounced back to /login".
	if cookieDomain != "" {
		expireSessionCookie(w, secure, "")
	}
	c := &http.Cookie{
		Name:     SessionCookie,
		Value:    sessionID,
		Domain:   cookieDomain, // empty by default: host-only cookie, unchanged behavior
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
	if persistent {
		c.Expires = expires
	}
	http.SetCookie(w, c)
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request, cookieDomain string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	// Always clear the host-only cookie. If a Domain is configured, clear
	// that variant too — otherwise logout (or a Domain→host-only config
	// change) leaves a cookie the browser still sends.
	expireSessionCookie(w, secure, "")
	if cookieDomain != "" {
		expireSessionCookie(w, secure, cookieDomain)
	}
}

func expireSessionCookie(w http.ResponseWriter, secure bool, cookieDomain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Domain:   cookieDomain,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func SessionIDFromRequest(r *http.Request) (string, error) {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return "", err
	}
	if c.Value == "" {
		return "", fmt.Errorf("empty session")
	}
	return c.Value, nil
}

func NewPendingAuth(now time.Time) (*PendingAuth, error) {
	id, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	return &PendingAuth{
		Token:     id,
		CreatedAt: now,
		ExpiresAt: now.Add(pendingAuthTTL),
	}, nil
}

func NewEnrollToken(now time.Time) (*EnrollToken, error) {
	id, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	return &EnrollToken{
		Token:     id,
		CreatedAt: now,
		ExpiresAt: now.Add(enrollTokenTTL),
	}, nil
}

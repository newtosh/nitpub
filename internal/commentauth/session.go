package commentauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"
)

const (
	// SessionCookie is deliberately distinct from auth.SessionCookie — a
	// different namespace for a different trust boundary (KTD1).
	SessionCookie = "nitpub_comment"
	// BindingCookie carries the CSRF state-binding value set at
	// StartCommentAuth and checked at CommentAuthCallback (KTD8).
	BindingCookie = "nitpub_comment_binding"
	// TokenCookie holds the visitor's live Mastodon OAuth access token,
	// scoped to the instance recorded in their comment session (SessionCookie).
	// Deliberately never persisted server-side: keeping this in an
	// HttpOnly/Secure/SameSite=Lax cookie instead of our own database means
	// a nitpub database compromise never exposes live third-party tokens
	// for however many visitors have commented recently — the tradeoff for
	// not revoking on every single post (KTD5's original posture) is that
	// the token now outlives one request, so it should live somewhere a
	// server-side breach can't reach at all, not just somewhere we trust.
	TokenCookie = "nitpub_comment_token"

	sessionMaxAge     = 30 * 24 * time.Hour
	sessionTouchAfter = 24 * time.Hour
	// tokenMaxAge caps how long a cached Mastodon token is reused before a
	// visitor has to redo the full OAuth consent round-trip again. 24h,
	// not sessionMaxAge (30d) — the identity cookie is low-stakes (just a
	// remembered display name), the token cookie can actually post as the
	// visitor, so it gets a shorter leash independent of how long they stay
	// "remembered".
	tokenMaxAge = 24 * time.Hour

	// pendingAuthTTL is its own constant, not shared with auth.pendingAuthTTL
	// (5 min) — a redirect to a third-party instance and back tends to take
	// longer than the internal enrollment flow that constant was tuned for
	// (KTD4).
	pendingAuthTTL = 10 * time.Minute

	// MaxDraftTextBytes bounds the visitor-authored comment text stored in
	// a pending-auth record (KTD4).
	MaxDraftTextBytes = 5000
)

// NewSessionID returns a random opaque identifier, shared by comment
// sessions, pending-auth tokens, and binding-cookie values.
func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSessionRecord builds a new comment session, always persistent
// (30-day sliding window per R6).
func CreateSessionRecord(id, instance, handle, displayName, avatarURL string, now time.Time) *CommentSession {
	return &CommentSession{
		ID:          id,
		Instance:    instance,
		Handle:      handle,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		CreatedAt:   now,
		ExpiresAt:   now.Add(sessionMaxAge),
		LastSeen:    now,
	}
}

// TouchSession updates last_seen and may extend expiry up to created_at+30d.
func TouchSession(sess *CommentSession, now time.Time) bool {
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

func (s *Store) ValidateSession(id string, now time.Time) (*CommentSession, error) {
	sess, err := s.GetSession(id)
	if err != nil {
		return nil, err
	}
	if now.After(sess.ExpiresAt) {
		_ = s.DeleteSession(id)
		return nil, fmt.Errorf("comment session expired")
	}
	if !TouchSession(sess, now) {
		_ = s.DeleteSession(id)
		return nil, fmt.Errorf("comment session expired")
	}
	if err := s.PutSession(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string, expires time.Time) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
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
		return "", fmt.Errorf("empty comment session")
	}
	return c.Value, nil
}

// SetTokenCookie caches the visitor's Mastodon access token client-side for
// up to 24h (tokenMaxAge), so posting a second comment doesn't require a
// full OAuth consent round-trip again.
func SetTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     TokenCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(tokenMaxAge.Seconds()),
	})
}

func ClearTokenCookie(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     TokenCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func TokenFromRequest(r *http.Request) (string, error) {
	c, err := r.Cookie(TokenCookie)
	if err != nil {
		return "", err
	}
	if c.Value == "" {
		return "", fmt.Errorf("empty comment token")
	}
	return c.Value, nil
}

// NewPendingCommentAuth builds a pending-auth record, capping DraftText at
// MaxDraftTextBytes (KTD4).
func NewPendingCommentAuth(postSlug, draftText, instance string, now time.Time) (*PendingCommentAuth, error) {
	token, err := NewSessionID()
	if err != nil {
		return nil, err
	}
	if len(draftText) > MaxDraftTextBytes {
		draftText = truncateUTF8(draftText, MaxDraftTextBytes)
	}
	return &PendingCommentAuth{
		Token:     token,
		PostSlug:  postSlug,
		DraftText: draftText,
		Instance:  instance,
		CreatedAt: now,
		ExpiresAt: now.Add(pendingAuthTTL),
	}, nil
}

// truncateUTF8 cuts s to at most maxBytes bytes without splitting a
// multi-byte rune: a byte-index slice can leave a trailing partial rune,
// which DecodeLastRuneInString reports as (RuneError, 1) — trim that
// single byte and retry until the tail is a complete rune (or empty).
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := s[:maxBytes]
	for len(b) > 0 {
		if r, size := utf8.DecodeLastRuneInString(b); r != utf8.RuneError || size != 1 {
			return b
		}
		b = b[:len(b)-1]
	}
	return b
}

// SetBindingCookie stores the pending-auth token in a short-lived,
// first-party cookie so CommentAuthCallback can verify the OAuth `state`
// param actually belongs to this browser (KTD8), not just that some
// pending-auth record with that token exists.
//
// Path is "/", not scoped to "/comment/callback" — narrower scoping was
// tried first but didn't reliably survive the real cross-site redirect
// chain (this site -> the visitor's instance -> back) in at least one
// real browser, even though curl round-trips it fine. HttpOnly and
// SameSite=Lax are the actual CSRF protection here; Path scoping was
// defense-in-depth only, not worth the fragility.
func SetBindingCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     BindingCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	})
}

func ClearBindingCookie(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     BindingCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// VerifyBindingCookie reports whether the request carries a binding cookie
// matching the given state token, in constant time.
func VerifyBindingCookie(r *http.Request, state string) bool {
	c, err := r.Cookie(BindingCookie)
	if err != nil || c.Value == "" || state == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) == 1
}

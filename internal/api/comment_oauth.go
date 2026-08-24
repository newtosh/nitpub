package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/newtosh/nitpub/internal/commentauth"
	"github.com/newtosh/nitpub/internal/mastodon"
	"github.com/newtosh/nitpub/internal/outbox"
)

// CommentHandler implements the Mastodon-powered comment flow:
// docs/plans/2026-08-21-002-feat-mastodon-powered-comments-plan.md.
type CommentHandler struct {
	outbox   *outbox.Service
	sessions *commentauth.Store
	apps     *mastodon.AppRegistrar
	client   *mastodon.Client
	baseURL  string
	// validateInstance defaults to mastodon.ValidateInstanceHost (KTD7).
	// Overridable in tests, which necessarily point at a loopback-hosted
	// fake instance that the real check would (correctly) reject.
	validateInstance func(string) error
}

func NewCommentHandler(ob *outbox.Service, sessions *commentauth.Store, apps *mastodon.AppRegistrar, client *mastodon.Client, baseURL string) *CommentHandler {
	return &CommentHandler{
		outbox:           ob,
		sessions:         sessions,
		apps:             apps,
		client:           client,
		baseURL:          baseURL,
		validateInstance: mastodon.ValidateInstanceHost,
	}
}

// instanceFromInput normalizes a visitor-typed instance field: strips a
// leading "@" and, for a full handle ("user@instance" or "@user@instance"),
// keeps only the domain after the last "@".
func instanceFromInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "@")
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	return s
}

func (h *CommentHandler) callbackURL() string {
	return h.baseURL + "/comment/callback"
}

func (h *CommentHandler) postURL(slug string) (string, error) {
	post, err := h.outbox.GetPublishedPost(slug)
	if err != nil {
		return "", err
	}
	return post.ID, nil
}

type startCommentAuthRequest struct {
	PostSlug string `json:"post_slug"`
	Instance string `json:"instance"`
	Draft    string `json:"draft_text"`
}

type startCommentAuthResponse struct {
	// RedirectURL is set when a full OAuth round-trip is required.
	RedirectURL string `json:"redirect_url,omitempty"`
	// Posted is set instead when a cached token (TokenCookie) let the
	// comment post immediately, with no redirect at all.
	Posted bool `json:"posted,omitempty"`
}

// resolveAndPost runs the resolve+post half of the flow shared by
// CommentAuthCallback (after a fresh OAuth exchange) and StartCommentAuth's
// cached-token fast path. draftText must be non-empty — callers check that
// before calling this.
func (h *CommentHandler) resolveAndPost(ctx context.Context, instance, token, postSlug, draftText string) error {
	postURL, err := h.postURL(postSlug)
	if err != nil {
		return err
	}
	statusID, err := h.client.ResolveStatus(ctx, instance, token, postURL)
	if err != nil {
		return err
	}
	return h.client.PostReply(ctx, instance, token, statusID, draftText)
}

// StartCommentAuth begins the OAuth flow (F1/F2 step 1). It is a JSON POST,
// not a bare GET (KTD7) — a cross-site page cannot trigger a
// Content-Type: application/json POST without a CORS preflight, and this
// server sends no Access-Control-Allow-Origin, so the browser blocks it
// before it ever reaches us.
func (h *CommentHandler) StartCommentAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req startCommentAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if _, err := h.outbox.GetPublishedPost(req.PostSlug); err != nil {
		http.Error(w, "post not found", http.StatusNotFound)
		return
	}
	// Accept either a bare domain or a full handle ("user@instance"),
	// matching Mastodon's own remote-follow prompt — take the part after
	// the last "@" when present, so a visitor who instinctively types
	// their handle isn't rejected.
	req.Instance = instanceFromInput(req.Instance)
	if err := h.validateInstance(req.Instance); err != nil {
		http.Error(w, "couldn't reach that instance", http.StatusBadRequest)
		return
	}

	// Fast path: a comment session and a still-cached token (TokenCookie,
	// up to 24h old) for the same instance means the visitor already has a
	// live grant — skip the whole register+redirect+consent round-trip and
	// post directly. Only applies once there's real text to post; the
	// identify-only first sign-in always needs the full flow since there's
	// no token to cache yet.
	if req.Draft != "" {
		if sessionID, err := commentauth.SessionIDFromRequest(r); err == nil {
			if sess, err := h.sessions.ValidateSession(sessionID, time.Now()); err == nil && sess.Instance == req.Instance {
				if token, err := commentauth.TokenFromRequest(r); err == nil {
					if err := h.resolveAndPost(r.Context(), req.Instance, token, req.PostSlug, req.Draft); err == nil {
						w.Header().Set("Content-Type", "application/json")
						_ = json.NewEncoder(w).Encode(startCommentAuthResponse{Posted: true})
						return
					}
					// Cached token no longer works (revoked on the visitor's
					// instance, expired there, etc.) — clear it and fall
					// through to a full re-auth rather than surfacing an
					// opaque failure for a case the visitor can't fix.
					log.Printf("comment-oauth: cached token no longer valid on %s, falling back to full auth", req.Instance)
					commentauth.ClearTokenCookie(w, r)
				}
			}
		}
	}

	reg, err := h.apps.RegisterOrGetApp(r.Context(), req.Instance, h.callbackURL())
	if err != nil {
		log.Printf("comment-oauth: register app on %s: %v", req.Instance, err)
		http.Error(w, "couldn't reach that instance", http.StatusBadGateway)
		return
	}

	now := time.Now()
	pending, err := commentauth.NewPendingCommentAuth(req.PostSlug, req.Draft, req.Instance, now)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.sessions.PutPending(pending); err != nil {
		log.Printf("comment-oauth: store pending auth: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	commentauth.SetBindingCookie(w, r, pending.Token, pending.ExpiresAt)

	authorizeURL := &url.URL{
		Scheme: "https",
		Host:   req.Instance,
		Path:   "/oauth/authorize",
	}
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {reg.ClientID},
		"redirect_uri":  {h.callbackURL()},
		"scope":         {reg.Scope},
		"state":         {pending.Token},
	}
	authorizeURL.RawQuery = q.Encode()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(startCommentAuthResponse{RedirectURL: authorizeURL.String()})
}

// CommentAuthCallback handles the redirect back from the visitor's
// instance: state/binding verification, token exchange, post resolution,
// posting the reply, and best-effort revocation (F1/F2 steps 2+, KTD5,
// KTD8).
func (h *CommentHandler) CommentAuthCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if !commentauth.VerifyBindingCookie(r, state) {
		c, cerr := r.Cookie(commentauth.BindingCookie)
		if cerr != nil {
			log.Printf("comment-oauth: binding cookie missing on callback (state=%s): %v", state, cerr)
		} else {
			log.Printf("comment-oauth: binding cookie present but mismatched (state=%s, cookie=%s)", state, c.Value)
		}
		h.redirectError(w, r, "", "error_auth", "")
		return
	}
	pending, err := h.sessions.GetPending(state)
	if err != nil {
		log.Printf("comment-oauth: pending auth lookup failed (state=%s): %v", state, err)
		h.redirectError(w, r, "", "error_auth", "")
		return
	}
	// One-time use: remove immediately so a replayed callback can't reuse it.
	if err := h.sessions.DeletePending(state); err != nil {
		log.Printf("comment-oauth: delete pending auth (one-time-use, replay window): %v", err)
	}
	commentauth.ClearBindingCookie(w, r)

	if time.Now().After(pending.ExpiresAt) {
		h.redirectError(w, r, pending.PostSlug, "error_expired", "")
		return
	}
	if code == "" {
		h.redirectError(w, r, pending.PostSlug, "error_auth", pending.DraftText)
		return
	}

	reg, err := h.apps.RegisterOrGetApp(r.Context(), pending.Instance, h.callbackURL())
	if err != nil {
		h.redirectError(w, r, pending.PostSlug, "error_instance", pending.DraftText)
		return
	}

	token, err := h.client.ExchangeToken(r.Context(), pending.Instance, reg.ClientID, reg.ClientSecret, code, h.callbackURL())
	if err != nil {
		log.Printf("comment-oauth: exchange token on %s: %v", pending.Instance, err)
		h.redirectError(w, r, pending.PostSlug, "error_auth", pending.DraftText)
		return
	}

	// The entering-instance phase's first sign-in round-trip identifies the
	// visitor with no comment text yet (CommentBox.vue's beginSignIn ->
	// submitInstance sends an empty draft — the compose box only exists
	// once signed in). Resolving the post and posting a reply is only
	// meaningful once there's actual text to post; without this guard,
	// every identify-only sign-in hit Mastodon with an empty status and
	// got rejected with "Text can't be blank".
	posted := false
	if strings.TrimSpace(pending.DraftText) != "" {
		postURL, err := h.postURL(pending.PostSlug)
		if err != nil {
			h.client.RevokeTokenBestEffort(r.Context(), pending.Instance, reg.ClientID, reg.ClientSecret, token)
			h.redirectError(w, r, pending.PostSlug, "error_auth", pending.DraftText)
			return
		}

		statusID, err := h.client.ResolveStatus(r.Context(), pending.Instance, token, postURL)
		if err != nil {
			if errors.Is(err, mastodon.ErrScopeRejected) {
				// Fix the cache for the next visitor from this domain; this
				// request's token still can't be retroactively widened (KTD3).
				if _, rerr := h.apps.Reregister(r.Context(), pending.Instance, h.callbackURL()); rerr != nil {
					log.Printf("comment-oauth: reregister %s with fallback scope: %v", pending.Instance, rerr)
				}
			}
			log.Printf("comment-oauth: resolve status on %s: %v", pending.Instance, err)
			h.client.RevokeTokenBestEffort(r.Context(), pending.Instance, reg.ClientID, reg.ClientSecret, token)
			h.redirectError(w, r, pending.PostSlug, "error_auth", pending.DraftText)
			return
		}

		if err := h.client.PostReply(r.Context(), pending.Instance, token, statusID, pending.DraftText); err != nil {
			log.Printf("comment-oauth: post reply on %s: %v", pending.Instance, err)
			h.client.RevokeTokenBestEffort(r.Context(), pending.Instance, reg.ClientID, reg.ClientSecret, token)
			h.redirectError(w, r, pending.PostSlug, "error_auth", pending.DraftText)
			return
		}
		posted = true
	}

	// Identify the account for "Commenting as @handle@instance" (R6) —
	// best-effort, before the token is revoked below.
	handle, displayName, avatarURL := "", "", ""
	if acct, err := h.client.VerifyCredentials(r.Context(), pending.Instance, token); err == nil {
		handle, displayName, avatarURL = acct.Username, acct.DisplayName, acct.Avatar
	} else {
		log.Printf("comment-oauth: verify credentials on %s: %v", pending.Instance, err)
	}

	// Cache the token client-side instead of revoking it here (former
	// KTD5 posture: revoke on every use, including this success path).
	// Every subsequent comment within tokenMaxAge (24h) reuses it via
	// StartCommentAuth's fast path instead of making the visitor redo
	// Mastodon's OAuth consent screen every time. Never written to our own
	// database — only this HttpOnly cookie holds it, so a nitpub DB
	// compromise can't expose it. The three error paths above still revoke
	// immediately, since those tokens have no cached future use.
	commentauth.SetTokenCookie(w, r, token)

	// Without a handle there's nothing meaningful to show as "Commenting
	// as @...@instance" — skip remembering this visitor rather than
	// showing a broken "@@instance" (R6). The comment itself still posted
	// above; only the remembered-identity convenience is degraded.
	if handle != "" {
		sessionID, err := commentauth.NewSessionID()
		if err == nil {
			sess := commentauth.CreateSessionRecord(sessionID, pending.Instance, handle, displayName, avatarURL, time.Now())
			if err := h.sessions.PutSession(sess); err == nil {
				commentauth.SetSessionCookie(w, r, sess.ID, sess.ExpiresAt)
			} else {
				log.Printf("comment-oauth: store comment session: %v", err)
			}
		} else {
			log.Printf("comment-oauth: generate comment session id: %v", err)
		}
	}

	// Identify-only sign-in (no draft text, nothing posted) lands signed in
	// with the compose box ready — not the "comment sent" banner, which
	// would be a lie about what just happened.
	status := "signed_in"
	if posted {
		status = "success"
	}
	http.Redirect(w, r, "/p/"+pending.PostSlug+"?comment="+status+"#replies", http.StatusFound)
}

type commentSessionResponse struct {
	Instance    string `json:"instance"`
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
}

// CommentSessionStatus reports whether the visitor is remembered (R6, U4).
func (h *CommentHandler) CommentSessionStatus(w http.ResponseWriter, r *http.Request) {
	id, err := commentauth.SessionIDFromRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sess, err := h.sessions.ValidateSession(id, time.Now())
	if err != nil {
		commentauth.ClearSessionCookie(w, r)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(commentSessionResponse{
		Instance:    sess.Instance,
		Handle:      sess.Handle,
		DisplayName: sess.DisplayName,
		AvatarURL:   sess.AvatarURL,
	})
}

// CommentLogout clears the visitor's remembered identity, and — now that a
// token can outlive a single comment (TokenCookie, up to 24h) — revokes it
// on the visitor's own instance too, best-effort, so logging out actually
// ends the live grant rather than leaving it valid until it naturally
// expires client-side.
func (h *CommentHandler) CommentLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if sessionID, err := commentauth.SessionIDFromRequest(r); err == nil {
		if sess, err := h.sessions.ValidateSession(sessionID, time.Now()); err == nil {
			if token, err := commentauth.TokenFromRequest(r); err == nil {
				if reg, err := h.apps.RegisterOrGetApp(r.Context(), sess.Instance, h.callbackURL()); err == nil {
					h.client.RevokeTokenBestEffort(r.Context(), sess.Instance, reg.ClientID, reg.ClientSecret, token)
				}
			}
		}
		_ = h.sessions.DeleteSession(sessionID)
	}
	commentauth.ClearSessionCookie(w, r)
	commentauth.ClearTokenCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *CommentHandler) redirectError(w http.ResponseWriter, r *http.Request, postSlug, status, draft string) {
	if postSlug == "" {
		// No pending record to anchor a post — nothing meaningful to
		// redirect back to.
		http.Error(w, "invalid or expired request", http.StatusBadRequest)
		return
	}
	q := url.Values{"comment": {status}}
	if draft != "" {
		q.Set("draft", draft)
	}
	http.Redirect(w, r, "/p/"+postSlug+"?"+q.Encode()+"#replies", http.StatusFound)
}

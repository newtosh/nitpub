package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/newtosh/nitpub/internal/mastodon"
	"github.com/newtosh/nitpub/internal/outbox"
)

// referencePendingTTL bounds how long a connect attempt's CSRF state stays
// valid — the admin redirects to the reference instance and back, same
// order-of-magnitude round-trip as the visitor comment-auth flow.
const referencePendingTTL = 10 * time.Minute

// referencePending is a tiny in-memory CSRF-state store for the connect
// flow: state token -> instance domain + expiry. Not persisted — a nitpub
// restart mid-connect just means the admin clicks Connect again, and this
// avoids a bucket/schema for state that's only ever needed for minutes.
var (
	referencePendingMu sync.Mutex
	referencePending   = map[string]referencePendingEntry{}
)

type referencePendingEntry struct {
	instance string
	expires  time.Time
}

func newReferenceState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func putReferencePending(state, instance string) {
	referencePendingMu.Lock()
	defer referencePendingMu.Unlock()
	referencePending[state] = referencePendingEntry{instance: instance, expires: time.Now().Add(referencePendingTTL)}
}

// takeReferencePending returns and removes the pending entry for state
// (one-time use, same reasoning as commentauth's pending-auth records).
func takeReferencePending(state string) (referencePendingEntry, bool) {
	referencePendingMu.Lock()
	defer referencePendingMu.Unlock()
	entry, ok := referencePending[state]
	delete(referencePending, state)
	if !ok || time.Now().After(entry.expires) {
		return referencePendingEntry{}, false
	}
	return entry, true
}

type referenceStatusResponse struct {
	Connected bool   `json:"connected"`
	Instance  string `json:"instance,omitempty"`
}

// AdminGetReferenceStatus reports whether a reference instance is connected.
func (h *Handler) AdminGetReferenceStatus(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp := referenceStatusResponse{}
	if h.referenceAuth != nil {
		if auth, err := h.referenceAuth.Get(); err == nil && auth != nil {
			resp.Connected = true
			resp.Instance = auth.Instance
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type startReferenceConnectResponse struct {
	RedirectURL string `json:"redirect_url"`
}

// AdminStartReferenceConnect begins the OAuth flow against the site's
// configured reference instance (Federation.ReferenceInstanceOrDefault).
func (h *Handler) AdminStartReferenceConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.referenceApps == nil || h.mastodonClient == nil {
		http.Error(w, "reference instance connect not available", http.StatusServiceUnavailable)
		return
	}
	manifest, err := h.site.Load()
	if err != nil {
		http.Error(w, "could not load site config", http.StatusInternalServerError)
		return
	}
	instance := manifest.Federation.ReferenceInstanceOrDefault()
	if err := h.referenceValidateInstance(instance); err != nil {
		http.Error(w, "couldn't reach that instance", http.StatusBadRequest)
		return
	}

	reg, err := h.referenceApps.RegisterOrGetApp(r.Context(), instance, h.referenceCallbackURL())
	if err != nil {
		log.Printf("reference-connect: register app on %s: %v", instance, err)
		http.Error(w, "couldn't reach that instance", http.StatusBadGateway)
		return
	}

	state, err := newReferenceState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	putReferencePending(state, instance)

	authorizeURL := &url.URL{Scheme: "https", Host: instance, Path: "/oauth/authorize"}
	q := url.Values{
		"response_type": {"code"},
		"client_id":     {reg.ClientID},
		"redirect_uri":  {h.referenceCallbackURL()},
		"scope":         {reg.Scope},
		"state":         {state},
	}
	authorizeURL.RawQuery = q.Encode()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(startReferenceConnectResponse{RedirectURL: authorizeURL.String()})
}

// AdminReferenceCallback handles the redirect back from the reference
// instance: state verification, token exchange, and storing the grant.
func (h *Handler) AdminReferenceCallback(w http.ResponseWriter, r *http.Request) {
	redirectAdmin := func(status string) {
		http.Redirect(w, r, "/admin/federation?reference="+status, http.StatusFound)
	}

	if h.referenceApps == nil || h.mastodonClient == nil || h.referenceAuth == nil {
		redirectAdmin("error")
		return
	}
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	entry, ok := takeReferencePending(state)
	if !ok {
		redirectAdmin("error")
		return
	}
	if code == "" {
		redirectAdmin("error")
		return
	}

	reg, err := h.referenceApps.RegisterOrGetApp(r.Context(), entry.instance, h.referenceCallbackURL())
	if err != nil {
		log.Printf("reference-connect: register app on %s: %v", entry.instance, err)
		redirectAdmin("error")
		return
	}
	token, err := h.mastodonClient.ExchangeToken(r.Context(), entry.instance, reg.ClientID, reg.ClientSecret, code, h.referenceCallbackURL())
	if err != nil {
		log.Printf("reference-connect: exchange token on %s: %v", entry.instance, err)
		redirectAdmin("error")
		return
	}
	if err := h.referenceAuth.Put(mastodon.ReferenceAuth{Instance: entry.instance, Token: token}); err != nil {
		log.Printf("reference-connect: store grant: %v", err)
		redirectAdmin("error")
		return
	}

	redirectAdmin("connected")
}

// AdminDisconnectReference revokes (best-effort) and clears the current grant.
func (h *Handler) AdminDisconnectReference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.referenceAuth == nil {
		http.Error(w, "reference instance connect not available", http.StatusServiceUnavailable)
		return
	}
	if err := h.referenceAuth.Delete(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type resolvePermalinksResult struct {
	Resolved int `json:"resolved"`
	Skipped  int `json:"skipped"`
}

// AdminResolveReferencePermalinks resolves remote permalinks for every
// already-shared post that doesn't have one yet — covers posts shared
// before a reference instance was connected (new shares resolve on their
// own; see FederationPublisher.Complete).
func (h *Handler) AdminResolveReferencePermalinks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	posts, err := h.outbox.ListPosts()
	if err != nil {
		http.Error(w, "could not list posts", http.StatusInternalServerError)
		return
	}
	publisher := h.federationPublisher()
	result := resolvePermalinksResult{}
	for _, post := range posts {
		if post.Federation == nil || !post.Federation.Shared || post.Federation.RemoteURL != "" {
			continue
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		err := publisher.ResolveRemoteURLNow(ctx, outbox.PostSlug(post.ID), post.ID)
		cancel()
		if err != nil {
			log.Printf("reference-resolve: %s: %v", post.ID, err)
			result.Skipped++
			continue
		}
		result.Resolved++
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

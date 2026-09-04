package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	vocab "github.com/go-ap/activitypub"

	"github.com/newtosh/nitpub/internal/bluesky"
	"github.com/newtosh/nitpub/internal/mastodon"
	"github.com/newtosh/nitpub/internal/outbox"
)

// FederationPublisher records delivery state and fans out ActivityPub activities.
type FederationPublisher struct {
	Outbox  *outbox.Service
	Deliver func(activity any) error
	// Reference and MastodonClient are optional — when both are set and an
	// admin has connected a reference instance, a successful share kicks
	// off a best-effort background resolve of the post's permalink there
	// (see internal/sitecontent's ReferenceInstance doc comment for what
	// that permalink does and doesn't mean).
	Reference      *mastodon.ReferenceAuthStore
	MastodonClient *mastodon.Client
	// BlueskyClient and BlueskyAuth back the optional Bluesky crosspost
	// trigger (U5) — nil means Bluesky isn't wired up at all, and
	// CompleteBluesky is a silent no-op. A non-nil pair with no stored
	// Auth (admin never connected) is likewise a silent no-op — see
	// CompleteBluesky.
	BlueskyClient *bluesky.Client
	BlueskyAuth   *bluesky.AuthStore
}

func (p FederationPublisher) Complete(post *outbox.Post, create *vocab.Create, federate bool) (*outbox.Post, error) {
	slug := outbox.PostSlug(post.ID)
	if !federate {
		return p.Outbox.SetFederation(slug, outbox.FederationState{Shared: false})
	}

	fed, err := outbox.FederatedActivity(post, create)
	if err != nil {
		updated, _ := p.Outbox.SetFederation(slug, outbox.FederationState{
			Shared: true,
			Error:  err.Error(),
		})
		if updated != nil {
			return updated, fmt.Errorf("federation error: %w", err)
		}
		return post, fmt.Errorf("federation error: %w", err)
	}

	if p.Deliver != nil {
		if err := p.Deliver(fed); err != nil {
			updated, _ := p.Outbox.SetFederation(slug, outbox.FederationState{
				Shared: true,
				Error:  err.Error(),
			})
			if updated != nil {
				return updated, fmt.Errorf("delivery failed: %w", err)
			}
			return post, fmt.Errorf("delivery failed: %w", err)
		}
	}

	now := time.Now().UTC()
	updated, err := p.Outbox.SetFederation(slug, outbox.FederationState{
		Shared:   true,
		SharedAt: &now,
	})
	if err == nil {
		p.resolveRemoteURLAsync(slug, post.ID)
	}
	return updated, err
}

// resolveRemoteURLAsync best-effort resolves the post's permalink on the
// connected reference instance and stores it. Backgrounded because the
// resolving instance may not have received/indexed the object yet — this
// makes a real network call to a third party and isn't allowed to block
// the response to whoever just published the post.
func (p FederationPublisher) resolveRemoteURLAsync(slug, postURL string) {
	if p.Reference == nil || p.MastodonClient == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := p.ResolveRemoteURLNow(ctx, slug, postURL); err != nil {
			log.Printf("federation: resolve remote permalink: %v", err)
		}
	}()
}

// ResolveRemoteURLNow resolves and stores postURL's permalink on the
// connected reference instance, synchronously. Returns an error (rather
// than swallowing it) so a caller iterating many posts — see
// AdminResolveReferencePermalinks — can report how many actually resolved.
func (p FederationPublisher) ResolveRemoteURLNow(ctx context.Context, slug, postURL string) error {
	if p.Reference == nil || p.MastodonClient == nil {
		return fmt.Errorf("reference instance not configured")
	}
	auth, err := p.Reference.Get()
	if err != nil {
		return err
	}
	if auth == nil {
		return fmt.Errorf("no reference instance connected")
	}
	remoteURL, err := p.MastodonClient.ResolvePermalink(ctx, auth.Instance, auth.Token, postURL)
	if err != nil {
		return err
	}
	return p.Outbox.SetFederationRemoteURL(slug, remoteURL)
}

// CompleteBluesky starts a Bluesky crosspost for post when wantBluesky is
// true and an admin has actually connected a Bluesky account (KTD2/R3).
// It's a silent no-op — no error, no state written — when Bluesky isn't
// wired up (BlueskyClient/BlueskyAuth nil) or isn't connected, since the
// admin simply hasn't set it up.
//
// The pending state is written synchronously, before delivery starts, so
// it's observable immediately after the publish response returns; delivery
// itself runs in a background goroutine (bounded context, logged on
// failure) mirroring resolveRemoteURLAsync — it must not block the
// publish response (R3).
// Returns the post as stored after the pending write (so a caller building
// an HTTP response can reflect it immediately), or post unchanged when
// nothing was started.
func (p FederationPublisher) CompleteBluesky(post *outbox.Post, wantBluesky bool) *outbox.Post {
	if !wantBluesky || p.BlueskyClient == nil || p.BlueskyAuth == nil {
		return post
	}
	auth, err := p.BlueskyAuth.Get()
	if err != nil || auth == nil {
		return post
	}

	slug := outbox.PostSlug(post.ID)
	now := time.Now().UTC()
	updated, err := p.Outbox.SetBluesky(slug, outbox.BlueskyState{Status: "pending", PendingSince: &now})
	if err != nil {
		log.Printf("bluesky: set pending state: %v", err)
		return post
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := bluesky.Deliver(ctx, p.BlueskyClient, p.BlueskyAuth, p.Outbox, post); err != nil {
			log.Printf("bluesky: deliver: %v", err)
		}
	}()
	return updated
}

func (h *Handler) crossPostDefault() bool {
	if h.site == nil {
		return true
	}
	m, err := h.site.Load()
	if err != nil {
		return true
	}
	return m.Federation.Enabled()
}

func (h *Handler) resolveFederate(explicit *bool) bool {
	if explicit != nil {
		return *explicit
	}
	return h.crossPostDefault()
}

func (h *Handler) federationPublisher() FederationPublisher {
	return FederationPublisher{
		Outbox:         h.outbox,
		Deliver:        h.deliver,
		Reference:      h.referenceAuth,
		MastodonClient: h.mastodonClient,
		BlueskyClient:  h.blueskyClient,
		BlueskyAuth:    h.blueskyAuth,
	}
}

func (h *Handler) completeFederation(post *outbox.Post, create *vocab.Create, federate bool) (*outbox.Post, error) {
	return h.federationPublisher().Complete(post, create, federate)
}

// completeBluesky starts (or silently skips) the async Bluesky crosspost
// for post — see FederationPublisher.CompleteBluesky.
func (h *Handler) completeBluesky(post *outbox.Post, wantBluesky bool) *outbox.Post {
	return h.federationPublisher().CompleteBluesky(post, wantBluesky)
}

// AdminRetryBlueskyPost handles POST /api/posts/{slug}/bluesky/retry — a
// foreground, admin-triggered re-attempt of Bluesky delivery for a post
// that already has a prior attempt recorded (post.Bluesky != nil). Unlike
// the publish-time trigger, this runs bluesky.Deliver synchronously: the
// admin explicitly asked for a retry and waits for the result.
//
// A "pending" state younger than the staleness bound (see
// outbox.BlueskyState.StalePending) is treated as still genuinely in
// flight and refused with 409 rather than raced by a concurrent Deliver
// call; an older "pending" (stuck by e.g. a process restart mid-delivery,
// KTD5) is retried like any other non-success state.
func (h *Handler) AdminRetryBlueskyPost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.blueskyClient == nil || h.blueskyAuth == nil {
		http.Error(w, "bluesky not available", http.StatusServiceUnavailable)
		return
	}

	slug := r.PathValue("slug")
	post, err := h.outbox.GetPost(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if post.Bluesky == nil {
		http.Error(w, "post has no prior bluesky attempt to retry", http.StatusBadRequest)
		return
	}
	if post.Bluesky.Status == "pending" && !post.Bluesky.StalePending() {
		http.Error(w, "bluesky delivery already in progress", http.StatusConflict)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	deliverErr := bluesky.Deliver(ctx, h.blueskyClient, h.blueskyAuth, h.outbox, post)

	updated, getErr := h.outbox.GetPost(slug)
	if getErr != nil {
		http.Error(w, getErr.Error(), http.StatusInternalServerError)
		return
	}
	if deliverErr != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(updated)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updated)
}

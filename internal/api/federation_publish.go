package api

import (
	"context"
	"fmt"
	"log"
	"time"

	vocab "github.com/go-ap/activitypub"

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
	}
}

func (h *Handler) completeFederation(post *outbox.Post, create *vocab.Create, federate bool) (*outbox.Post, error) {
	return h.federationPublisher().Complete(post, create, federate)
}

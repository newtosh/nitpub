package api

import (
	"fmt"
	"time"

	vocab "github.com/go-ap/activitypub"

	"github.com/newtosh/nitpub/internal/outbox"
)

// FederationPublisher records delivery state and fans out ActivityPub activities.
type FederationPublisher struct {
	Outbox  *outbox.Service
	Deliver func(activity any) error
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
	return p.Outbox.SetFederation(slug, outbox.FederationState{
		Shared:   true,
		SharedAt: &now,
	})
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
	return FederationPublisher{Outbox: h.outbox, Deliver: h.deliver}
}

func (h *Handler) completeFederation(post *outbox.Post, create *vocab.Create, federate bool) (*outbox.Post, error) {
	return h.federationPublisher().Complete(post, create, federate)
}

package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/outbox"
)

type federationInfoResponse struct {
	Actor         string `json:"actor"`
	Domain        string `json:"domain"`
	Acct          string `json:"acct"`
	ActorURL      string `json:"actor_url"`
	FollowerCount int    `json:"follower_count"`
	FollowPolicy  string `json:"follow_policy"`
	FollowersOpen bool   `json:"followers_open"`
}

// AdminGetFederation returns read-only ActivityPub identity info for the admin UI.
func (h *Handler) AdminGetFederation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.federationDomain == "" {
		http.Error(w, "federation not configured", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	followers := 0
	if h.followerCount != nil {
		followers = h.followerCount()
	}
	_ = json.NewEncoder(w).Encode(federationInfoResponse{
		Actor:         h.federationActor,
		Domain:        h.federationDomain,
		Acct:          apstore.FormatAcct(h.federationActor, h.federationDomain),
		ActorURL:      h.federationBaseURL + "/actor",
		FollowerCount: followers,
		FollowPolicy:  "open",
		FollowersOpen: true,
	})
}

// AdminResendAccepts re-delivers Accept activities to all followers.
func (h *Handler) AdminResendAccepts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.resendAccepts == nil {
		http.Error(w, "federation not configured", http.StatusInternalServerError)
		return
	}
	sent, err := h.resendAccepts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sent": sent})
}

// AdminBackfillFederation delivers never-shared posts to followers using stable activity IDs.
func (h *Handler) AdminBackfillFederation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.backfillFederation == nil {
		http.Error(w, "federation not configured", http.StatusInternalServerError)
		return
	}
	result, err := h.backfillFederation()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// AdminRedeliverShared re-sends already-federated posts (idempotent on remote servers).
func (h *Handler) AdminRedeliverShared(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.redeliverShared == nil {
		http.Error(w, "federation not configured", http.StatusInternalServerError)
		return
	}
	result, err := h.redeliverShared()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

type federationDeliveryRow struct {
	Slug      string  `json:"slug"`
	Kind      string  `json:"kind"`
	CreatedAt string  `json:"created_at"`
	Status    string  `json:"status"`
	Error     string  `json:"error,omitempty"`
	SharedAt  *string `json:"shared_at,omitempty"`
}

// AdminFederationDeliveries returns each post's current federation delivery
// status for the admin delivery log. It is a live snapshot derived from
// Post.Federation, not a persisted delivery history.
func (h *Handler) AdminFederationDeliveries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit, hasLimit := parseIntQuery(r, "limit")
	offset, hasOffset := parseIntQuery(r, "offset")
	if !hasLimit || limit <= 0 {
		limit = 50
	}
	if !hasOffset || offset < 0 {
		offset = 0
	}
	// Deliberately published-only (not ListPostsForAuthorPaginated): drafts
	// have no federation activity, so they have nothing to show in a
	// delivery log.
	posts, total, err := h.outbox.ListPostsPaginated(limit, offset)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	rows := make([]federationDeliveryRow, 0, len(posts))
	for _, post := range posts {
		row := federationDeliveryRow{
			Slug:      outbox.PostSlug(post.ID),
			Kind:      string(post.Kind),
			CreatedAt: post.CreatedAt.Format(time.RFC3339),
			Status:    "pending",
		}
		if post.Federation != nil {
			if post.Federation.Error != "" {
				row.Status = "error"
				row.Error = post.Federation.Error
			} else if outbox.FederationDelivered(post) {
				row.Status = "delivered"
			}
			if post.Federation.SharedAt != nil {
				s := post.Federation.SharedAt.Format(time.RFC3339)
				row.SharedAt = &s
			}
		}
		rows = append(rows, row)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"deliveries": rows,
		"total":      total,
	})
}

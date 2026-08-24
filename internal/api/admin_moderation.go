package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/newtosh/nitpub/internal/moderation"
)

// AdminListPendingReplies returns every reply currently awaiting moderation.
func (h *Handler) AdminListPendingReplies(w http.ResponseWriter, r *http.Request) {
	h.listReplies(w, r, h.moderation.PendingReplies)
}

// AdminListReviewedReplies returns every already-actioned reply (approved,
// rejected, or skipped) — the admin queue's "Reviewed" view, from which a
// past decision can be reverted back to pending.
func (h *Handler) AdminListReviewedReplies(w http.ResponseWriter, r *http.Request) {
	h.listReplies(w, r, h.moderation.ReviewedReplies)
}

func (h *Handler) listReplies(w http.ResponseWriter, r *http.Request, list func() ([]moderation.Reply, error)) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.moderation == nil {
		http.Error(w, "moderation not configured", http.StatusInternalServerError)
		return
	}
	replies, err := list()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(replies)
}

// AdminApproveReply moves a reply to approved, making it publicly visible.
func (h *Handler) AdminApproveReply(w http.ResponseWriter, r *http.Request) {
	h.setReplyStatus(w, r, moderation.StatusApproved)
}

// AdminRejectReply moves a reply to rejected.
func (h *Handler) AdminRejectReply(w http.ResponseWriter, r *http.Request) {
	h.setReplyStatus(w, r, moderation.StatusRejected)
}

// AdminSkipReply moves a reply to skipped — set aside without publishing,
// without blocking the sending actor.
func (h *Handler) AdminSkipReply(w http.ResponseWriter, r *http.Request) {
	h.setReplyStatus(w, r, moderation.StatusSkipped)
}

// AdminRevertReply moves an already-actioned reply (approved, rejected, or
// skipped) back to pending, for re-review.
func (h *Handler) AdminRevertReply(w http.ResponseWriter, r *http.Request) {
	h.setReplyStatus(w, r, moderation.StatusPending)
}

func (h *Handler) setReplyStatus(w http.ResponseWriter, r *http.Request, status moderation.Status) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.moderation == nil {
		http.Error(w, "moderation not configured", http.StatusInternalServerError)
		return
	}
	key := r.PathValue("id")
	if key == "" {
		http.NotFound(w, r)
		return
	}
	if err := h.moderation.SetReplyStatus(key, status); err != nil {
		switch {
		case errors.Is(err, moderation.ErrReplyNotFound):
			http.NotFound(w, r)
		case errors.Is(err, moderation.ErrInvalidStatusTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "storage error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"status": string(status)})
}

type actorRequest struct {
	Actor string `json:"actor"`
}

// AdminListTrustedActors returns the trusted (allow-list) actor URIs.
func (h *Handler) AdminListTrustedActors(w http.ResponseWriter, r *http.Request) {
	h.listActors(w, r, h.moderation.ListTrusted)
}

// AdminAddTrustedActor adds an actor URI to the trusted list.
func (h *Handler) AdminAddTrustedActor(w http.ResponseWriter, r *http.Request) {
	h.addActor(w, r, h.moderation.AddTrusted)
}

// AdminRemoveTrustedActor removes an actor URI from the trusted list.
func (h *Handler) AdminRemoveTrustedActor(w http.ResponseWriter, r *http.Request) {
	h.removeActor(w, r, h.moderation.RemoveTrusted)
}

// AdminListBlockedActors returns the blocked actor URIs.
func (h *Handler) AdminListBlockedActors(w http.ResponseWriter, r *http.Request) {
	h.listActors(w, r, h.moderation.ListBlocked)
}

// AdminAddBlockedActor adds an actor URI to the blocked list.
func (h *Handler) AdminAddBlockedActor(w http.ResponseWriter, r *http.Request) {
	h.addActor(w, r, h.moderation.AddBlocked)
}

// AdminRemoveBlockedActor removes an actor URI from the blocked list.
// Blocking (or unblocking) an actor never retroactively changes the status
// of replies already stored — it only affects future ingestion decisions.
func (h *Handler) AdminRemoveBlockedActor(w http.ResponseWriter, r *http.Request) {
	h.removeActor(w, r, h.moderation.RemoveBlocked)
}

func (h *Handler) listActors(w http.ResponseWriter, r *http.Request, list func() ([]string, error)) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.moderation == nil {
		http.Error(w, "moderation not configured", http.StatusInternalServerError)
		return
	}
	actors, err := list()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(actors)
}

func (h *Handler) addActor(w http.ResponseWriter, r *http.Request, add func(actor string) error) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.moderation == nil {
		http.Error(w, "moderation not configured", http.StatusInternalServerError)
		return
	}
	var req actorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Actor == "" {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := add(req.Actor); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"actor": req.Actor})
}

func (h *Handler) removeActor(w http.ResponseWriter, r *http.Request, remove func(actor string) error) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.moderation == nil {
		http.Error(w, "moderation not configured", http.StatusInternalServerError)
		return
	}
	actor := r.PathValue("actor")
	if actor == "" {
		http.NotFound(w, r)
		return
	}
	if err := remove(actor); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"actor": actor})
}

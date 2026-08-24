package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// publicReply is the public-facing shape of an approved reply — deliberately
// omits internal moderation status and the composite storage key (R9, U4).
type publicReply struct {
	Actor      string `json:"actor"`
	AuthorName string `json:"author_name,omitempty"`
	Content    string `json:"content"`
	URL        string `json:"url,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
	// ObjectID and ParentObjectID let the client reconstruct nested-reply
	// threading (a reply to a reply): ParentObjectID matches another reply's
	// ObjectID within the same response for a nested reply, or matches
	// nothing in the list for a top-level reply (its target was the post
	// itself, not another reply) — the client treats an unmatched
	// ParentObjectID as a root.
	ObjectID       string `json:"object_id,omitempty"`
	ParentObjectID string `json:"parent_object_id,omitempty"`
	ReceivedAt     string `json:"received_at,omitempty"`
}

// GetPostReplies returns a post's approved replies, unauthenticated. A post
// with zero replies returns 200 with an empty array, not 404 — zero replies
// is a normal state, not an error (R8, R9, KTD4).
func (h *Handler) GetPostReplies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	slug := r.PathValue("id")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	if h.moderation == nil {
		http.Error(w, "moderation not configured", http.StatusInternalServerError)
		return
	}
	replies, err := h.moderation.ApprovedRepliesForPost(slug)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	out := make([]publicReply, 0, len(replies))
	for _, rp := range replies {
		out = append(out, publicReply{
			Actor:          rp.Actor,
			AuthorName:     rp.AuthorName,
			Content:        rp.Content,
			URL:            rp.URL,
			AvatarURL:      rp.AvatarURL,
			ObjectID:       rp.ObjectID,
			ParentObjectID: rp.InReplyTo,
			ReceivedAt:     rp.ReceivedAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

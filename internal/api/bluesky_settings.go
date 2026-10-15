package api

import (
	"encoding/json"
	"net/http"

	"github.com/newtosh/nitpub/internal/bluesky"
)

type blueskyStatusResponse struct {
	Connected      bool   `json:"connected"`
	Handle         string `json:"handle,omitempty"`
	NeedsReconnect bool   `json:"needs_reconnect"`
}

// AdminGetBlueskyStatus reports whether a Bluesky account is connected.
func (h *Handler) AdminGetBlueskyStatus(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp := blueskyStatusResponse{}
	if h.blueskyAuth != nil {
		if auth, err := h.blueskyAuth.Get(); err == nil && auth != nil {
			resp.Connected = true
			resp.Handle = auth.Handle
			resp.NeedsReconnect = auth.Invalid
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type connectBlueskyRequest struct {
	Handle      string `json:"handle"`
	AppPassword string `json:"app_password"`
}

// AdminConnectBluesky validates the given handle/app-password against
// Bluesky (via CreateSession) and, on success, persists the account.
// Nothing is stored on failure, and app_password is never echoed back or
// logged.
func (h *Handler) AdminConnectBluesky(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.blueskyClient == nil || h.blueskyAuth == nil {
		http.Error(w, "bluesky connect not available", http.StatusServiceUnavailable)
		return
	}
	var req connectBlueskyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Handle == "" || req.AppPassword == "" {
		http.Error(w, "handle and app_password are required", http.StatusBadRequest)
		return
	}

	session, err := h.blueskyClient.CreateSession(r.Context(), req.Handle, req.AppPassword)
	if err != nil {
		http.Error(w, "could not authenticate with bluesky", http.StatusBadRequest)
		return
	}

	if err := h.blueskyAuth.Put(bluesky.Auth{
		DID:        session.DID,
		Handle:     session.Handle,
		RefreshJWT: session.RefreshJWT,
	}); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(blueskyStatusResponse{Connected: true, Handle: session.Handle})
}

// AdminDisconnectBluesky clears the stored Bluesky account.
func (h *Handler) AdminDisconnectBluesky(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.blueskyAuth == nil {
		http.Error(w, "bluesky connect not available", http.StatusServiceUnavailable)
		return
	}
	if err := h.blueskyAuth.Delete(); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

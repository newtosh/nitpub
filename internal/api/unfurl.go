package api

import (
	"encoding/json"
	"net/http"

	"github.com/newtosh/nitpub/internal/linkpreview"
)

func (h *Handler) Unfurl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw := r.URL.Query().Get("url")
	if raw == "" {
		http.Error(w, "url required", http.StatusBadRequest)
		return
	}
	preview, err := linkpreview.Fetch(raw)
	if err != nil {
		http.Error(w, "preview unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_ = json.NewEncoder(w).Encode(preview)
}

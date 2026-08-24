package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ServeIcon serves a Phosphor icon's SVG by name (e.g. "heart" or
// "heart-fill"), fetching and caching it on first request — see
// internal/icons for the ":name:" markdown shortcode this backs.
func (h *Handler) ServeIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.icons == nil {
		http.NotFound(w, r)
		return
	}
	name := strings.TrimSuffix(r.PathValue("name"), ".svg")
	data, err := h.icons.Get(r.Context(), name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

// ServeIconCatalog returns the full searchable icon list ({name, tags}[])
// backing the compose editor's shortcode autocomplete. Author-only — it's
// an authoring aid, not something a visitor's browser needs.
func (h *Handler) ServeIconCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.icons == nil {
		http.NotFound(w, r)
		return
	}
	entries, err := h.icons.Catalog(r.Context())
	if err != nil {
		http.Error(w, "icon catalog unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_ = json.NewEncoder(w).Encode(entries)
}

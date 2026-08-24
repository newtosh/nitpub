package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (h *Handler) ServeSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.search == nil {
		http.Error(w, "search not configured", http.StatusInternalServerError)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}
	results := h.search.Search(q, 50)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
}

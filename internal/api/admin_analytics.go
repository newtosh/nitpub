package api

import (
	"encoding/json"
	"net/http"

	"github.com/newtosh/nitpub/internal/analytics"
)

// AdminGetAnalytics proxies GoatCounter pageview stats to the admin UI.
// Author-only, and 404s when analytics is disabled — see
// internal/analytics for the upstream fetch/cache and
// docs/plans/2026-08-23-001-feat-goatcounter-analytics-plan.md.
func (h *Handler) AdminGetAnalytics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.analytics == nil {
		http.NotFound(w, r)
		return
	}
	window := analytics.ParseWindow(r.URL.Query().Get("window"))
	stats, err := h.analytics.Stats(r.Context(), window)
	if err != nil {
		// err is already scrubbed of the Authorization header/token by
		// internal/analytics; still avoid echoing it verbatim to the
		// client, same as the rest of this handler set.
		http.Error(w, "analytics unavailable", http.StatusBadGateway)
		return
	}
	resp := struct {
		analytics.Stats
		GoatCounterURL string `json:"goatcounter_url"`
	}{Stats: stats, GoatCounterURL: h.analyticsPublicURL}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

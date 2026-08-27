package api

import (
	"encoding/json"
	"net/http"

	"github.com/newtosh/nitpub/internal/telemetry"
)

type adminTelemetryStatusResponse struct {
	Enabled bool `json:"enabled"`
}

// AdminGetTelemetryStatus reports whether telemetry is enabled.
func (h *Handler) AdminGetTelemetryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.telemetryStore == nil {
		http.NotFound(w, r)
		return
	}
	enabled, err := h.telemetryStore.TelemetryEnabled()
	if err != nil {
		http.Error(w, "telemetry status unavailable", http.StatusInternalServerError)
		return
	}
	resp := adminTelemetryStatusResponse{Enabled: enabled}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type adminTelemetrySetRequest struct {
	Enabled bool `json:"enabled"`
}

// AdminSetTelemetryEnabled toggles telemetry on or off. Turning it on for
// the first time (no identity persisted yet) registers the instance
// synchronously; registration failure leaves telemetry disabled rather
// than flipping the flag without a token (see internal/telemetry).
func (h *Handler) AdminSetTelemetryEnabled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.telemetryStore == nil {
		http.NotFound(w, r)
		return
	}

	var req adminTelemetrySetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Enabled {
		if h.telemetryRegisterURL == "" {
			http.Error(w, "telemetry is not configured on this instance", http.StatusConflict)
			return
		}
		if _, ok, err := h.telemetryStore.GetTelemetryIdentity(); err != nil {
			http.Error(w, "telemetry status unavailable", http.StatusInternalServerError)
			return
		} else if !ok {
			id, token, err := telemetry.Register(r.Context(), h.telemetryRegisterURL)
			if err != nil {
				http.Error(w, "registration failed: "+err.Error(), http.StatusBadGateway)
				return
			}
			if err := h.telemetryStore.SetTelemetryIdentity(id, token); err != nil {
				http.Error(w, "failed to persist telemetry identity", http.StatusInternalServerError)
				return
			}
		}
	}

	if err := h.telemetryStore.SetTelemetryEnabled(req.Enabled); err != nil {
		http.Error(w, "failed to update telemetry setting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(adminTelemetryStatusResponse(req))
}

package api

import (
	"encoding/json"
	"net/http"

	"github.com/newtosh/nitpub/internal/updatecheck"
	"github.com/newtosh/nitpub/internal/version"
)

type adminVersionResponse struct {
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	LatestURL       string `json:"latest_url,omitempty"`
	UpdateAvailable bool   `json:"update_available"`
	CheckError      string `json:"check_error,omitempty"`
}

// AdminCheckVersion reports the running version against the latest one
// published on GitHub. Read-only — it never triggers an update itself;
// see `nitpub update --apply` / deploy/update.sh for that.
func (h *Handler) AdminCheckVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp := adminVersionResponse{Current: version.Version}
	rel, err := updatecheck.Latest()
	if err != nil {
		resp.CheckError = "Could not reach GitHub to check for updates."
	} else {
		resp.Latest = rel.Tag
		resp.LatestURL = rel.URL
		resp.UpdateAvailable = rel.Tag != "" && rel.Tag != version.Version
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

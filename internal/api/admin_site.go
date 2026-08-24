package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/newtosh/nitpub/internal/sitecontent"
)

type adminSiteFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type adminSiteResponse struct {
	Manifest       sitecontent.Manifest `json:"manifest"`
	Files          []adminSiteFile      `json:"files"`
	ManifestExists bool                 `json:"manifest_exists"`
}

func (h *Handler) AdminGetSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.site == nil {
		http.Error(w, "site not configured", http.StatusInternalServerError)
		return
	}
	m, err := h.site.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, exists, err := h.site.ListPageFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// A nil slice serializes to JSON null, not []; the admin UI reads
	// data.files.length unconditionally, so a fresh instance with zero
	// configured pages would crash the Page files tab on load.
	files := []adminSiteFile{}
	for _, ref := range m.Pages {
		data, err := h.site.ReadFile(ref.File)
		if err != nil {
			files = append(files, adminSiteFile{Path: ref.File, Content: ""})
			continue
		}
		files = append(files, adminSiteFile{Path: ref.File, Content: string(data)})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(adminSiteResponse{
		Manifest:       m,
		Files:          files,
		ManifestExists: exists,
	})
}

func (h *Handler) AdminPutManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.site == nil {
		http.Error(w, "site not configured", http.StatusInternalServerError)
		return
	}
	var m sitecontent.Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := h.site.WriteManifest(m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.rebuildSearchIndex()
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AdminPutSiteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if h.site == nil {
		http.Error(w, "site not configured", http.StatusInternalServerError)
		return
	}
	rel := strings.TrimPrefix(r.PathValue("relPath"), "/")
	if rel == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if err := h.site.WriteFile(rel, body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.rebuildSearchIndex()
	w.WriteHeader(http.StatusNoContent)
}

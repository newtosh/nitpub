package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/newtosh/nitpub/internal/sitecontent"
	"github.com/newtosh/nitpub/internal/version"
)

func (h *Handler) ServeSite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
	nav := m.Nav
	if nav == nil {
		nav = []sitecontent.NavItem{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"title":      h.siteTitle,
		"branding":   m.Branding,
		"nav":        nav,
		"home":       m.Home,
		"archive":    m.Archive,
		"search":     m.Search,
		"federation": m.Federation,
		"content":    m.Content,
		// analytics_enabled is a deploy-time config.toml flag (internal/config),
		// not part of sitecontent.Manifest — it is not admin-editable, unlike
		// every other field in this response. Do not wire an edit form for it;
		// see the plan's System-Wide Impact note.
		"analytics_enabled": h.analyticsEnabled,
		"footer": map[string]any{
			"text":             m.Footer.Text,
			"github_url":       sitecontent.NitpubGithubURL,
			"show_github_link": m.Footer.GithubLinkEnabled(),
		},
		"version": version.Version,
	})
}

func (h *Handler) ServeSitePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.site == nil {
		http.Error(w, "site not configured", http.StatusInternalServerError)
		return
	}
	rel := strings.TrimPrefix(r.PathValue("path"), "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	urlPath := "/" + rel
	page, err := h.site.PageByPath(urlPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sitePageJSON(page, h.auth.Authenticated(r)))
}

func sitePageJSON(p *sitecontent.Page, authenticated bool) map[string]any {
	res := map[string]any{
		"type": p.Type,
		"path": p.Path,
	}
	if p.Title != "" {
		res["title"] = p.Title
	}
	// Only an authenticated admin needs the backing file path (deep-link
	// to Site > Page files) — no reason to expose internal file layout to
	// an anonymous visitor.
	if authenticated && p.File != "" {
		res["file"] = p.File
	}
	switch p.Type {
	case "markdown":
		res["body"] = p.Body
	case "links":
		res["links"] = p.Links
	}
	return res
}

package actor

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/config"
)

type jrdLink struct {
	Rel  string `json:"rel"`
	Type string `json:"type,omitempty"`
	Href string `json:"href"`
}

type jrd struct {
	Subject string    `json:"subject"`
	Links   []jrdLink `json:"links"`
}

// ServeWebFinger handles /.well-known/webfinger requests.
func (s *Service) ServeWebFinger(w http.ResponseWriter, r *http.Request) {
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	if !acctMatches(resource, s.cfg.Actor, s.cfg.Domain) {
		http.NotFound(w, r)
		return
	}

	doc := jrd{
		Subject: apstore.FormatAcct(s.cfg.Actor, s.cfg.Domain),
		Links: []jrdLink{{
			Rel:  "self",
			Type: "application/activity+json",
			Href: s.cfg.BaseURL + "/actor",
		}},
	}
	w.Header().Set("Content-Type", "application/jrd+json")
	_ = json.NewEncoder(w).Encode(doc)
}

// ServeHostMeta advertises the WebFinger endpoint (RFC 7033).
func (s *Service) ServeHostMeta(w http.ResponseWriter, _ *http.Request) {
	template := s.cfg.BaseURL + "/.well-known/webfinger?resource={uri}"
	body := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0">` +
		`<Link rel="lrdd" type="application/jrd+json" template="` + template + `"/>` +
		`</XRD>`
	w.Header().Set("Content-Type", "application/xrd+xml")
	_, _ = w.Write([]byte(body))
}

// acctMatches reports whether resource identifies the configured local actor.
// Matching is case-insensitive on username and domain per fediverse conventions.
func acctMatches(resource, username, domain string) bool {
	user, dom, ok := parseAcctResource(resource)
	if !ok {
		return false
	}
	return strings.EqualFold(user, username) && strings.EqualFold(dom, domain)
}

func parseAcctResource(resource string) (username, domain string, ok bool) {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return "", "", false
	}
	if strings.HasPrefix(resource, "acct:") {
		resource = strings.TrimPrefix(resource, "acct:")
	} else if strings.HasPrefix(resource, "@") {
		resource = strings.TrimPrefix(resource, "@")
	}
	parts := strings.SplitN(resource, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ParseAcct validates an acct: resource string.
func ParseAcct(resource, username, domain string) bool {
	return acctMatches(resource, username, domain)
}

// WebFingerURL returns the configured webfinger subject.
func WebFingerURL(cfg config.Config) string {
	return apstore.FormatAcct(cfg.Actor, cfg.Domain)
}

package server

import (
	"bytes"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/newtosh/nitpub/internal/auth"
)

// themeResolver returns the palette id for HTML injection.
type themeResolver func() (string, error)

// spaHandler serves embedded static assets and falls back to index.html for client routes.
// analyticsPublicURL, when non-empty, injects GoatCounter's count.js beacon into
// every SPA shell response (see injectGoatCounterBeacon).
func spaHandler(static fs.FS, resolver themeResolver, siteURL, actorURL, siteTitle, analyticsPublicURL string) http.Handler {
	assets := http.StripPrefix("/", http.FileServer(http.FS(static)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if strings.HasPrefix(name, "assets/") {
			if _, err := fs.Stat(static, name); err == nil {
				assets.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}
		serveIndex(w, r, static, resolver, siteURL, actorURL, siteTitle, analyticsPublicURL)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, static fs.FS, resolver themeResolver, siteURL, actorURL, siteTitle, analyticsPublicURL string) {
	data, err := fs.ReadFile(static, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	themeID := auth.DefaultThemeID
	if resolver != nil {
		if id, err := resolver(); err == nil {
			themeID = auth.NormalizeThemeID(id)
		}
	}
	data = injectFediverseVerification(data, siteURL, actorURL)
	data = injectThemeHTML(data, themeID)
	// The built index.html always ships with a hardcoded <title>nitpub</title>
	// — the SPA fixes that client-side after Vue mounts, but anything that
	// reads the page's title without running JS first (password managers
	// saving a login, social link previews, bookmarks) sees the wrong one
	// until then. Patch it into the actual response instead.
	if siteTitle != "" {
		data = injectTitle(data, siteTitle)
	}
	if analyticsPublicURL != "" {
		data = injectGoatCounterBeacon(data, analyticsPublicURL)
	}
	if r.Method == http.MethodHead {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(data))
}

// injectGoatCounterBeacon adds GoatCounter's self-hosted count.js snippet
// before </head>. Public URL is the stats subdomain (analytics_public_url),
// which must expose /count and /count.js without auth — see deploy/Caddyfile.
func injectGoatCounterBeacon(page []byte, publicURL string) []byte {
	base := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if base == "" {
		return page
	}
	if bytes.Contains(page, []byte(`data-goatcounter=`)) {
		return page
	}
	headClose := bytes.Index(bytes.ToLower(page), []byte("</head>"))
	if headClose < 0 {
		return page
	}
	countURL := html.EscapeString(base + "/count")
	scriptURL := html.EscapeString(base + "/count.js")
	snippet := fmt.Sprintf(`<script data-goatcounter="%s" async src="%s"></script>`, countURL, scriptURL)
	out := make([]byte, 0, len(page)+len(snippet))
	out = append(out, page[:headClose]...)
	out = append(out, snippet...)
	out = append(out, page[headClose:]...)
	return out
}

func injectFediverseVerification(page []byte, siteURL, actorURL string) []byte {
	var links []string
	if siteURL != "" && !containsRelMeHref(page, siteURL) {
		links = append(links, fmt.Sprintf(`<link rel="me" href="%s">`, html.EscapeString(siteURL)))
	}
	if actorURL != "" && actorURL != siteURL && !containsRelMeHref(page, actorURL) {
		links = append(links, fmt.Sprintf(`<link rel="me" href="%s">`, html.EscapeString(actorURL)))
	}
	if len(links) == 0 {
		return page
	}
	headClose := bytes.Index(bytes.ToLower(page), []byte("</head>"))
	if headClose < 0 {
		return page
	}
	injected := strings.Join(links, "")
	out := make([]byte, 0, len(page)+len(injected))
	out = append(out, page[:headClose]...)
	out = append(out, injected...)
	out = append(out, page[headClose:]...)
	return out
}

func containsRelMeHref(page []byte, target string) bool {
	return bytes.Contains(page, []byte(`rel="me"`)) && bytes.Contains(page, []byte(html.EscapeString(target)))
}

func injectTitle(page []byte, title string) []byte {
	open := bytes.Index(bytes.ToLower(page), []byte("<title>"))
	close := bytes.Index(bytes.ToLower(page), []byte("</title>"))
	if open < 0 || close < 0 || close < open {
		return page
	}
	out := make([]byte, 0, len(page)+len(title))
	out = append(out, page[:open+len("<title>")]...)
	out = append(out, []byte(html.EscapeString(title))...)
	out = append(out, page[close:]...)
	return out
}

func injectThemeHTML(html []byte, themeID string) []byte {
	return setHTMLDataAttr(html, "theme", themeID)
}

func setHTMLDataAttr(html []byte, name, value string) []byte {
	attr := fmt.Sprintf(`data-%s="`, name)
	if bytes.Contains(html, []byte(attr)) {
		start := bytes.Index(html, []byte(attr))
		if start >= 0 {
			valStart := start + len(attr)
			end := bytes.Index(html[valStart:], []byte(`"`))
			if end >= 0 {
				prefix := append([]byte{}, html[:valStart]...)
				suffix := html[valStart+end:]
				return append(append(prefix, []byte(value)...), suffix...)
			}
		}
	}
	open := bytes.Index(html, []byte("<html"))
	if open < 0 {
		return html
	}
	close := bytes.Index(html[open:], []byte(">"))
	if close < 0 {
		return html
	}
	insertAt := open + close
	insert := []byte(fmt.Sprintf(` data-%s="%s"`, name, value))
	out := make([]byte, 0, len(html)+len(insert))
	out = append(out, html[:insertAt]...)
	out = append(out, insert...)
	out = append(out, html[insertAt:]...)
	return out
}

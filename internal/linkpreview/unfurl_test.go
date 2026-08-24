package linkpreview

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseHTML(t *testing.T) {
	base, _ := url.Parse("https://www.theverge.com/article")
	html := `<!DOCTYPE html><html><head>
<title>Fallback title</title>
<meta property="og:title" content="OG Title" />
<meta property="og:description" content="A modular keyboard review." />
<meta property="og:image" content="https://cdn.example/hero.jpg" />
<meta property="og:site_name" content="The Verge" />
</head><body></body></html>`
	p := parseHTML(base, []byte(html))
	if p.Title != "OG Title" {
		t.Fatalf("title = %q", p.Title)
	}
	if p.Description != "A modular keyboard review." {
		t.Fatalf("description = %q", p.Description)
	}
	if p.Image != "https://cdn.example/hero.jpg" {
		t.Fatalf("image = %q", p.Image)
	}
	if p.SiteName != "The Verge" {
		t.Fatalf("site_name = %q", p.SiteName)
	}
}

func TestAssertSafeURLBlocksPrivate(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1/test")
	if err := assertSafeURL(u); err == nil {
		t.Fatal("expected private IP to be blocked")
	}
}

func TestAssertSafeURLAllowsHTTPS(t *testing.T) {
	u, _ := url.Parse("https://example.com/path")
	if err := assertSafeURL(u); err != nil {
		t.Fatalf("expected public https url to pass: %v", err)
	}
}

func TestResolveURLRelative(t *testing.T) {
	base, _ := url.Parse("https://example.com/posts/1")
	got := resolveURL(base, "/images/x.jpg")
	if got != "https://example.com/images/x.jpg" {
		t.Fatalf("resolveURL = %q", got)
	}
}

func TestParseHTMLDescriptionFallback(t *testing.T) {
	base, _ := url.Parse("https://example.com")
	html := `<html><head><meta name="description" content="Plain desc" /></head></html>`
	p := parseHTML(base, []byte(html))
	if !strings.Contains(p.Description, "Plain desc") {
		t.Fatalf("description = %q", p.Description)
	}
}

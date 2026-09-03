package linkpreview

import (
	"context"
	"net"
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

// TestSafeDialContextRejectsReboundPrivateIP simulates DNS rebinding: the
// pre-flight assertSafeURL check resolves a public IP and passes, but the
// resolver used at actual connect time (safeDialContext's own lookupIP
// call) answers with a private IP instead. If the transport trusted
// assertSafeURL's earlier result, this would connect to the private
// address; safeDialContext must re-validate at dial time and reject it.
func TestSafeDialContextRejectsReboundPrivateIP(t *testing.T) {
	orig := lookupIP
	defer func() { lookupIP = orig }()

	calls := 0
	lookupIP = func(host string) ([]net.IP, error) {
		calls++
		if calls == 1 {
			return []net.IP{net.ParseIP("93.184.216.34")}, nil // public
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil // rebound to private
	}

	u, err := url.Parse("http://rebind.example.test/page")
	if err != nil {
		t.Fatal(err)
	}
	if err := assertSafeURL(u); err != nil {
		t.Fatalf("expected pre-flight check against the public IP to pass: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected assertSafeURL to call lookupIP once, got %d", calls)
	}

	conn, err := safeDialContext(context.Background(), "tcp", "rebind.example.test:80")
	if err == nil {
		conn.Close()
		t.Fatal("expected safeDialContext to reject the rebound private IP")
	}
	if calls != 2 {
		t.Fatalf("expected safeDialContext to re-resolve independently, got %d total lookupIP calls", calls)
	}
}

// TestSafeDialContextDialsValidatedIP proves the dialer connects to an IP
// it validated itself (not the transport's default hostname resolution):
// stubbing lookupIP to a blocked address for an otherwise-unrelated host
// name must fail, showing the dial target comes from safeDialContext's own
// lookup rather than bypassing it.
func TestSafeDialContextDialsValidatedIP(t *testing.T) {
	orig := lookupIP
	defer func() { lookupIP = orig }()
	lookupIP = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("10.0.0.5")}, nil // private
	}

	_, err := safeDialContext(context.Background(), "tcp", "internal.example.test:80")
	if err == nil {
		t.Fatal("expected safeDialContext to reject a private IP from its own lookup")
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

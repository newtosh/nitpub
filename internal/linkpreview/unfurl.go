package linkpreview

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const maxBodyBytes = 1 << 20

// Preview holds Open Graph metadata for a link card.
type Preview struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

var client = &http.Client{
	Timeout: 12 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return assertSafeURL(req.URL)
	},
	// Transport pins every connection (initial request and each redirect
	// hop) to an IP resolved and validated at dial time, via safeDialContext
	// — see its doc comment for why assertSafeURL alone isn't enough.
	Transport: &http.Transport{
		DialContext: safeDialContext,
	},
}

// lookupIP resolves a hostname's IPs. A package var (rather than a direct
// net.LookupIP call) so tests can stub DNS resolution to simulate
// rebinding — a resolver that answers differently between the pre-flight
// assertSafeURL check and the actual connect-time lookup.
var lookupIP = net.LookupIP

// safeDialContext is the http.Transport's DialContext. assertSafeURL's
// hostname validation happens before the request is sent, but the
// transport's default dialer re-resolves DNS independently at connect
// time — a DNS-rebinding response could pass assertSafeURL with a public
// IP, then resolve to a private one moments later when the transport
// actually connects (TOCTOU). safeDialContext closes that gap by doing its
// own resolution-and-validation immediately before dialing, then dialing
// the exact IP it just validated — there is no separate "validate" step
// whose result could go stale before "connect" runs.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("blocked host")
	}
	ips, err := lookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup failed")
	}

	var dialer net.Dialer
	var lastErr error
	for _, ip := range ips {
		if isBlockedIP(ip) {
			lastErr = fmt.Errorf("blocked host")
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no safe address found")
	}
	return nil, lastErr
}

// Fetch loads OG metadata for an https? URL.
func Fetch(rawURL string) (Preview, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return Preview{}, fmt.Errorf("invalid url")
	}
	if err := assertSafeURL(u); err != nil {
		return Preview{}, err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return Preview{}, err
	}
	req.Header.Set("User-Agent", "nitpub-linkpreview/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := client.Do(req)
	if err != nil {
		return Preview{}, fmt.Errorf("fetch failed")
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Preview{}, fmt.Errorf("fetch status %d", res.StatusCode)
	}

	ct := res.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "text/html") && !strings.Contains(strings.ToLower(ct), "application/xhtml") {
		return Preview{}, fmt.Errorf("unsupported content type")
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxBodyBytes))
	if err != nil {
		return Preview{}, err
	}

	preview := parseHTML(u, body)
	if preview.Title == "" {
		preview.Title = u.Hostname()
	}
	preview.URL = u.String()
	return preview, nil
}

func assertSafeURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("blocked host")
	}
	ips, err := lookupIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed")
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("blocked host")
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	return false
}

func parseHTML(base *url.URL, body []byte) Preview {
	var p Preview
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return p
	}
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "meta" {
			var name, property, content string
			for _, a := range n.Attr {
				switch strings.ToLower(a.Key) {
				case "name":
					name = strings.ToLower(a.Val)
				case "property":
					property = strings.ToLower(a.Val)
				case "content":
					content = a.Val
				}
			}
			key := property
			if key == "" {
				key = name
			}
			switch key {
			case "og:title":
				if p.Title == "" {
					p.Title = strings.TrimSpace(content)
				}
			case "og:description", "description":
				if p.Description == "" {
					p.Description = strings.TrimSpace(content)
				}
			case "og:image":
				if p.Image == "" {
					p.Image = resolveURL(base, content)
				}
			case "og:site_name":
				if p.SiteName == "" {
					p.SiteName = strings.TrimSpace(content)
				}
			}
		}
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil && title == "" {
			title = strings.TrimSpace(n.FirstChild.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if p.Title == "" {
		p.Title = title
	}
	if p.SiteName == "" {
		p.SiteName = base.Hostname()
	}
	return p
}

func resolveURL(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

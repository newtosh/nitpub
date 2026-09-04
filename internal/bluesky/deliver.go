package bluesky

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/newtosh/nitpub/internal/linkpreview"
	"github.com/newtosh/nitpub/internal/outbox"
)

const maxImages = 4

// maxImageBytes caps a fetched image's size before it's handed to
// UploadBlob. Bluesky's own blob limit is smaller (~1MB); this is a
// generous upper bound so a misbehaving/huge remote image can't exhaust
// memory during fetch, not an attempt to match their exact limit.
const maxImageBytes = 5 << 20

var mdImageRef = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

// imgRef is one image reference extracted from a post's markdown content.
type imgRef struct {
	URL string
	Alt string
}

// extractImages pulls up to maxImages `![alt](url)` image references out of
// post content, in document order. total is the number actually found
// (before capping) — a caller uses total > len(images) to detect and record
// truncation (KTD7).
func extractImages(content string) (images []imgRef, total int) {
	matches := mdImageRef.FindAllStringSubmatch(content, -1)
	total = len(matches)
	for i, m := range matches {
		if i >= maxImages {
			break
		}
		images = append(images, imgRef{Alt: m[1], URL: m[2]})
	}
	return images, total
}

// imageEmbed and imageEmbedItem are the app.bsky.embed.images lexicon shape.
type imageEmbed struct {
	Type   string           `json:"$type"`
	Images []imageEmbedItem `json:"images"`
}

type imageEmbedItem struct {
	Image BlobRef `json:"image"`
	Alt   string  `json:"alt"`
}

// externalEmbed and externalEmbedData are the app.bsky.embed.external
// lexicon shape.
type externalEmbed struct {
	Type     string            `json:"$type"`
	External externalEmbedData `json:"external"`
}

type externalEmbedData struct {
	URI         string   `json:"uri"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Thumb       *BlobRef `json:"thumb,omitempty"`
}

// fetchImageBytes and fetchLinkPreview are package vars (not direct calls)
// so tests can point them at an httptest.Server without going through the
// SSRF guard meant for arbitrary remote URLs.
var fetchImageBytes = realFetchImageBytes
var fetchLinkPreview = linkpreview.Fetch

// buildImageEmbed uploads each image (R6) and returns the resulting embed.
// Any fetch or upload failure aborts the whole post — a partial image embed
// isn't useful, and CreateRecord must never be called with one.
func buildImageEmbed(ctx context.Context, client *Client, accessJWT string, images []imgRef) (imageEmbed, error) {
	items := make([]imageEmbedItem, 0, len(images))
	for _, img := range images {
		data, mimeType, err := fetchImageBytes(ctx, img.URL)
		if err != nil {
			return imageEmbed{}, fmt.Errorf("bluesky: fetch image %s: %w", img.URL, err)
		}
		blob, err := client.UploadBlob(ctx, accessJWT, data, mimeType)
		if err != nil {
			return imageEmbed{}, fmt.Errorf("bluesky: upload image %s: %w", img.URL, err)
		}
		items = append(items, imageEmbedItem{Image: blob, Alt: img.Alt})
	}
	return imageEmbed{Type: "app.bsky.embed.images", Images: items}, nil
}

// buildExternalEmbed builds a link card (R7) from linkpreview's OG metadata.
// linkpreview.Fetch failing (dead link, blocked host, timeout, ...) falls
// back to a bare-URI embed rather than failing the whole post (KTD11) — a
// plain link is still better than no post at all. A thumbnail upload
// failure is likewise best-effort: the card just goes out without one.
func buildExternalEmbed(ctx context.Context, client *Client, accessJWT, linkURL string) externalEmbed {
	preview, err := fetchLinkPreview(linkURL)
	if err != nil {
		return externalEmbed{Type: "app.bsky.embed.external", External: externalEmbedData{URI: linkURL}}
	}

	var thumb *BlobRef
	if preview.Image != "" {
		if data, mimeType, ferr := fetchImageBytes(ctx, preview.Image); ferr == nil {
			if blob, uerr := client.UploadBlob(ctx, accessJWT, data, mimeType); uerr == nil {
				thumb = &blob
			}
		}
	}

	return externalEmbed{
		Type: "app.bsky.embed.external",
		External: externalEmbedData{
			URI:         linkURL,
			Title:       preview.Title,
			Description: preview.Description,
			Thumb:       thumb,
		},
	}
}

// externalLinkURL determines the URL an external-embed card should point
// at: the first link facet BuildPostText already found (e.g. a truncated
// note's "Read more" link), or failing that the same URL content.go would
// have used — the quote's source for a quote post, the post's own canonical
// URL otherwise.
func externalLinkURL(post *outbox.Post, text BlueskyPostText) string {
	for _, f := range text.Facets {
		for _, feat := range f.Features {
			if feat.URI != "" {
				return feat.URI
			}
		}
	}
	if post.Kind == outbox.KindQuote && post.Quote != nil {
		return post.Quote.SourceURL
	}
	return post.ID
}

// blueskyWebURL converts CreateRecord's at://<did>/app.bsky.feed.post/<rkey>
// URI into a browsable https://bsky.app/... link — the raw at:// form isn't
// a URL a browser can open, so storing it verbatim (the original shape of
// this function, before review caught it) made the "posted" badge's link
// silently dead. Falls back to the at:// URI unchanged if it doesn't match
// the expected shape, rather than producing a broken bsky.app link.
func blueskyWebURL(atURI, did string) string {
	const prefix = "at://"
	rest := strings.TrimPrefix(atURI, prefix)
	if rest == atURI {
		return atURI
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[1] != feedPostCollection {
		return atURI
	}
	rkey := parts[2]
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", did, rkey)
}

// isTooLongError reports whether err looks like Bluesky rejecting a record
// for exceeding its server-side text length limit. Bluesky's exact wording
// isn't documented as a stable contract, so this matches defensively on the
// vocabulary its errors are known to use rather than one exact string.
func isTooLongError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "too long") || strings.Contains(msg, "graphemes")
}

// refreshAccessToken exchanges auth's refresh token for a new session and
// persists the rotated refresh token. If the refresh call itself fails —
// not just an expired access token, the refresh token itself being
// rejected — the stored connection is marked invalid (KTD12) so a status
// endpoint can surface needs_reconnect instead of a per-post error.
func refreshAccessToken(ctx context.Context, client *Client, authStore *AuthStore, auth *Auth) (string, error) {
	session, err := client.RefreshSession(ctx, auth.RefreshJWT)
	if err != nil {
		invalid := *auth
		invalid.Invalid = true
		if putErr := authStore.Put(invalid); putErr != nil {
			return "", fmt.Errorf("bluesky: refresh session failed: %w (and failed to mark connection invalid: %v)", err, putErr)
		}
		return "", fmt.Errorf("bluesky: connection invalid, reconnect required: %w", err)
	}
	if err := authStore.Put(Auth{DID: session.DID, Handle: session.Handle, RefreshJWT: session.RefreshJWT}); err != nil {
		return "", err
	}
	return session.AccessJWT, nil
}

// deliverOnce builds and sends one post: text, an image or external-link
// embed, and a too-long retry if Bluesky rejects the text on length. It does
// not touch auth — a 401 anywhere in here is returned as-is for Deliver to
// handle via refresh-and-retry.
func deliverOnce(ctx context.Context, client *Client, accessJWT, did string, post *outbox.Post) (uri string, truncated bool, err error) {
	images, totalImages := extractImages(post.Content)
	truncated = totalImages > len(images)

	text, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		return "", truncated, err
	}

	var embed interface{}
	if len(images) > 0 {
		built, ierr := buildImageEmbed(ctx, client, accessJWT, images)
		if ierr != nil {
			return "", truncated, ierr
		}
		embed = built
	} else if linkURL := externalLinkURL(post, text); linkURL != "" {
		embed = buildExternalEmbed(ctx, client, accessJWT, linkURL)
	}

	record := PostRecord{Text: text.Text, Facets: text.Facets, Embed: embed}
	uri, _, err = client.CreateRecord(ctx, accessJWT, did, record)
	if err != nil && isTooLongError(err) {
		harder, terr := BuildPostText(post, MaxGraphemes-20)
		if terr == nil {
			record.Text = harder.Text
			record.Facets = harder.Facets
			uri, _, err = client.CreateRecord(ctx, accessJWT, did, record)
		}
	}
	if err != nil {
		return "", truncated, err
	}
	return uri, truncated, nil
}

// Deliver sends post to Bluesky and records the outcome via
// outboxSvc.SetBluesky — Status "posted" with the resulting URI on success,
// or "error" with the failure message. It never writes both. The caller is
// responsible for having already set Status "pending" before calling this.
func Deliver(ctx context.Context, client *Client, authStore *AuthStore, outboxSvc *outbox.Service, post *outbox.Post) error {
	auth, err := authStore.Get()
	if err != nil {
		return err
	}
	if auth == nil {
		return fmt.Errorf("bluesky: not connected")
	}

	accessJWT, err := refreshAccessToken(ctx, client, authStore, auth)
	if err != nil {
		return err
	}

	slug := outbox.PostSlug(post.ID)

	uri, truncated, err := deliverOnce(ctx, client, accessJWT, auth.DID, post)
	if err != nil && IsUnauthorized(err) {
		accessJWT, err = refreshAccessToken(ctx, client, authStore, auth)
		if err != nil {
			return err
		}
		uri, truncated, err = deliverOnce(ctx, client, accessJWT, auth.DID, post)
	}
	if err != nil {
		if _, setErr := outboxSvc.SetBluesky(slug, outbox.BlueskyState{Status: "error", Error: err.Error()}); setErr != nil {
			return setErr
		}
		return err
	}

	now := time.Now().UTC()
	_, err = outboxSvc.SetBluesky(slug, outbox.BlueskyState{
		Status:    "posted",
		PostedAt:  &now,
		URI:       blueskyWebURL(uri, auth.DID),
		Truncated: truncated,
	})
	return err
}

// --- SSRF-safe image fetch, mirroring internal/linkpreview/unfurl.go's
// safeDialContext/assertSafeURL pattern (see its doc comments for why the
// dial-time re-check matters, not just the pre-flight one). Duplicated
// rather than imported because those helpers are unexported and fetching
// raw image bytes isn't linkpreview's job.

var imageHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return assertSafeImageURL(req.URL)
	},
	Transport: &http.Transport{DialContext: safeImageDialContext},
}

var lookupImageIP = net.LookupIP

func safeImageDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, fmt.Errorf("blocked host")
	}
	ips, err := lookupImageIP(host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup failed")
	}

	var dialer net.Dialer
	var lastErr error
	for _, ip := range ips {
		if isBlockedImageIP(ip) {
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

func assertSafeImageURL(u *url.URL) error {
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
	ips, err := lookupImageIP(host)
	if err != nil {
		return fmt.Errorf("dns lookup failed")
	}
	for _, ip := range ips {
		if isBlockedImageIP(ip) {
			return fmt.Errorf("blocked host")
		}
	}
	return nil
}

func isBlockedImageIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified()
}

// realFetchImageBytes is fetchImageBytes' real implementation — SSRF-safe
// per the guard above.
func realFetchImageBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid image url")
	}
	if err := assertSafeImageURL(u); err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "nitpub-bluesky/1.0")

	res, err := imageHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetch image failed")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fetch image status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, maxImageBytes))
	if err != nil {
		return nil, "", err
	}
	mimeType := res.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(body)
	}
	return body, mimeType, nil
}

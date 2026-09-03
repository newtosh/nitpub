package outbox

import (
	"strings"
	"testing"
)

func TestNoteFederationHTMLBold(t *testing.T) {
	html, err := NoteFederationHTML("Hello **fediverse**")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<strong>fediverse</strong>") {
		t.Fatalf("html = %q", html)
	}
	if strings.Contains(html, "**") {
		t.Fatalf("raw markdown leaked: %q", html)
	}
}

func TestNoteFederationHTMLStripsCallout(t *testing.T) {
	md := "> [!TIP]\n> Remember this."
	html, err := NoteFederationHTML(md)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "[!TIP]") {
		t.Fatalf("callout marker leaked: %q", html)
	}
	if !strings.Contains(html, "Remember this.") {
		t.Fatalf("missing body: %q", html)
	}
}

func TestNoteFederationHTMLHandleInCode(t *testing.T) {
	html, err := NoteFederationHTML("This is the first federated post from **nitpub** at `@nit@nitpub.com`.")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("html = %q", html)
	if strings.Contains(html, `\`) {
		t.Fatalf("unexpected backslash in html: %q", html)
	}
	if !strings.Contains(html, "<code>@\u200bnit@\u200bnitpub.com</code>\u200b.") {
		t.Fatalf("html = %q", html)
	}
}

func TestNoteFederationHTMLStripsImages(t *testing.T) {
	html, err := NoteFederationHTML("Look ![alt](/media/x.png) here")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<img") || strings.Contains(html, "/media/x.png") {
		t.Fatalf("image leaked: %q", html)
	}
}

// TestNoteFederationHTMLQuotePostShape feeds BuildQuoteContent's composed
// markdown through the real NoteFederationHTML conversion (goldmark +
// bluemonday, not a stub) and asserts the sanitized HTML preserves the
// link, blockquote, commentary, and via line in that order — the shape R4
// promises stays identical across every quote post (U5).
func TestNoteFederationHTMLQuotePostShape(t *testing.T) {
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Title:      "Example Article",
		Excerpt:    "The excerpt text.",
		Commentary: "My commentary paragraph.",
		Via:        "Some Friend",
	}
	md, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	html, err := NoteFederationHTML(md)
	if err != nil {
		t.Fatal(err)
	}

	linkIdx := strings.Index(html, `<a href="https://example.com/article"`)
	blockquoteIdx := strings.Index(html, "<blockquote>")
	commentaryIdx := strings.Index(html, "My commentary paragraph.")
	viaIdx := strings.Index(html, "via Some Friend")
	if linkIdx == -1 || blockquoteIdx == -1 || commentaryIdx == -1 || viaIdx == -1 {
		t.Fatalf("missing expected element(s): %q", html)
	}
	if linkIdx >= blockquoteIdx || blockquoteIdx >= commentaryIdx || commentaryIdx >= viaIdx {
		t.Fatalf("expected link, blockquote, commentary, via in that order, got: %q", html)
	}
	if !strings.Contains(html, "The excerpt text.") {
		t.Fatalf("expected blockquote to carry the excerpt text, got %q", html)
	}
	if !strings.Contains(html, ">Example Article<") {
		t.Fatalf("expected link text to be the fetched title, got %q", html)
	}
}

// TestNoteFederationHTMLQuotePostStripsUnsafeMarkup reuses the note-kind
// sanitization guarantee (see TestNoteFederationHTMLStripsImages above) on a
// quote-post fixture: raw HTML slipped into the excerpt/commentary through
// the real bluemonday policy must not survive, since Mastodon renders this
// content unescaped.
func TestNoteFederationHTMLQuotePostStripsUnsafeMarkup(t *testing.T) {
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Excerpt:    "Excerpt <script>alert(1)</script> text.",
		Commentary: "Take this <img src=x onerror=alert(1)>.",
	}
	md, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	html, err := NoteFederationHTML(md)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "<script") || strings.Contains(html, "<img") || strings.Contains(html, "onerror=") {
		t.Fatalf("unsafe markup leaked into sanitized quote HTML: %q", html)
	}
}

// TestNoteFederationHTMLQuotePostOmitsViaWhenBlank covers AE4 at the
// federation-HTML layer: a via-less quote post's sanitized HTML contains no
// via text anywhere, not just a blank/empty via line.
func TestNoteFederationHTMLQuotePostOmitsViaWhenBlank(t *testing.T) {
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Excerpt:    "The excerpt text.",
		Commentary: "My commentary paragraph.",
	}
	md, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	html, err := NoteFederationHTML(md)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "via") {
		t.Fatalf("expected no via text in HTML when Via is blank, got: %q", html)
	}
}

func TestArticleFederationSummary(t *testing.T) {
	content := "# My Title\n\nBody with **markdown** and a link https://example.com"
	got := ArticleFederationSummary(content)
	if strings.Contains(got, "**") || strings.Contains(got, "#") {
		t.Fatalf("markdown leaked: %q", got)
	}
	if !strings.Contains(got, "Body with markdown") {
		t.Fatalf("summary = %q", got)
	}
}

func TestArticleFederationContentHTMLIncludesTitleAndLinkedHost(t *testing.T) {
	content := "My Title\n\nBody with **markdown** content."
	got := ArticleFederationContentHTML("https://nwtn.sh/posts/abc-123", content)

	if !strings.Contains(got, "<strong>My Title</strong>") {
		t.Fatalf("title missing from content: %q", got)
	}
	if !strings.Contains(got, "Body with markdown content.") {
		t.Fatalf("excerpt missing from content: %q", got)
	}
	if !strings.Contains(got, `<a href="https://nwtn.sh/posts/abc-123" rel="nofollow noopener" target="_blank">nwtn.sh</a>`) {
		t.Fatalf("expected a clickable link naming the host, got: %q", got)
	}
}

func TestArticleFederationContentHTMLEscapesTitleAndExcerpt(t *testing.T) {
	content := "<script>alert(1)</script>\n\nBody with <b>raw html</b>."
	got := ArticleFederationContentHTML("https://nwtn.sh/posts/xyz", content)

	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>raw html</b>") {
		t.Fatalf("unescaped author-controlled html leaked into federated content: %q", got)
	}
}

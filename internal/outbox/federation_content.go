package outbox

import (
	"bytes"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// Mastodon renders Note.content as sanitized HTML, not Markdown.
// See https://docs.joinmastodon.org/spec/activitypub/#sanitization
var mastodonHTMLPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"p", "br", "span",
		"a", "del", "pre", "code",
		"em", "strong", "b", "i", "u",
		"ul", "ol", "li", "blockquote",
	)
	p.AllowAttrs("href", "rel", "class").OnElements("a")
	p.AllowAttrs("class").OnElements("span")
	p.AllowAttrs("start", "reversed").OnElements("ol")
	p.AllowAttrs("value").OnElements("li")
	return p
}()

var (
	githubAlertLine = regexp.MustCompile(`(?m)^>\s*\[!([A-Za-z]+)\]\s*(.*)$`)
	embedLine       = regexp.MustCompile(`(?m)^\s*(https?://(?:www\.)?(?:youtube\.com|youtu\.be|vimeo\.com|open\.spotify\.com)\S+)\s*$`)
	imageLine       = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
	codeTagContent  = regexp.MustCompile(`<code>([^<]*)</code>`)
	fediverseHandle = regexp.MustCompile(`@([A-Za-z0-9_]+)@([A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z]{2,})`)
	codePeriod      = regexp.MustCompile(`</code>\.`)
)

const zwsp = "\u200b"

// NoteFederationHTML converts stored note markdown into HTML Mastodon can render.
func NoteFederationHTML(markdown string) (string, error) {
	md := preprocessNoteForFederation(markdown)
	var buf bytes.Buffer
	gm := goldmark.New(
		goldmark.WithExtensions(extension.Linkify),
	)
	if err := gm.Convert([]byte(md), &buf); err != nil {
		return "", err
	}
	return shieldMentionsInCodeHTML(mastodonHTMLPolicy.Sanitize(buf.String())), nil
}

// shieldMentionsInCodeHTML stops Mastodon from autolinking handles inside <code>.
// Entity-escaping @ alone is not enough — Mastodon decodes entities then mention-parses.
// Zero-width spaces break mention regex without visible change.
func shieldMentionsInCodeHTML(html string) string {
	html = codeTagContent.ReplaceAllStringFunc(html, func(match string) string {
		parts := codeTagContent.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return "<code>" + shieldCodeText(parts[1]) + "</code>"
	})
	return codePeriod.ReplaceAllString(html, "</code>"+zwsp+".")
}

func shieldCodeText(s string) string {
	return fediverseHandle.ReplaceAllStringFunc(s, func(m string) string {
		var b strings.Builder
		for _, r := range m {
			if r == '@' {
				b.WriteRune('@')
				b.WriteString(zwsp)
				continue
			}
			b.WriteRune(r)
		}
		return b.String()
	})
}

// ArticleFederationSummary returns plain text for the federated Note summary.
func ArticleFederationSummary(content string) string {
	body := articlePlainBody(content)
	if body == "" {
		body = articlePlainTitle(content)
	}
	return truncateRunes(body, 280)
}

// ArticleFederationContentHTML builds the Mastodon-compatible mirror Note's
// visible content: the article's title, an excerpt, and a real "Read more"
// hyperlink using the post URL's host as link text — Mastodon otherwise
// renders a bare permalink as inert plain text, not a clickable link.
//
// Earlier this went through note.Summary instead, which AS2 defines as the
// spoiler_text/content-warning field: Mastodon hid the excerpt behind an
// unwanted "show more" toggle and dropped the title line entirely (it was
// never included in the excerpt to begin with). This puts everything in
// note.Content instead, so it renders as a normal, fully visible post.
func ArticleFederationContentHTML(postID, content string) string {
	title := articlePlainTitle(content)
	excerpt := ArticleFederationSummary(content)
	linkText := postID
	if u, err := url.Parse(postID); err == nil && u.Host != "" {
		linkText = u.Host
	}

	var b strings.Builder
	if title != "" {
		b.WriteString("<p><strong>")
		b.WriteString(html.EscapeString(title))
		b.WriteString("</strong></p>")
	}
	if excerpt != "" {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(excerpt))
		b.WriteString("</p>")
	}
	b.WriteString(`<p>Read more on <a href="`)
	b.WriteString(html.EscapeString(postID))
	b.WriteString(`" rel="nofollow noopener" target="_blank">`)
	b.WriteString(html.EscapeString(linkText))
	b.WriteString("</a></p>")
	return b.String()
}

func preprocessNoteForFederation(markdown string) string {
	md := githubAlertLine.ReplaceAllString(markdown, "> **$1:** $2")
	md = imageLine.ReplaceAllString(md, "")
	md = embedLine.ReplaceAllString(md, "$1")
	return strings.TrimSpace(md)
}

func articlePlainTitle(content string) string {
	line := strings.TrimSpace(firstLine(content))
	return strings.TrimLeft(line, "# ")
}

func articlePlainBody(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) < 2 {
		return ""
	}
	return strings.TrimSpace(stripMarkdownLite(lines[1]))
}

func firstLine(content string) string {
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		return content[:i]
	}
	return content
}

func stripMarkdownLite(s string) string {
	s = githubAlertLine.ReplaceAllString(s, "$2")
	s = imageLine.ReplaceAllString(s, "")
	s = embedLine.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

package bluesky

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rivo/uniseg"

	"github.com/newtosh/nitpub/internal/outbox"
)

// MaxGraphemes is Bluesky's post text limit (300 grapheme clusters, R4/R5).
const MaxGraphemes = 300

const facetLinkType = "app.bsky.richtext.facet#link"

// FacetIndex marks a byte range within a Facet's post text.
type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

// FacetFeature is one rich-text feature attached to a Facet range. This
// package only ever produces the link feature.
type FacetFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri"`
}

// Facet marks a byte range of BlueskyPostText.Text as a rich-text feature —
// AT Protocol's app.bsky.richtext.facet lexicon. Bluesky's post renderer has
// no markdown/HTML step: plain text renders as plain text, and only a facet
// makes a range tappable, so every link in the output text needs one.
type Facet struct {
	Index    FacetIndex     `json:"index"`
	Features []FacetFeature `json:"features"`
}

// BlueskyPostText is post text ready for PostRecord.Text, plus the facets
// needed to render its link(s) as tappable.
type BlueskyPostText struct {
	Text   string
	Facets []Facet
}

// BuildPostText builds Bluesky post text (and link facets) from a nitpub
// post, fit to maxGraphemes grapheme clusters (normally MaxGraphemes; a
// caller retrying after CreateRecord rejects a post as too long — Bluesky
// measures independently server-side — can pass a harder budget to
// re-truncate).
func BuildPostText(post *outbox.Post, maxGraphemes int) (BlueskyPostText, error) {
	if maxGraphemes <= 0 {
		maxGraphemes = MaxGraphemes
	}
	switch post.Kind {
	case outbox.KindArticle:
		return buildArticleText(post, maxGraphemes), nil
	case outbox.KindQuote:
		return buildQuoteText(post, maxGraphemes)
	default:
		return buildNoteText(post, maxGraphemes), nil
	}
}

func buildNoteText(post *outbox.Post, maxGraphemes int) BlueskyPostText {
	text, facets := markdownToPlainText(post.Content)
	if uniseg.GraphemeClusterCount(text) <= maxGraphemes {
		return BlueskyPostText{Text: text, Facets: facets}
	}
	return truncateWithReadMoreLink(text, post.ID, maxGraphemes)
}

// truncateWithReadMoreLink truncates text to leave room for a trailing
// "Read more: <url>" line, and facets that URL (R4). Any facets found
// within the truncated-away tail are simply dropped along with it.
func truncateWithReadMoreLink(text, url string, maxGraphemes int) BlueskyPostText {
	const label = "\n\nRead more: "
	suffix := label + url
	budget := maxGraphemes - uniseg.GraphemeClusterCount(suffix)
	if budget < 0 {
		budget = 0
	}
	body := strings.TrimRight(truncateToGraphemes(text, budget), " \t\n\r")

	full := body + suffix
	start := len(body) + len(label)
	end := start + len(url)
	return BlueskyPostText{
		Text: full,
		Facets: []Facet{{
			Index:    FacetIndex{ByteStart: start, ByteEnd: end},
			Features: []FacetFeature{{Type: facetLinkType, URI: url}},
		}},
	}
}

// buildArticleText always produces an excerpt + link, never the full
// article body (R5), regardless of how short or long the article is.
func buildArticleText(post *outbox.Post, maxGraphemes int) BlueskyPostText {
	title := articleTitle(post.Content)
	bodyText, _ := markdownToPlainText(articleBody(post.Content))
	excerpt := firstParagraph(bodyText)

	url := post.ID
	prefix := ""
	if title != "" {
		prefix = title + "\n\n"
	}
	const sep = "\n\n"
	suffix := sep + url

	budget := maxGraphemes - uniseg.GraphemeClusterCount(prefix) - uniseg.GraphemeClusterCount(suffix)
	if budget < 0 {
		budget = 0
	}
	if uniseg.GraphemeClusterCount(excerpt) > budget {
		trimBudget := budget - 1 // reserve one grapheme for the ellipsis
		if trimBudget < 0 {
			trimBudget = 0
		}
		excerpt = strings.TrimRight(truncateToGraphemes(excerpt, trimBudget), " \t\n\r") + "…"
	}

	full := prefix + excerpt + suffix
	start := len(prefix) + len(excerpt) + len(sep)
	end := start + len(url)
	return BlueskyPostText{
		Text: full,
		Facets: []Facet{{
			Index:    FacetIndex{ByteStart: start, ByteEnd: end},
			Features: []FacetFeature{{Type: facetLinkType, URI: url}},
		}},
	}
}

// buildQuoteText composes a quote post's excerpt, optional commentary, and
// source link, truncating commentary first and only then the excerpt when
// over budget — the source link is never truncated or dropped, and the
// excerpt is never dropped entirely (only shortened).
func buildQuoteText(post *outbox.Post, maxGraphemes int) (BlueskyPostText, error) {
	q := post.Quote
	if q == nil {
		return BlueskyPostText{}, fmt.Errorf("bluesky: quote post is missing QuoteFields")
	}
	url := strings.TrimSpace(q.SourceURL)
	excerpt := strings.TrimSpace(q.Excerpt)
	commentary := strings.TrimSpace(q.Commentary)

	build := func(excerpt, commentary string) BlueskyPostText {
		var parts []string
		if excerpt != "" {
			parts = append(parts, excerpt)
		}
		if commentary != "" {
			parts = append(parts, commentary)
		}
		parts = append(parts, url)
		text := strings.Join(parts, "\n\n")
		start := len(text) - len(url)
		return BlueskyPostText{
			Text: text,
			Facets: []Facet{{
				Index:    FacetIndex{ByteStart: start, ByteEnd: len(text)},
				Features: []FacetFeature{{Type: facetLinkType, URI: url}},
			}},
		}
	}

	if got := build(excerpt, commentary); uniseg.GraphemeClusterCount(got.Text) <= maxGraphemes {
		return got, nil
	}

	// Over budget: shrink commentary first, down to nothing.
	for n := uniseg.GraphemeClusterCount(commentary); n >= 0; n-- {
		c := strings.TrimRight(truncateToGraphemes(commentary, n), " \t\n\r")
		if got := build(excerpt, c); uniseg.GraphemeClusterCount(got.Text) <= maxGraphemes {
			return got, nil
		}
	}

	// Commentary is fully dropped and it's still over budget: shrink the
	// excerpt too, but never to nothing.
	for n := uniseg.GraphemeClusterCount(excerpt); n > 0; n-- {
		e := strings.TrimRight(truncateToGraphemes(excerpt, n), " \t\n\r")
		if got := build(e, ""); uniseg.GraphemeClusterCount(got.Text) <= maxGraphemes {
			return got, nil
		}
	}

	// Even a 1-grapheme excerpt plus the source link doesn't fit — the
	// budget is smaller than the link itself allows for. Best effort: keep
	// the link, which must never be dropped.
	return build("", ""), nil
}

var (
	mdImage  = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`)
	mdLink   = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	mdHeader = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	multiNL  = regexp.MustCompile(`\n{3,}`)
)

// markdownToPlainText strips common nitpub markdown syntax down to plain
// text, tracking each link's URL as a facet over the *output* byte range.
// Images are dropped entirely (no useful text for a Bluesky post). This is
// deliberately minimal, not a full markdown parser — it only needs to
// handle the syntax nitpub's compose UI actually produces.
func markdownToPlainText(md string) (string, []Facet) {
	md = mdImage.ReplaceAllString(md, "")
	md = multiNL.ReplaceAllString(md, "\n\n")
	md = mdHeader.ReplaceAllString(md, "")

	var b strings.Builder
	var facets []Facet
	last := 0
	for _, loc := range mdLink.FindAllStringSubmatchIndex(md, -1) {
		b.WriteString(stripInlineMarks(md[last:loc[0]]))
		url := md[loc[4]:loc[5]]
		start := b.Len()
		b.WriteString(url)
		facets = append(facets, Facet{
			Index:    FacetIndex{ByteStart: start, ByteEnd: b.Len()},
			Features: []FacetFeature{{Type: facetLinkType, URI: url}},
		})
		last = loc[1]
	}
	b.WriteString(stripInlineMarks(md[last:]))

	text := b.String()
	trimmed := strings.TrimLeft(text, " \t\n\r")
	shift := len(text) - len(trimmed)
	trimmed = strings.TrimRight(trimmed, " \t\n\r")
	for i := range facets {
		facets[i].Index.ByteStart -= shift
		facets[i].Index.ByteEnd -= shift
	}
	return trimmed, facets
}

func stripInlineMarks(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "`", "")
	return s
}

func articleTitle(content string) string {
	first := content
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		first = content[:i]
	}
	return strings.TrimLeft(strings.TrimSpace(first), "# ")
}

func articleBody(content string) string {
	parts := strings.SplitN(content, "\n", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func firstParagraph(s string) string {
	parts := strings.SplitN(s, "\n\n", 2)
	return strings.TrimSpace(parts[0])
}

// truncateToGraphemes returns the longest prefix of s with at most n
// grapheme clusters, cutting on a UAX #29 grapheme cluster boundary rather
// than a rune or byte offset — a byte/rune cut could split a multi-codepoint
// emoji or other cluster mid-sequence.
func truncateToGraphemes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	gr := uniseg.NewGraphemes(s)
	count := 0
	end := 0
	for gr.Next() {
		count++
		if count > n {
			return s[:end]
		}
		_, to := gr.Positions()
		end = to
	}
	return s
}

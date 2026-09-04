package bluesky

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"

	"github.com/newtosh/nitpub/internal/outbox"
)

func uniCount(s string) int {
	return uniseg.GraphemeClusterCount(s)
}

func TestBuildPostText_NoteVerbatimUnderBudget(t *testing.T) {
	post := &outbox.Post{ID: "https://nit.pub/posts/abc", Kind: outbox.KindNote, Content: "hello world, a short note"}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Text != post.Content {
		t.Fatalf("expected verbatim text, got %q", got.Text)
	}
	if len(got.Facets) != 0 {
		t.Fatalf("expected no facets for a plain note, got %+v", got.Facets)
	}
}

func TestBuildPostText_NoteWithLinkGetsFacet(t *testing.T) {
	post := &outbox.Post{ID: "https://nit.pub/posts/abc", Kind: outbox.KindNote, Content: "check out [nitpub](https://nit.pub) today"}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got.Text, "[") || strings.Contains(got.Text, "]") {
		t.Fatalf("expected markdown link syntax stripped, got %q", got.Text)
	}
	if !strings.Contains(got.Text, "https://nit.pub") {
		t.Fatalf("expected URL to survive in plain text, got %q", got.Text)
	}
	if len(got.Facets) != 1 {
		t.Fatalf("expected exactly one facet, got %+v", got.Facets)
	}
	f := got.Facets[0]
	if f.Features[0].URI != "https://nit.pub" {
		t.Fatalf("facet URI mismatch: %+v", f)
	}
	if got.Text[f.Index.ByteStart:f.Index.ByteEnd] != "https://nit.pub" {
		t.Fatalf("facet byte range doesn't point at the URL: %q", got.Text[f.Index.ByteStart:f.Index.ByteEnd])
	}
}

func TestBuildPostText_NoteOverBudgetTruncatesWithReadMoreLink(t *testing.T) {
	long := strings.Repeat("a very long sentence that keeps going on and on. ", 20)
	post := &outbox.Post{ID: "https://nit.pub/posts/xyz", Kind: outbox.KindNote, Content: long}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uniCount(got.Text) > MaxGraphemes {
		t.Fatalf("truncated text still over budget: %d graphemes", uniCount(got.Text))
	}
	if !strings.Contains(got.Text, post.ID) {
		t.Fatalf("expected read-more link back to canonical post, got %q", got.Text)
	}
	if len(got.Facets) != 1 {
		t.Fatalf("expected exactly one read-more facet, got %+v", got.Facets)
	}
	f := got.Facets[0]
	if got.Text[f.Index.ByteStart:f.Index.ByteEnd] != post.ID {
		t.Fatalf("facet byte range doesn't point at the post URL: %q", got.Text[f.Index.ByteStart:f.Index.ByteEnd])
	}
}

func TestBuildPostText_NoteOverBudgetKeepsEarlyLinkFacet(t *testing.T) {
	// A link inside the kept prefix must stay tappable after truncation —
	// only facets in the truncated-away tail should be dropped.
	link := "[nitpub](https://nit.pub)"
	long := "Check out " + link + " today. " + strings.Repeat("a very long sentence that keeps going on and on. ", 20)
	post := &outbox.Post{ID: "https://nit.pub/posts/xyz", Kind: outbox.KindNote, Content: long}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uniCount(got.Text) > MaxGraphemes {
		t.Fatalf("truncated text still over budget: %d graphemes", uniCount(got.Text))
	}
	if len(got.Facets) != 2 {
		t.Fatalf("expected the early link facet plus the read-more facet, got %+v", got.Facets)
	}
	linkFacet := got.Facets[0]
	if linkFacet.Features[0].URI != "https://nit.pub" {
		t.Fatalf("first facet should be the early link, got %+v", linkFacet)
	}
	if got.Text[linkFacet.Index.ByteStart:linkFacet.Index.ByteEnd] != "https://nit.pub" {
		t.Fatalf("early link facet byte range doesn't point at the URL: %q", got.Text[linkFacet.Index.ByteStart:linkFacet.Index.ByteEnd])
	}
	readMoreFacet := got.Facets[1]
	if got.Text[readMoreFacet.Index.ByteStart:readMoreFacet.Index.ByteEnd] != post.ID {
		t.Fatalf("read-more facet byte range doesn't point at the post URL: %q", got.Text[readMoreFacet.Index.ByteStart:readMoreFacet.Index.ByteEnd])
	}
}

func TestBuildPostText_ArticleAlwaysExcerptAndLinkNeverFullText(t *testing.T) {
	shortArticle := "My Title\n\nJust one short paragraph of body text."
	longArticle := "My Title\n\n" + strings.Repeat("This article body paragraph goes on for a very long time indeed. ", 50)

	for name, content := range map[string]string{"short": shortArticle, "long": longArticle} {
		t.Run(name, func(t *testing.T) {
			post := &outbox.Post{ID: "https://nit.pub/posts/art", Kind: outbox.KindArticle, Content: content}
			got, err := BuildPostText(post, MaxGraphemes)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Text == content {
				t.Fatalf("article text must never be the full article verbatim")
			}
			if uniCount(got.Text) > MaxGraphemes {
				t.Fatalf("article text over budget: %d graphemes", uniCount(got.Text))
			}
			if !strings.Contains(got.Text, post.ID) {
				t.Fatalf("expected article link, got %q", got.Text)
			}
			if len(got.Facets) != 1 || got.Facets[0].Features[0].URI != post.ID {
				t.Fatalf("expected one facet pointing at post ID, got %+v", got.Facets)
			}
		})
	}
}

func TestBuildPostText_QuoteEmptyCommentaryStillFitsExcerptAndLink(t *testing.T) {
	post := &outbox.Post{
		ID:   "https://nit.pub/posts/q1",
		Kind: outbox.KindQuote,
		Quote: &outbox.QuoteFields{
			SourceURL: "https://example.com/article",
			Title:     "Example Article",
			Excerpt:   "A short quoted excerpt.",
		},
	}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got.Text, "A short quoted excerpt.") {
		t.Fatalf("expected excerpt present, got %q", got.Text)
	}
	if !strings.Contains(got.Text, post.Quote.SourceURL) {
		t.Fatalf("expected source URL present, got %q", got.Text)
	}
	if len(got.Facets) != 1 || got.Facets[0].Features[0].URI != post.Quote.SourceURL {
		t.Fatalf("expected facet on source URL, got %+v", got.Facets)
	}
}

func TestBuildPostText_QuoteLongCommentaryTruncatesCommentaryFirst(t *testing.T) {
	excerpt := "The quoted excerpt stays intact."
	commentary := strings.Repeat("my commentary keeps rambling on and on. ", 20)
	post := &outbox.Post{
		ID:   "https://nit.pub/posts/q2",
		Kind: outbox.KindQuote,
		Quote: &outbox.QuoteFields{
			SourceURL:  "https://example.com/article",
			Excerpt:    excerpt,
			Commentary: commentary,
		},
	}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uniCount(got.Text) > MaxGraphemes {
		t.Fatalf("quote text over budget: %d graphemes", uniCount(got.Text))
	}
	if !strings.Contains(got.Text, excerpt) {
		t.Fatalf("expected excerpt to survive untouched, got %q", got.Text)
	}
	if !strings.Contains(got.Text, post.Quote.SourceURL) {
		t.Fatalf("expected source URL to survive untouched, got %q", got.Text)
	}
	if strings.Contains(got.Text, strings.TrimSpace(commentary)) {
		t.Fatalf("expected full commentary to have been truncated, got %q", got.Text)
	}
}

func TestBuildPostText_QuoteExcerptTooLongTruncatesExcerptAfterCommentaryDropped(t *testing.T) {
	excerpt := strings.Repeat("this excerpt is enormous and just keeps going and going and going. ", 20)
	post := &outbox.Post{
		ID:   "https://nit.pub/posts/q3",
		Kind: outbox.KindQuote,
		Quote: &outbox.QuoteFields{
			SourceURL:  "https://example.com/article",
			Excerpt:    excerpt,
			Commentary: "some commentary that must be dropped entirely",
		},
	}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uniCount(got.Text) > MaxGraphemes {
		t.Fatalf("quote text over budget: %d graphemes", uniCount(got.Text))
	}
	if !strings.Contains(got.Text, post.Quote.SourceURL) {
		t.Fatalf("source link must never be truncated away, got %q", got.Text)
	}
	if strings.Contains(got.Text, "some commentary") {
		t.Fatalf("commentary should be fully dropped before excerpt is touched, got %q", got.Text)
	}
	if strings.TrimSpace(excerpt) == extractExcerptLine(got.Text, post.Quote.SourceURL) {
		t.Fatalf("expected excerpt itself to be truncated, not left full-length")
	}
	// the excerpt is never dropped entirely
	if !strings.Contains(got.Text, "this excerpt is enormous") {
		t.Fatalf("expected excerpt to still be present (just shorter), got %q", got.Text)
	}
}

func extractExcerptLine(text, sourceURL string) string {
	return strings.TrimSpace(strings.Replace(text, sourceURL, "", 1))
}

// TestGraphemeCountingNotRuneApproximation would pass under naive rune
// counting for the wrong reason (or fail outright) — family emoji ZWJ
// sequences and flag emoji are made of many Unicode code points (runes) but
// count as one grapheme cluster each. A rune-count approximation would see
// this text as roughly 40+ "characters" long; grapheme-aware counting sees
// far fewer, and that's the whole point of using a UAX #29 library here.
func TestGraphemeCountingNotRuneApproximation(t *testing.T) {
	// 🇺🇸 = 2 runes (regional indicators), 1 grapheme.
	// 👨‍👩‍👧‍👦 = 7 runes (4 people + 3 ZWJ joiners), 1 grapheme.
	text := strings.Repeat("🇺🇸👨‍👩‍👧‍👦", 10) // 10 grapheme "words" of 2 clusters each = 20 graphemes
	naiveRuneCount := len([]rune(text))
	graphemeCount := uniCount(text)

	if graphemeCount != 20 {
		t.Fatalf("expected 20 grapheme clusters, got %d", graphemeCount)
	}
	if naiveRuneCount == graphemeCount {
		t.Fatalf("test is meaningless if rune count equals grapheme count (got %d both)", naiveRuneCount)
	}
	if naiveRuneCount <= graphemeCount {
		t.Fatalf("expected naive rune count (%d) to overcount vs grapheme count (%d)", naiveRuneCount, graphemeCount)
	}

	// Prove BuildPostText's budget check is grapheme-based: a note built
	// entirely from 250 of these 2-rune-or-more clusters (way over 300
	// runes/bytes) must still be treated as fitting a 300-grapheme budget
	// verbatim, because it's under 300 *graphemes*.
	note := strings.Repeat("🇺🇸", 250) // 250 graphemes, 1000 runes
	post := &outbox.Post{ID: "https://nit.pub/posts/emoji", Kind: outbox.KindNote, Content: note}
	got, err := BuildPostText(post, MaxGraphemes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Text != note {
		t.Fatalf("expected verbatim (under grapheme budget despite high rune count), got truncated: %q", got.Text)
	}
}

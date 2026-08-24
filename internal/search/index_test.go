package search

import (
	"testing"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/sitecontent"
)

func TestSearchPostAndPage(t *testing.T) {
	idx := NewIndex()
	posts := []outbox.Post{{
		Kind:    outbox.KindArticle,
		Content: "My Title\n\nuniquekeyword body text",
		ID:      "http://example.test/posts/abc",
	}}
	pages := []sitecontent.Page{{
		Path:  "/about",
		Title: "About",
		Body:  "about uniquekeyword page",
	}}
	idx.Rebuild(posts, pages)
	results := idx.Search("uniquekeyword", 10)
	if len(results) != 2 {
		t.Fatalf("got %d results", len(results))
	}
}

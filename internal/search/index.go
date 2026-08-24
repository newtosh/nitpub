package search

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/sitecontent"
)

// Result is a single search hit.
type Result struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

// Index holds searchable documents in memory.
type Index struct {
	mu      sync.RWMutex
	entries []entry
}

type entry struct {
	Type  string
	Title string
	Body  string
	URL   string
}

// NewIndex builds an empty index.
func NewIndex() *Index {
	return &Index{}
}

// Rebuild replaces the index from posts and markdown site pages.
func (idx *Index) Rebuild(posts []outbox.Post, pages []sitecontent.Page) {
	var entries []entry
	for _, p := range posts {
		title := postTitle(p)
		body := postBody(p)
		slug := slugFromID(p.ID)
		entries = append(entries, entry{
			Type:  "post",
			Title: title,
			Body:  body,
			URL:   "/p/" + slug,
		})
	}
	for _, pg := range pages {
		entries = append(entries, entry{
			Type:  "page",
			Title: pg.Title,
			Body:  pg.Body,
			URL:   pg.Path,
		})
	}
	idx.mu.Lock()
	idx.entries = entries
	idx.mu.Unlock()
}

func postTitle(p outbox.Post) string {
	if p.Kind == outbox.KindArticle {
		line := strings.TrimSpace(strings.Split(p.Content, "\n")[0])
		return strings.TrimLeft(line, "# ")
	}
	line := strings.TrimSpace(strings.Split(p.Content, "\n")[0])
	if strings.HasPrefix(line, "# ") {
		return strings.TrimSpace(strings.TrimPrefix(line, "# "))
	}
	return truncateRunes(line, 80)
}

func postBody(p outbox.Post) string {
	if p.Kind == outbox.KindArticle {
		lines := strings.SplitN(p.Content, "\n", 2)
		if len(lines) < 2 {
			return ""
		}
		return lines[1]
	}
	return p.Content
}

func slugFromID(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// Search finds case-insensitive substring matches.
func (idx *Index) Search(query string, max int) []Result {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	if max <= 0 {
		max = 50
	}
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var out []Result
	for _, e := range idx.entries {
		hay := strings.ToLower(e.Title + "\n" + e.Body)
		if !strings.Contains(hay, q) {
			continue
		}
		out = append(out, Result{
			Type:    e.Type,
			Title:   e.Title,
			Snippet: snippetAround(e.Body, q, 120),
			URL:     e.URL,
		})
		if len(out) >= max {
			break
		}
	}
	return out
}

func snippetAround(body, query string, maxRunes int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	lower := strings.ToLower(body)
	i := strings.Index(lower, query)
	if i < 0 {
		return truncateRunes(body, maxRunes)
	}
	start := i - 40
	if start < 0 {
		start = 0
	}
	snip := body[start:]
	return truncateRunes(snip, maxRunes)
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

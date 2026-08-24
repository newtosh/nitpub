package api

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"github.com/newtosh/nitpub/internal/outbox"
)

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

func (h *Handler) ServeFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	posts, err := h.outbox.ListPosts()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	base := strings.TrimSuffix(h.outbox.BaseURL(), "/")
	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "nitpub",
			Link:        base + "/",
			Description: "Notes and articles from nitpub",
		},
	}
	for _, p := range posts {
		feed.Channel.Items = append(feed.Channel.Items, rssItem{
			Title:       feedTitle(p),
			Link:        h.outbox.Permalink(p),
			Description: feedDescription(p),
			PubDate:     p.CreatedAt.UTC().Format(time.RFC1123Z),
			GUID:        h.outbox.Permalink(p),
		})
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_, _ = w.Write([]byte(xml.Header))
	_ = enc.Encode(feed)
}

func feedTitle(p outbox.Post) string {
	if p.Kind == outbox.KindArticle {
		if line := firstLine(p.Content); line != "" {
			return line
		}
		return "Article"
	}
	content := strings.TrimSpace(p.Content)
	if len(content) > 80 {
		return content[:77] + "..."
	}
	if content == "" {
		return "Note"
	}
	return content
}

func feedDescription(p outbox.Post) string {
	if p.Kind == outbox.KindArticle {
		return articleBody(p.Content)
	}
	return strings.TrimSpace(p.Content)
}

func firstLine(content string) string {
	if i := strings.IndexByte(content, '\n'); i >= 0 {
		content = content[:i]
	}
	line := strings.TrimSpace(content)
	for strings.HasPrefix(line, "#") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
	}
	return line
}

func articleBody(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) < 2 {
		return ""
	}
	return strings.TrimSpace(lines[1])
}

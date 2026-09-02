package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/search"
	"github.com/newtosh/nitpub/internal/sitecontent"
	"github.com/newtosh/nitpub/internal/store"
)

func testSiteHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	siteRoot := filepath.Join(dir, "site")
	if err := os.MkdirAll(filepath.Join(siteRoot, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `
[home]
recent_count = 5

[[nav]]
label = "About"
path = "/about"
icon = "user"

[[pages]]
path = "/about"
type = "markdown"
file = "pages/about.md"
`
	if err := os.WriteFile(filepath.Join(siteRoot, "site.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteRoot, "pages/about.md"), []byte("# About\n\nHello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	siteSvc, err := sitecontent.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	idx := search.NewIndex()
	rebuild := func() {
		posts, err := ob.ListPosts()
		if err != nil {
			return
		}
		pages, err := siteSvc.MarkdownPagesForSearch()
		if err != nil {
			return
		}
		idx.Rebuild(posts, pages)
	}
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, siteSvc, idx, rebuild, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false, false)
	return h, sid
}

func TestServeSite(t *testing.T) {
	h, _ := testSiteHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/site", nil)
	rec := httptest.NewRecorder()
	h.ServeSite(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Nav []struct {
			Label string `json:"label"`
			Path  string `json:"path"`
			Icon  string `json:"icon"`
		} `json:"nav"`
		Home struct {
			RecentCount int `json:"recent_count"`
		} `json:"home"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Nav) != 1 || body.Nav[0].Icon != "user" {
		t.Fatalf("nav = %+v", body.Nav)
	}
	if body.Home.RecentCount != 5 {
		t.Fatalf("recent_count = %d", body.Home.RecentCount)
	}
}

func TestServeSiteAnalyticsFlag(t *testing.T) {
	h, _ := testSiteHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/site", nil)
	rec := httptest.NewRecorder()
	h.ServeSite(rec, req)
	var off struct {
		AnalyticsEnabled bool `json:"analytics_enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&off); err != nil {
		t.Fatal(err)
	}
	if off.AnalyticsEnabled {
		t.Fatal("analytics_enabled should be false by default")
	}

	h.analyticsEnabled = true
	req = httptest.NewRequest(http.MethodGet, "/api/site", nil)
	rec = httptest.NewRecorder()
	h.ServeSite(rec, req)
	var on struct {
		AnalyticsEnabled bool `json:"analytics_enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&on); err != nil {
		t.Fatal(err)
	}
	if !on.AnalyticsEnabled {
		t.Fatal("analytics_enabled should be true after enabling")
	}
}

func TestServeSiteQuotePostsFlag(t *testing.T) {
	h, _ := testSiteHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/site", nil)
	rec := httptest.NewRecorder()
	h.ServeSite(rec, req)
	var off struct {
		QuotePostsEnabled bool `json:"quote_posts_enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&off); err != nil {
		t.Fatal(err)
	}
	if off.QuotePostsEnabled {
		t.Fatal("quote_posts_enabled should be false by default")
	}

	h.quotePostsEnabled = true
	req = httptest.NewRequest(http.MethodGet, "/api/site", nil)
	rec = httptest.NewRecorder()
	h.ServeSite(rec, req)
	var on struct {
		QuotePostsEnabled bool `json:"quote_posts_enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&on); err != nil {
		t.Fatal(err)
	}
	if !on.QuotePostsEnabled {
		t.Fatal("quote_posts_enabled should be true after enabling")
	}
}

func TestServeSitePage(t *testing.T) {
	h, _ := testSiteHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/site/pages/about", nil)
	req.SetPathValue("path", "about")
	rec := httptest.NewRecorder()
	h.ServeSitePage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Type  string `json:"type"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Type != "markdown" || body.Title != "About" || body.Body == "" {
		t.Fatalf("%+v", body)
	}
}

func TestServeSitePageMissing(t *testing.T) {
	h, _ := testSiteHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/site/pages/nope", nil)
	req.SetPathValue("path", "nope")
	rec := httptest.NewRecorder()
	h.ServeSitePage(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestListPostsPaginated(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	for i := 0; i < 7; i++ {
		if _, _, err := ob.CreatePost(outbox.KindNote, "post"); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "user", nil, nil, "", false, false)

	req := httptest.NewRequest(http.MethodGet, "/api/posts?limit=5&offset=0", nil)
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var page struct {
		Posts []outbox.Post `json:"posts"`
		Total int           `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Posts) != 5 || page.Total != 7 {
		t.Fatalf("posts=%d total=%d", len(page.Posts), page.Total)
	}
}

func TestAdminPutManifestRequiresAuth(t *testing.T) {
	h, _ := testSiteHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/site/manifest", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	h.AdminPutManifest(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServeSearchEmptyQuery(t *testing.T) {
	h, _ := testSiteHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	rec := httptest.NewRecorder()
	h.ServeSearch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServeSearchMatchesPost(t *testing.T) {
	h, sid := testSiteHandler(t)
	create := bytes.NewBufferString(`{"kind":"note","content":"unique-findme-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", create)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.ServePosts(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d", rec.Code)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/search?q=findme", nil)
	searchRec := httptest.NewRecorder()
	h.ServeSearch(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", searchRec.Code, searchRec.Body.String())
	}
	var result struct {
		Results []struct {
			Type string `json:"type"`
		} `json:"results"`
	}
	if err := json.NewDecoder(searchRec.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected search hit")
	}
}

package sitecontent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	m, err := svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	if m.Archive.PageSize != 20 {
		t.Fatalf("page_size = %d", m.Archive.PageSize)
	}
}

func TestLoadMarkdownAndLinksPages(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(svc.Root(), "pages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc.Root(), "pages", "about.md"), []byte("# About\n\nHello."), 0o644); err != nil {
		t.Fatal(err)
	}
	links := []byte("title = \"Projects\"\n\n[[links]]\ntitle = \"nitpub\"\nurl = \"https://nitpub.com\"\nicon = \"globe\"\n")
	if err := os.WriteFile(filepath.Join(svc.Root(), "pages", "projects.links.toml"), links, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `[[pages]]
path = "/about"
type = "markdown"
file = "pages/about.md"

[[pages]]
path = "/projects"
type = "links"
file = "pages/projects.links.toml"
`
	if err := os.WriteFile(svc.ManifestPath(), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	about, err := svc.PageByPath("/about")
	if err != nil {
		t.Fatal(err)
	}
	if about.Title != "About" || about.Body == "" {
		t.Fatalf("about page: %+v", about)
	}
	projects, err := svc.PageByPath("/projects")
	if err != nil {
		t.Fatal(err)
	}
	if len(projects.Links) != 1 || projects.Links[0].Title != "nitpub" {
		t.Fatalf("projects: %+v", projects)
	}
}

func TestReservedPathRejected(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	manifest := `[[pages]]
path = "/api/evil"
type = "markdown"
file = "pages/x.md"
`
	if err := os.WriteFile(svc.ManifestPath(), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Load(); err == nil {
		t.Fatal("expected reserved path error")
	}
}

func TestWriteFileRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.WriteFile("../escape.md", []byte("nope")); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestWriteManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := Defaults()
	m.Home.RecentCount = 5
	m.Nav = []NavItem{{Label: "About", Path: "/about", Icon: "user"}}
	if err := svc.WriteManifest(m); err != nil {
		t.Fatal(err)
	}
	loaded, err := svc.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Home.RecentCount != 5 {
		t.Fatalf("recent_count = %d", loaded.Home.RecentCount)
	}
}

package sitecontent

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
)

// Service loads and writes site content files under data_dir/site.
type Service struct {
	root string
	mu   sync.RWMutex
}

// New creates the site content directory if needed.
func New(dataDir string) (*Service, error) {
	root := filepath.Join(dataDir, "site")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create site dir: %w", err)
	}
	return &Service{root: root}, nil
}

// Root returns the absolute site content directory.
func (s *Service) Root() string { return s.root }

// ManifestPath returns the path to site.toml.
func (s *Service) ManifestPath() string {
	return filepath.Join(s.root, "site.toml")
}

// Load reads site.toml and resolves page files. Missing manifest returns defaults.
func (s *Service) Load() (Manifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.ManifestPath())
	if err != nil {
		if os.IsNotExist(err) {
			return Defaults(), nil
		}
		return Manifest{}, err
	}
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse site.toml: %w", err)
	}
	if err := validateManifest(&m); err != nil {
		return Manifest{}, err
	}
	applyManifestDefaults(&m)
	return m, nil
}

func applyManifestDefaults(m *Manifest) {
	d := Defaults()
	if m.Archive.Mode == "" {
		m.Archive.Mode = d.Archive.Mode
	}
	if m.Archive.PageSize <= 0 {
		m.Archive.PageSize = d.Archive.PageSize
	}
	if m.Home.RecentCount < 0 {
		m.Home.RecentCount = 0
	}
	if m.Footer.Text == "" {
		m.Footer.Text = d.Footer.Text
	}
	if m.Footer.ShowGithubLink == nil {
		m.Footer.ShowGithubLink = d.Footer.ShowGithubLink
	}
	if m.Branding.Tagline == "" {
		m.Branding.Tagline = DefaultTagline
	}
}

func validateManifest(m *Manifest) error {
	seen := make(map[string]struct{})
	for i := range m.Nav {
		item := &m.Nav[i]
		p, err := normalizeNavPath(item.Path)
		if err != nil {
			return fmt.Errorf("nav[%d]: %w", i, err)
		}
		item.Path = p
		if err := validateIcon(item.Icon); err != nil {
			return fmt.Errorf("nav[%d]: %w", i, err)
		}
	}
	for i := range m.Pages {
		ref := &m.Pages[i]
		p, err := normalizePagePath(ref.Path)
		if err != nil {
			return fmt.Errorf("pages[%d]: %w", i, err)
		}
		ref.Path = p
		if ref.Type != "markdown" && ref.Type != "links" {
			return fmt.Errorf("pages[%d]: unknown type %q", i, ref.Type)
		}
		if _, err := safeRelFile(ref.File); err != nil {
			return fmt.Errorf("pages[%d]: %w", i, err)
		}
		if _, ok := seen[p]; ok {
			return fmt.Errorf("duplicate page path %q", p)
		}
		seen[p] = struct{}{}
	}
	return nil
}

// PageByPath resolves a custom page by URL path (e.g. /about).
func (s *Service) PageByPath(urlPath string) (*Page, error) {
	m, err := s.Load()
	if err != nil {
		return nil, err
	}
	urlPath = pathClean(urlPath)
	var ref *PageRef
	for i := range m.Pages {
		if m.Pages[i].Path == urlPath {
			ref = &m.Pages[i]
			break
		}
	}
	if ref == nil {
		return nil, fmt.Errorf("page not found")
	}
	return s.resolvePage(ref)
}

func pathClean(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return path.Clean(p)
}

func (s *Service) resolvePage(ref *PageRef) (*Page, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rel, err := safeRelFile(ref.File)
	if err != nil {
		return nil, err
	}
	full := filepath.Join(s.root, rel)
	if filepath.Dir(full) != filepath.Clean(filepath.Join(s.root, filepath.Dir(rel))) {
		return nil, fmt.Errorf("invalid file path")
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ref.File, err)
	}
	page := &Page{Path: ref.Path, Type: ref.Type, File: ref.File}
	switch ref.Type {
	case "markdown":
		page.Body = string(data)
		page.Title, page.Body = titleFromMarkdown(page.Body)
	case "links":
		var lf linksFile
		if err := toml.Unmarshal(data, &lf); err != nil {
			return nil, fmt.Errorf("parse links file: %w", err)
		}
		page.Title = lf.Title
		page.Links = lf.Links
		for i := range page.Links {
			if err := validateIcon(page.Links[i].Icon); err != nil {
				return nil, fmt.Errorf("links[%d]: %w", i, err)
			}
		}
	default:
		return nil, fmt.Errorf("unknown page type %q", ref.Type)
	}
	return page, nil
}

// titleFromMarkdown extracts a leading "# Title" line as the page title and
// strips it from the returned body — the caller (CustomPageView.vue) renders
// the title as its own <h1> above the markdown body, so leaving the source
// line in would render it a second time via the markdown itself.
func titleFromMarkdown(body string) (title string, rest string) {
	lines := strings.SplitN(body, "\n", 2)
	line := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(line, "# ") {
		return "", body
	}
	title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
	if len(lines) < 2 {
		return title, ""
	}
	return title, strings.TrimLeft(lines[1], "\n")
}

// WriteManifest validates and writes site.toml.
func (s *Service) WriteManifest(m Manifest) error {
	if err := validateManifest(&m); err != nil {
		return err
	}
	applyManifestDefaults(&m)
	data, err := toml.Marshal(m)
	if err != nil {
		return err
	}
	return s.writeFile("site.toml", data)
}

// WriteFile writes a file relative to the site root (e.g. pages/about.md).
func (s *Service) WriteFile(rel string, content []byte) error {
	rel, err := safeRelFile(rel)
	if err != nil {
		return err
	}
	return s.writeFile(rel, content)
}

func (s *Service) writeFile(rel string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	full := filepath.Join(s.root, rel)
	if filepath.Dir(full) != filepath.Clean(filepath.Join(s.root, filepath.Dir(rel))) {
		return fmt.Errorf("invalid file path")
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, full)
}

// ReadFile reads a file relative to the site root.
func (s *Service) ReadFile(rel string) ([]byte, error) {
	rel, err := safeRelFile(rel)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return os.ReadFile(filepath.Join(s.root, rel))
}

// ListPageFiles returns manifest page file paths and whether site.toml exists.
func (s *Service) ListPageFiles() ([]string, bool, error) {
	m, err := s.Load()
	if err != nil {
		return nil, false, err
	}
	_, err = os.Stat(s.ManifestPath())
	exists := err == nil
	var files []string
	for _, p := range m.Pages {
		files = append(files, p.File)
	}
	return files, exists, nil
}

// MarkdownPagesForSearch returns markdown page paths and bodies for indexing.
func (s *Service) MarkdownPagesForSearch() ([]Page, error) {
	m, err := s.Load()
	if err != nil {
		return nil, err
	}
	var out []Page
	for _, ref := range m.Pages {
		if ref.Type != "markdown" {
			continue
		}
		p, err := s.PageByPath(ref.Path)
		if err != nil {
			continue
		}
		out = append(out, *p)
	}
	return out, nil
}

package icons

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

// catalogUpstreamURL points at Phosphor's own icon metadata source (the
// same repo icons.go fetches SVGs from) — not a JSON API, a ~500KB
// TypeScript module. Every icon's {name, tags} pair is extracted with a
// regexp rather than a real TS parser: each icon is a flat, bracket-free
// object literal (no nested braces), so a non-greedy match from "name:" up
// to the next "}" can't run past its own icon's boundary into the next.
const catalogUpstreamURL = "https://raw.githubusercontent.com/phosphor-icons/core/main/src/icons.ts"

const maxCatalogBytes = 4 << 20 // 4 MiB — the real file is ~500KB

// CatalogEntry is one searchable icon: its shortcode name plus Phosphor's
// own tags (synonyms an author might actually type, e.g. "mail" for the
// icon named "envelope").
type CatalogEntry struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

var iconBlockRe = regexp.MustCompile(`name:\s*"([a-z][a-z0-9-]*)"[^}]*?tags:\s*\[([^\]]*)\][^}]*?\}`)
var tagRe = regexp.MustCompile(`"([^"]+)"`)

// Catalog returns the full searchable icon list, fetching and parsing
// Phosphor's metadata once and caching the result to disk forever after —
// same convention as Get for individual SVGs.
func (s *Service) Catalog(ctx context.Context) ([]CatalogEntry, error) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()

	if s.catalogCache != nil {
		return s.catalogCache, nil
	}

	path := s.catalogCachePath()
	if data, err := os.ReadFile(path); err == nil {
		var entries []CatalogEntry
		if err := json.Unmarshal(data, &entries); err == nil && len(entries) > 0 {
			s.catalogCache = entries
			return entries, nil
		}
	}

	entries, err := s.fetchCatalog(ctx)
	if err != nil {
		return nil, err
	}

	if data, err := json.Marshal(entries); err == nil {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o644)
	}
	s.catalogCache = entries
	return entries, nil
}

func (s *Service) catalogCachePath() string {
	return filepath.Join(filepath.Dir(s.dir), "icon-catalog.json")
}

func (s *Service) fetchCatalog(ctx context.Context) ([]CatalogEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogUpstreamURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch icon catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("icon catalog not found upstream (status %d)", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalogBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxCatalogBytes {
		return nil, fmt.Errorf("icon catalog response too large")
	}

	matches := iconBlockRe.FindAllSubmatch(data, -1)
	entries := make([]CatalogEntry, 0, len(matches))
	for _, m := range matches {
		name := string(m[1])
		var tags []string
		for _, t := range tagRe.FindAllSubmatch(m[2], -1) {
			tag := string(t[1])
			if tag == "*new*" {
				continue
			}
			tags = append(tags, tag)
		}
		entries = append(entries, CatalogEntry{Name: name, Tags: tags})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("icon catalog parse produced no entries")
	}
	return entries, nil
}

// Package icons serves Phosphor icons (https://phosphoricons.com, MIT) by
// name, for the ":icon-name:" markdown shortcode. Icons are fetched from
// Phosphor's own GitHub repo on first request for a given name and cached to
// disk forever after — nothing is bundled ahead of time, and a visitor's
// browser never talks to GitHub directly; only this server does, once per
// icon name, ever.
package icons

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	maxIconBytes = 256 << 10 // 256 KiB — real Phosphor icons are a few KB
	// defaultUpstreamBase points at Phosphor's own repo. Overridable via
	// Service.upstreamBase (unexported, test-only) so tests hit a local
	// fake server instead of the real internet.
	defaultUpstreamBase = "https://raw.githubusercontent.com/phosphor-icons/core/main/assets"
)

// weights are ordered longest-suffix-first isn't required since they're all
// checked as exact trailing segments, but keep "duotone" before shorter
// prefixless collisions is a non-issue — these are the only five Phosphor
// uses beyond the suffixless default (regular).
var weights = []string{"thin", "light", "bold", "fill", "duotone"}

// nameRe matches a single Phosphor icon's kebab-case name (the part between
// the colons in ":name:"), consistent with Phosphor's own file naming.
// Rejecting anything else here is what keeps a server-side fetch from ever
// being pointed somewhere unexpected — the only attacker-influenceable part
// of the upstream URL is this name.
var nameRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// Service caches fetched icon SVGs on disk under dataDir/icons.
type Service struct {
	dir          string
	client       *http.Client
	upstreamBase string

	catalogMu    sync.Mutex
	catalogCache []CatalogEntry
}

func New(dataDir string) (*Service, error) {
	dir := filepath.Join(dataDir, "icons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create icons dir: %w", err)
	}
	return &Service{
		dir:          dir,
		client:       &http.Client{Timeout: 10 * time.Second},
		upstreamBase: defaultUpstreamBase,
	}, nil
}

// splitName separates a full shortcode name ("cloud-lightning-fill") into
// its base icon name and weight, defaulting to "regular" (Phosphor's own
// suffixless default) when no recognized weight suffix is present.
// splitName rejects a base name that's entirely numeric — real Phosphor
// icons are never named e.g. "30", so this quietly avoids treating an
// incidental "12:30:45"-shaped timestamp in prose as an icon shortcode; the
// markdown rule that produces candidate names in the first place applies
// the same guard before ever reaching here (belt and suspenders, since this
// is also the point where an attacker-facing name gets validated before a
// server-side fetch).
func splitName(full string) (base, weight string, ok bool) {
	if !nameRe.MatchString(full) {
		return "", "", false
	}
	base, weight = full, "regular"
	for _, w := range weights {
		if suf := "-" + w; strings.HasSuffix(full, suf) && len(full) > len(suf) {
			base, weight = strings.TrimSuffix(full, suf), w
			break
		}
	}
	if isNumeric(base) {
		return "", "", false
	}
	return base, weight, true
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (s *Service) cachePath(base, weight string) string {
	return filepath.Join(s.dir, weight, base+".svg")
}

func (s *Service) upstreamURL(base, weight string) string {
	filename := base + ".svg"
	if weight != "regular" {
		filename = base + "-" + weight + ".svg"
	}
	return fmt.Sprintf("%s/%s/%s", s.upstreamBase, weight, filename)
}

// Get returns an icon's raw SVG bytes, fetching and caching it on first use.
func (s *Service) Get(ctx context.Context, name string) ([]byte, error) {
	base, weight, ok := splitName(name)
	if !ok {
		return nil, fmt.Errorf("invalid icon name %q", name)
	}

	path := s.cachePath(base, weight)
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.upstreamURL(base, weight), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch icon %q: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("icon %q not found upstream (status %d)", name, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIconBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIconBytes || !bytes.Contains(data, []byte("<svg")) {
		return nil, fmt.Errorf("unexpected response fetching icon %q", name)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create icon cache dir: %w", err)
	}
	// Best-effort: a write failure still returns the freshly fetched bytes
	// to this caller — it just means the next request fetches again.
	_ = os.WriteFile(path, data, 0o644)
	return data, nil
}

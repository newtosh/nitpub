// Package analytics proxies pageview stats from a self-hosted GoatCounter
// instance (goatcounter.com) server-side, so its API token never reaches
// the browser and nitpub's admin UI can render the data with its own
// styling instead of embedding GoatCounter's dashboard. See
// docs/plans/2026-08-23-001-feat-goatcounter-analytics-plan.md.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	maxResponseBytes = 1 << 20 // 1 MiB — generous for a stats JSON response
	cacheTTL         = 60 * time.Second
	fetchLimit       = 10               // top N pages / referrers
	fetchTimeout     = 15 * time.Second // overall budget for the 3 sequential upstream calls in Stats
)

// Window is a supported time span for the stats query. GoatCounter's
// stats/* endpoints accept start/end as RFC3339 timestamps; Window maps to
// a duration back from now.
type Window string

const (
	Window24h Window = "24h"
	Window7d  Window = "7d"
	Window30d Window = "30d"
)

// ParseWindow validates a caller-supplied window string, defaulting to
// Window7d for anything unrecognized (including empty).
func ParseWindow(raw string) Window {
	switch Window(raw) {
	case Window24h, Window7d, Window30d:
		return Window(raw)
	default:
		return Window7d
	}
}

func (w Window) duration() time.Duration {
	switch w {
	case Window24h:
		return 24 * time.Hour
	case Window30d:
		return 30 * 24 * time.Hour
	default: // Window7d and any unvalidated fallback
		return 7 * 24 * time.Hour
	}
}

// Stats is the reshaped view of GoatCounter's stats the admin UI renders.
type Stats struct {
	TotalPageviews int          `json:"total_pageviews"`
	DailyTotals    []DailyPoint `json:"daily_totals"`
	TopPages       []Breakdown  `json:"top_pages"`
	TopReferrers   []Breakdown  `json:"top_referrers"`
}

// DailyPoint is one day's pageview count within the queried window, in
// chronological order — the basis for the admin UI's trend sparkline.
type DailyPoint struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// Breakdown is one ranked row (a page path or a referrer) plus its count.
// Name is untrusted, visitor-influenced data (a Referer header or a
// requested path) — callers must render it as plain text, never as HTML
// or an unvalidated link href.
type Breakdown struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type cacheEntry struct {
	stats    Stats
	cachedAt time.Time
}

// Service fetches and caches GoatCounter stats, one cache entry per
// Window. Each Window has its own fetch lock (see windowLock), so a
// cache-miss fetch for one window never blocks a request for a
// different, already-cached window — only concurrent cache-miss callers
// for the *same* window serialize, collapsing into one upstream round
// trip instead of each independently firing 3 requests at GoatCounter's
// 4 req/s API limit.
type Service struct {
	baseURL  string
	apiToken string
	// vhost, when set, overrides the outgoing request's Host header to
	// match the GoatCounter site's own -vhost value (see internal/config's
	// AnalyticsVhost comment). GoatCounter resolves which site a request
	// is for by Host header, falling back to "the only site" only when
	// its instance has exactly one site total (zgo.at/goatcounter/v2's
	// handlers/mw.go, s.ByHost + single-site fallback) — so an empty
	// vhost silently relies on that fallback, which breaks the moment a
	// second site is added to the same GoatCounter instance for another
	// app on the same VPS.
	vhost  string
	client *http.Client
	ttl    time.Duration // unexported, test-overridable — see analytics_test.go
	now    func() time.Time

	mu    sync.Mutex // guards cache and locks only; never held during a fetch
	cache map[Window]cacheEntry
	locks map[Window]*sync.Mutex
}

func New(baseURL, apiToken, vhost string) *Service {
	return &Service{
		baseURL:  baseURL,
		apiToken: apiToken,
		vhost:    vhost,
		client:   &http.Client{Timeout: 10 * time.Second},
		ttl:      cacheTTL,
		now:      time.Now,
		cache:    make(map[Window]cacheEntry),
		locks:    make(map[Window]*sync.Mutex),
	}
}

// windowLock returns the fetch lock for window, creating it on first use.
func (s *Service) windowLock(window Window) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[window]
	if !ok {
		l = &sync.Mutex{}
		s.locks[window] = l
	}
	return l
}

func (s *Service) cachedStats(window Window) (Stats, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[window]
	if !ok || s.now().Sub(entry.cachedAt) >= s.ttl {
		return Stats{}, false
	}
	return entry.stats, true
}

// Stats returns the cached stats for window if fresh, otherwise fetches
// and reshapes them from GoatCounter. GoatCounter's exact response field
// names are mapped defensively (see fetchJSON callers below) — verify
// against a live instance before shipping, matching the plan's own
// Host-header verification note for this same API surface.
func (s *Service) Stats(ctx context.Context, window Window) (Stats, error) {
	if stats, ok := s.cachedStats(window); ok {
		return stats, nil
	}

	wl := s.windowLock(window)
	wl.Lock()
	defer wl.Unlock()

	// Double-checked: another goroutine may have populated the cache for
	// this window while we were waiting to acquire wl.
	if stats, ok := s.cachedStats(window); ok {
		return stats, nil
	}

	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	start := s.now().Add(-window.duration())
	params := map[string]string{"start": start.Format(time.RFC3339)}

	total, daily, err := s.fetchTotal(fetchCtx, params)
	if err != nil {
		return Stats{}, fmt.Errorf("fetch analytics total: %w", err)
	}
	pages, err := s.fetchPages(fetchCtx, params)
	if err != nil {
		return Stats{}, fmt.Errorf("fetch analytics top pages: %w", err)
	}
	refs, err := s.fetchBreakdown(fetchCtx, "/api/v0/stats/toprefs", params)
	if err != nil {
		return Stats{}, fmt.Errorf("fetch analytics top referrers: %w", err)
	}

	stats := Stats{TotalPageviews: total, DailyTotals: daily, TopPages: pages, TopReferrers: refs}

	s.mu.Lock()
	s.cache[window] = cacheEntry{stats: stats, cachedAt: s.now()}
	s.mu.Unlock()

	return stats, nil
}

func (s *Service) fetchTotal(ctx context.Context, params map[string]string) (int, []DailyPoint, error) {
	var body struct {
		Total int `json:"total"`
		Stats []struct {
			Day   string `json:"day"`
			Daily int    `json:"daily"`
		} `json:"stats"`
	}
	if err := s.getJSON(ctx, "/api/v0/stats/total", params, &body); err != nil {
		return 0, nil, err
	}
	daily := make([]DailyPoint, 0, len(body.Stats))
	for _, d := range body.Stats {
		daily = append(daily, DailyPoint{Day: d.Day, Count: d.Daily})
	}
	return body.Total, daily, nil
}

// fetchPages fetches GoatCounter's top-pages breakdown from
// /api/v0/stats/hits, whose response shape ({"hits":[{"path","count"}]})
// differs from the other stats/{page} endpoints ({"stats":[{"name","count"}]}
// — see fetchBreakdown). Decoding /api/v0/stats/hits with fetchBreakdown's
// shape silently ignores every row, since encoding/json drops unmatched
// keys instead of erroring.
func (s *Service) fetchPages(ctx context.Context, params map[string]string) ([]Breakdown, error) {
	var body struct {
		Hits []struct {
			Path  string `json:"path"`
			Count int    `json:"count"`
		} `json:"hits"`
	}
	all := map[string]string{"limit": fmt.Sprintf("%d", fetchLimit)}
	for k, v := range params {
		all[k] = v
	}
	if err := s.getJSON(ctx, "/api/v0/stats/hits", all, &body); err != nil {
		return nil, err
	}
	out := make([]Breakdown, 0, len(body.Hits))
	for _, row := range body.Hits {
		out = append(out, Breakdown{Name: row.Path, Count: row.Count})
	}
	return out, nil
}

func (s *Service) fetchBreakdown(ctx context.Context, path string, params map[string]string) ([]Breakdown, error) {
	var body struct {
		Stats []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"stats"`
	}
	all := map[string]string{"limit": fmt.Sprintf("%d", fetchLimit)}
	for k, v := range params {
		all[k] = v
	}
	if err := s.getJSON(ctx, path, all, &body); err != nil {
		return nil, err
	}
	out := make([]Breakdown, 0, len(body.Stats))
	for _, row := range body.Stats {
		out = append(out, Breakdown{Name: row.Name, Count: row.Count})
	}
	return out, nil
}

// getJSON performs an authenticated GET and decodes a bounded response
// body into dst. Errors never include the Authorization header or the
// raw response body — only the endpoint and status code — so a token
// never lands in a log via a wrapped error.
func (s *Service) getJSON(ctx context.Context, path string, params map[string]string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", path, err)
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+s.apiToken)
	if s.vhost != "" {
		req.Host = s.vhost
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: upstream unreachable", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request %s: upstream status %d", path, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Errorf("read response from %s: %w", path, err)
	}
	if len(data) > maxResponseBytes {
		return fmt.Errorf("response from %s exceeds size limit", path)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decode response from %s: %w", path, err)
	}
	return nil
}

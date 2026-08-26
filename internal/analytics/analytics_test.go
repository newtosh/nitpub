package analytics

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fakeGoatCounter(t *testing.T, wantToken string, requestCount *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(requestCount, 1)
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantToken {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer "+wantToken)
		}
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/api/v0/stats/total":
			body = `{"total": 42, "stats": [{"day": "2026-08-23", "daily": 18}, {"day": "2026-08-24", "daily": 24}]}`
		case "/api/v0/stats/hits":
			body = `{"hits": [{"path": "/about", "count": 10}]}`
		case "/api/v0/stats/toprefs":
			body = `{"stats": [{"name": "example.com", "count": 5}]}`
		case "/api/v0/stats/locations":
			body = `{"stats": [{"id": "US", "name": "United States", "count": 7}]}`
		default:
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
}

func TestStatsHappyPath(t *testing.T) {
	var reqs int32
	upstream := fakeGoatCounter(t, "test-token", &reqs)
	defer upstream.Close()

	svc := New(upstream.URL, "test-token", "")
	stats, err := svc.Stats(t.Context(), Window7d)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalPageviews != 42 {
		t.Fatalf("TotalPageviews = %d, want 42", stats.TotalPageviews)
	}
	if len(stats.TopPages) != 1 || stats.TopPages[0].Name != "/about" || stats.TopPages[0].Count != 10 {
		t.Fatalf("TopPages = %+v", stats.TopPages)
	}
	if len(stats.TopReferrers) != 1 || stats.TopReferrers[0].Name != "example.com" {
		t.Fatalf("TopReferrers = %+v", stats.TopReferrers)
	}
	if len(stats.TopLocations) != 1 || stats.TopLocations[0].Name != "United States" || stats.TopLocations[0].Code != "US" {
		t.Fatalf("TopLocations = %+v", stats.TopLocations)
	}
	wantDaily := []DailyPoint{{Day: "2026-08-23", Count: 18}, {Day: "2026-08-24", Count: 24}}
	if len(stats.DailyTotals) != len(wantDaily) || stats.DailyTotals[0] != wantDaily[0] || stats.DailyTotals[1] != wantDaily[1] {
		t.Fatalf("DailyTotals = %+v, want %+v", stats.DailyTotals, wantDaily)
	}
	// Host-header assumption check: the request above succeeded against the
	// fake server without setting an explicit Host header override. This
	// only proves the client doesn't need one for a plain httptest server —
	// it does not prove GoatCounter's real vhost-matching behaves the same
	// for token-authenticated calls. Verify against a live instance before
	// shipping (see the plan's Assumptions section and Definition of Done).
}

func TestVhostOverridesHostHeader(t *testing.T) {
	var gotHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/api/v0/stats/total":
			body = `{"total": 1}`
		default:
			body = `{}`
		}
		_, _ = w.Write([]byte(body))
	}))
	defer upstream.Close()

	svc := New(upstream.URL, "tok", "nitpub-internal")
	if _, err := svc.Stats(t.Context(), Window7d); err != nil {
		t.Fatal(err)
	}
	if gotHost != "nitpub-internal" {
		t.Fatalf("Host header = %q, want %q", gotHost, "nitpub-internal")
	}
}

func TestStatsAuthHeader(t *testing.T) {
	var reqs int32
	upstream := fakeGoatCounter(t, "expected-token", &reqs)
	defer upstream.Close()

	svc := New(upstream.URL, "expected-token", "")
	if _, err := svc.Stats(t.Context(), Window7d); err != nil {
		t.Fatal(err)
	}
	// fakeGoatCounter's handler asserts the header itself via t.Errorf.
}

func TestStatsCaching(t *testing.T) {
	var reqs int32
	upstream := fakeGoatCounter(t, "tok", &reqs)
	defer upstream.Close()

	svc := New(upstream.URL, "tok", "")
	svc.ttl = 50 * time.Millisecond

	if _, err := svc.Stats(t.Context(), Window7d); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Stats(t.Context(), Window7d); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&reqs); got != 4 {
		t.Fatalf("requests after 2 calls within TTL = %d, want 4 (one round of 4 endpoints)", got)
	}

	time.Sleep(60 * time.Millisecond)
	if _, err := svc.Stats(t.Context(), Window7d); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&reqs); got != 8 {
		t.Fatalf("requests after TTL expiry = %d, want 8", got)
	}
}

func TestStatsErrorPathDoesNotPoisonCache(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	svc := New(upstream.URL, "tok", "")
	if _, err := svc.Stats(t.Context(), Window7d); err == nil {
		t.Fatal("expected error from non-2xx upstream")
	}
	if _, ok := svc.cache[Window7d]; ok {
		t.Fatal("cache should remain unset after an error")
	}
}

func TestParseWindow(t *testing.T) {
	cases := map[string]Window{
		"24h": Window24h,
		"7d":  Window7d,
		"30d": Window30d,
		"":    Window7d,
		"bad": Window7d,
	}
	for raw, want := range cases {
		if got := ParseWindow(raw); got != want {
			t.Errorf("ParseWindow(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestStatsCachesPerWindow(t *testing.T) {
	var reqs int32
	upstream := fakeGoatCounter(t, "tok", &reqs)
	defer upstream.Close()

	svc := New(upstream.URL, "tok", "")
	if _, err := svc.Stats(t.Context(), Window24h); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Stats(t.Context(), Window30d); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&reqs); got != 8 {
		t.Fatalf("requests for 2 distinct windows = %d, want 8 (4 each, no cross-window cache hit)", got)
	}
	// Re-request the first window — should hit its own cache, not refetch.
	if _, err := svc.Stats(t.Context(), Window24h); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&reqs); got != 8 {
		t.Fatalf("requests after re-fetching cached window = %d, want still 8", got)
	}
}

// TestStatsConcurrentSameWindowSingleFetch proves the Service doc comment's
// central claim: concurrent cache-miss callers for the same window collapse
// into one upstream round trip instead of each independently firing 4
// requests at GoatCounter's 4 req/s API limit.
func TestStatsConcurrentSameWindowSingleFetch(t *testing.T) {
	var reqs int32
	upstream := fakeGoatCounter(t, "tok", &reqs)
	defer upstream.Close()

	svc := New(upstream.URL, "tok", "")

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := svc.Stats(t.Context(), Window7d); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&reqs); got != 4 {
		t.Fatalf("requests after %d concurrent same-window calls = %d, want 4 (one round of 4 endpoints)", n, got)
	}
}

func TestStatsUnreachable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := upstream.URL
	upstream.Close() // closed before use: connection refused

	svc := New(url, "tok", "")
	svc.client.Timeout = 500 * time.Millisecond
	start := time.Now()
	_, err := svc.Stats(t.Context(), Window7d)
	if err == nil {
		t.Fatal("expected error for unreachable upstream")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("Stats did not respect the configured client timeout")
	}
}

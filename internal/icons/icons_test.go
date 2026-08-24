package icons

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitName(t *testing.T) {
	cases := []struct {
		in       string
		wantBase string
		wantWt   string
		wantOK   bool
	}{
		{"heart", "heart", "regular", true},
		{"heart-fill", "heart", "fill", true},
		{"cloud-lightning", "cloud-lightning", "regular", true},
		{"cloud-lightning-bold", "cloud-lightning", "bold", true},
		{"cloud-lightning-duotone", "cloud-lightning", "duotone", true},
		{"30", "", "", false},         // purely numeric — never a real icon name
		{"", "", "", false},           // empty
		{"-fill", "", "", false},      // weight suffix with no base
		{"Heart", "", "", false},      // uppercase not allowed
		{"heart_fill", "", "", false}, // underscore not allowed
		{"../../etc", "", "", false},  // path traversal shaped
		{"heart-", "", "", false},     // trailing hyphen
	}
	for _, c := range cases {
		base, weight, ok := splitName(c.in)
		if ok != c.wantOK || (ok && (base != c.wantBase || weight != c.wantWt)) {
			t.Errorf("splitName(%q) = (%q, %q, %v), want (%q, %q, %v)", c.in, base, weight, ok, c.wantBase, c.wantWt, c.wantOK)
		}
	}
}

func TestGetFetchesAndCaches(t *testing.T) {
	fetches := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches++
		if r.URL.Path != "/regular/heart.svg" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg viewBox="0 0 256 256"><path d="M1 1"/></svg>`))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc.upstreamBase = upstream.URL

	data, err := svc.Get(context.Background(), "heart")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<svg") {
		t.Fatalf("expected svg content, got %q", data)
	}
	if fetches != 1 {
		t.Fatalf("expected 1 upstream fetch, got %d", fetches)
	}

	// Second call must hit the on-disk cache, not fetch again.
	if _, err := svc.Get(context.Background(), "heart"); err != nil {
		t.Fatal(err)
	}
	if fetches != 1 {
		t.Fatalf("expected cache hit on second Get, fetches = %d", fetches)
	}

	cachePath := filepath.Join(dir, "icons", "regular", "heart.svg")
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cached file at %s: %v", cachePath, err)
	}
}

func TestGetRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(context.Background(), "../../etc/passwd"); err == nil {
		t.Fatal("expected error for path-traversal-shaped name")
	}
}

func TestGetUpstreamMissing(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc.upstreamBase = upstream.URL

	if _, err := svc.Get(context.Background(), "not-a-real-icon"); err == nil {
		t.Fatal("expected error for a name Phosphor doesn't have")
	}
}

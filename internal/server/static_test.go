package server

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

//go:embed testdata
var testStatic embed.FS

func TestSPAHandlerFallback(t *testing.T) {
	sub, err := fs.Sub(testStatic, "testdata")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(sub, "p/test-uuid"); err == nil {
		t.Fatal("expected stat miss for p/test-uuid")
	}
	h := spaHandler(sub, nil, "https://example.test", "https://example.test/actor", "", "")

	req := httptest.NewRequest(http.MethodGet, "/p/test-uuid", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, location = %q", rec.Code, rec.Header().Get("Location"))
	}
	if !contains(rec.Body.String(), "<html") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestSPAHandlerThemeInjection(t *testing.T) {
	sub, err := fs.Sub(testStatic, "testdata")
	if err != nil {
		t.Fatal(err)
	}
	resolver := func() (string, error) { return "nord", nil }
	h := spaHandler(sub, resolver, "https://example.test", "https://example.test/actor", "", "")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !contains(body, `data-theme="nord"`) {
		t.Fatalf("body = %q", body)
	}
}

func TestInjectGoatCounterBeacon(t *testing.T) {
	html := []byte(`<!doctype html><html><head><title>nitpub</title></head><body></body></html>`)
	out := injectGoatCounterBeacon(html, "https://stats.example.test/")
	want := `data-goatcounter="https://stats.example.test/count"`
	if !bytes.Contains(out, []byte(want)) {
		t.Fatalf("missing data-goatcounter: %s", out)
	}
	if !bytes.Contains(out, []byte(`src="https://stats.example.test/count.js"`)) {
		t.Fatalf("missing count.js src: %s", out)
	}
	// Idempotent: a second pass must not duplicate the snippet.
	again := injectGoatCounterBeacon(out, "https://stats.example.test")
	if bytes.Count(again, []byte(`data-goatcounter=`)) != 1 {
		t.Fatalf("expected one beacon, got: %s", again)
	}
}

func TestSPAHandlerGoatCounterInjection(t *testing.T) {
	sub, err := fs.Sub(testStatic, "testdata")
	if err != nil {
		t.Fatal(err)
	}
	h := spaHandler(sub, nil, "https://example.test", "https://example.test/actor", "", "https://stats.example.test")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !contains(rec.Body.String(), `data-goatcounter="https://stats.example.test/count"`) {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestInjectFediverseVerification(t *testing.T) {
	html := []byte(`<!doctype html><html><head><title>nitpub</title></head><body></body></html>`)
	out := injectFediverseVerification(html, "https://nitpub.com", "https://nitpub.com/actor")
	if !bytes.Contains(out, []byte(`<link rel="me" href="https://nitpub.com">`)) {
		t.Fatalf("missing site rel=me: %s", out)
	}
	if !bytes.Contains(out, []byte(`<link rel="me" href="https://nitpub.com/actor">`)) {
		t.Fatalf("missing actor rel=me: %s", out)
	}
}

func TestInjectThemeHTML(t *testing.T) {
	html := []byte(`<!doctype html><html lang="en"><body></body></html>`)
	out := injectThemeHTML(html, "github")
	if !bytes.Contains(out, []byte(`data-theme="github"`)) {
		t.Fatalf("out = %s", out)
	}
	plain := []byte(`<!DOCTYPE html><html><body>nitpub</body></html>`)
	outPlain := injectThemeHTML(plain, "nord")
	if !bytes.Contains(outPlain, []byte(`data-theme="nord"`)) {
		t.Fatalf("out = %s", outPlain)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

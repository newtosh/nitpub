package server

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLegacyEnrollRedirect(t *testing.T) {
	mux := http.NewServeMux()
	registerLegacyRedirects(mux)

	req := httptest.NewRequest(http.MethodGet, "/admin/enroll?token=abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/author/enroll?token=abc" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestLegacyEditRedirect(t *testing.T) {
	mux := http.NewServeMux()
	registerLegacyRedirects(mux)

	req := httptest.NewRequest(http.MethodGet, "/admin/edit/my-slug", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/author/edit/my-slug" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestAuthorEditSPAFallback(t *testing.T) {
	sub, err := fs.Sub(testStatic, "testdata")
	if err != nil {
		t.Fatal(err)
	}
	h := spaHandler(sub, nil, "", "", "", "")

	req := httptest.NewRequest(http.MethodGet, "/author/edit/foo", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !contains(rec.Body.String(), "<html") {
		t.Fatalf("expected index.html body")
	}
}

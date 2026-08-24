package server

import (
	"context"
	"testing"

	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
)

// TestNewRegistersRoutesWithoutPanic confirms every route pattern registered
// in New() — including the moderation routes' {id}/{actor...} wildcards —
// is a valid Go 1.22 ServeMux pattern. An invalid pattern panics at
// registration time; no other test previously called server.New() directly.
func TestNewRegistersRoutesWithoutPanic(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cfg := config.Config{
		Domain:  "example.test",
		Port:    8080,
		DataDir: dir,
		Actor:   "nit",
		Secret:  "test-secret-test-secret-32-bytes",
		BaseURL: "http://example.test",
	}

	srv, err := New(context.Background(), cfg, st, nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer srv.Close()
	_ = srv.Handler()
}

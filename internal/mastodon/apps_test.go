package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/newtosh/nitpub/internal/store"
)

func testAppStore(t *testing.T) *AppStore {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewAppStore(st)
}

func TestRegisterOrGetAppCachesOnSecondCall(t *testing.T) {
	calls := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]string{"client_id": "cid", "client_secret": "csecret"})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	as := testAppStore(t)
	reg := NewAppRegistrar(c, as)

	domain := testDomain(t, srv)
	first, err := reg.RegisterOrGetApp(context.Background(), domain, "https://nitpub.example/comment/callback")
	if err != nil {
		t.Fatal(err)
	}
	second, err := reg.RegisterOrGetApp(context.Background(), domain, "https://nitpub.example/comment/callback")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 HTTP call, got %d", calls)
	}
	if first.ClientID != second.ClientID {
		t.Fatalf("expected cached registration to match: %+v vs %+v", first, second)
	}
}

func TestReregisterOverwritesCacheWithFallbackScope(t *testing.T) {
	var lastScope string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		lastScope = r.FormValue("scopes")
		_ = json.NewEncoder(w).Encode(map[string]string{"client_id": "cid2", "client_secret": "csecret2"})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	as := testAppStore(t)
	reg := NewAppRegistrar(c, as)

	domain := testDomain(t, srv)
	if _, err := reg.RegisterOrGetApp(context.Background(), domain, "https://nitpub.example/comment/callback"); err != nil {
		t.Fatal(err)
	}
	updated, err := reg.Reregister(context.Background(), domain, "https://nitpub.example/comment/callback")
	if err != nil {
		t.Fatal(err)
	}
	if lastScope != FallbackScope {
		t.Fatalf("expected fallback scope requested, got %q", lastScope)
	}
	cached, err := reg.RegisterOrGetApp(context.Background(), domain, "https://nitpub.example/comment/callback")
	if err != nil {
		t.Fatal(err)
	}
	if cached.ClientID != updated.ClientID || cached.Scope != FallbackScope {
		t.Fatalf("expected cache overwritten with fallback-scope registration: %+v", cached)
	}
}

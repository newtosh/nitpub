package mastodon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testDomain(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func TestRegisterApp(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/apps" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("scopes") != DefaultScope {
			t.Fatalf("unexpected scope: %s", r.FormValue("scopes"))
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"client_id": "cid", "client_secret": "csecret"})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	reg, err := c.RegisterApp(context.Background(), testDomain(t, srv), "https://nitpub.example/comment/callback", DefaultScope)
	if err != nil {
		t.Fatal(err)
	}
	if reg.ClientID != "cid" || reg.ClientSecret != "csecret" {
		t.Fatalf("unexpected registration: %+v", reg)
	}
}

func TestRegisterAppNon2xx(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	if _, err := c.RegisterApp(context.Background(), testDomain(t, srv), "https://x/callback", DefaultScope); err == nil {
		t.Fatal("expected error on non-2xx response")
	}
}

func TestExchangeToken(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	tok, err := c.ExchangeToken(context.Background(), testDomain(t, srv), "cid", "csecret", "code", "https://nitpub.example/comment/callback")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "tok" {
		t.Fatalf("unexpected token: %s", tok)
	}
}

func TestExchangeTokenFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	if _, err := c.ExchangeToken(context.Background(), testDomain(t, srv), "cid", "csecret", "bad-code", "https://x/callback"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveStatusFound(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("missing bearer token: %s", r.Header.Get("Authorization"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"statuses": []map[string]string{{"id": "555", "url": "https://nitpub.example/posts/abc"}},
		})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	id, err := c.ResolveStatus(context.Background(), testDomain(t, srv), "tok", "https://nitpub.example/posts/abc")
	if err != nil {
		t.Fatal(err)
	}
	if id != "555" {
		t.Fatalf("unexpected status id: %s", id)
	}
}

func TestResolveStatusNoExactMatchFails(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Search returns a result, but for a different URL/URI than
		// queried — must not be treated as a match (would thread the
		// reply onto an unrelated status).
		_ = json.NewEncoder(w).Encode(map[string]any{
			"statuses": []map[string]string{{"id": "999", "url": "https://nitpub.example/posts/unrelated"}},
		})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	_, err := c.ResolveStatus(context.Background(), testDomain(t, srv), "tok", "https://nitpub.example/posts/abc")
	if err != ErrStatusNotFound {
		t.Fatalf("expected ErrStatusNotFound on a non-exact match, got %v", err)
	}
}

func TestResolveStatusNotFound(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"statuses": []map[string]string{}})
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	_, err := c.ResolveStatus(context.Background(), testDomain(t, srv), "tok", "https://nitpub.example/posts/missing")
	if err != ErrStatusNotFound {
		t.Fatalf("expected ErrStatusNotFound, got %v", err)
	}
}

func TestResolveStatusScopeRejected(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	_, err := c.ResolveStatus(context.Background(), testDomain(t, srv), "tok", "https://nitpub.example/posts/x")
	if err != ErrScopeRejected {
		t.Fatalf("expected ErrScopeRejected, got %v", err)
	}
}

func TestPostReply(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/statuses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = r.ParseForm()
		if r.FormValue("in_reply_to_id") != "555" {
			t.Fatalf("missing in_reply_to_id: %s", r.FormValue("in_reply_to_id"))
		}
		if !strings.Contains(r.FormValue("status"), "great post") {
			t.Fatalf("missing comment text: %s", r.FormValue("status"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	if err := c.PostReply(context.Background(), testDomain(t, srv), "tok", "555", "great post!"); err != nil {
		t.Fatal(err)
	}
}

func TestPostReplyFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"validation failed"}`))
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	if err := c.PostReply(context.Background(), testDomain(t, srv), "tok", "555", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRevokeTokenBestEffortDoesNotPanicOnFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	c.RevokeTokenBestEffort(context.Background(), testDomain(t, srv), "cid", "csecret", "tok")
}

func TestRevokeToken(t *testing.T) {
	called := false
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/oauth/revoke" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClientWithHTTP(srv.Client())
	if err := c.RevokeToken(context.Background(), testDomain(t, srv), "cid", "csecret", "tok"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected revoke endpoint to be called")
	}
}

package bluesky

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClientWithHTTP(srv.URL, srv.Client())
}

func TestCreateSession_Success(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body struct {
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Identifier != "alice.bsky.social" || body.Password != "app-pass" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"did":        "did:plc:abc123",
			"handle":     "alice.bsky.social",
			"accessJwt":  "access-token",
			"refreshJwt": "refresh-token",
		})
	})

	session, err := c.CreateSession(context.Background(), "alice.bsky.social", "app-pass")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.DID == "" || session.AccessJWT == "" || session.RefreshJWT == "" {
		t.Fatalf("expected all session fields populated, got %+v", session)
	}
}

func TestCreateSession_InvalidAppPassword(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "AuthenticationRequired",
			"message": "Invalid identifier or password",
		})
	})

	_, err := c.CreateSession(context.Background(), "alice.bsky.social", "wrong-pass")
	if err == nil {
		t.Fatal("expected error for invalid app password, got nil")
	}
	if !IsUnauthorized(err) {
		t.Fatalf("expected IsUnauthorized(err) to be true, got err=%v", err)
	}
}

func TestRefreshSession(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.refreshSession" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer old-refresh-token" {
			t.Fatalf("unexpected Authorization header: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"did":        "did:plc:abc123",
			"handle":     "alice.bsky.social",
			"accessJwt":  "new-access-token",
			"refreshJwt": "new-refresh-token",
		})
	})

	session, err := c.RefreshSession(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("RefreshSession: %v", err)
	}
	if session.AccessJWT != "new-access-token" {
		t.Fatalf("expected new access token, got %q", session.AccessJWT)
	}
}

func TestUploadBlob(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.repo.uploadBlob" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("unexpected Content-Type: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("unexpected Authorization header: %s", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"blob": map[string]interface{}{
				"$type":    "blob",
				"ref":      map[string]string{"$link": "bafyreiabc"},
				"mimeType": "image/jpeg",
				"size":     4,
			},
		})
	})

	blob, err := c.UploadBlob(context.Background(), "access-token", []byte("data"), "image/jpeg")
	if err != nil {
		t.Fatalf("UploadBlob: %v", err)
	}
	if blob.Ref.Link == "" || blob.MimeType != "image/jpeg" {
		t.Fatalf("expected usable blob ref, got %+v", blob)
	}
}

func TestCreateRecord(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.repo.createRecord" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body struct {
			Repo       string `json:"repo"`
			Collection string `json:"collection"`
			Record     struct {
				Type string `json:"$type"`
				Text string `json:"text"`
			} `json:"record"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Repo != "did:plc:abc123" || body.Collection != "app.bsky.feed.post" {
			t.Fatalf("unexpected request body: %+v", body)
		}
		if body.Record.Text != "hello world" {
			t.Fatalf("unexpected record text: %q", body.Record.Text)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"uri": "at://did:plc:abc123/app.bsky.feed.post/xyz",
			"cid": "bafyreicid123",
		})
	})

	uri, cid, err := c.CreateRecord(context.Background(), "access-token", "did:plc:abc123", PostRecord{Text: "hello world"})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}
	if uri == "" || cid == "" {
		t.Fatalf("expected non-empty uri/cid, got uri=%q cid=%q", uri, cid)
	}
}

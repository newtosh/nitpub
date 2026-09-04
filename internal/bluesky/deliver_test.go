package bluesky

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/newtosh/nitpub/internal/linkpreview"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func newDeliverTestSvc(t *testing.T) (*outbox.Service, *AuthStore) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	svc := outbox.New(st, "https://nit.pub", "https://nit.pub/actor")
	return svc, NewAuthStore(st)
}

func seedAuth(t *testing.T, authStore *AuthStore, refreshJWT string) {
	t.Helper()
	if err := authStore.Put(Auth{DID: "did:plc:test", Handle: "alice.bsky.social", RefreshJWT: refreshJWT}); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func refreshSessionOK(w http.ResponseWriter) {
	writeJSON(w, map[string]string{
		"did": "did:plc:test", "handle": "alice.bsky.social",
		"accessJwt": "access-1", "refreshJwt": "refresh-2",
	})
}

// stubImageServer serves fixed image bytes over httptest — the "fake image
// server" the test scenarios call for, so no real network call is made.
func stubImageServer(t *testing.T, body []byte, contentType string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// stubImageFetch bypasses the SSRF guard for the duration of the test so
// fetchImageBytes can reach an httptest.Server on 127.0.0.1 — the guard
// itself belongs to (and is exercised by) the real-network code path, not
// delivery orchestration.
func stubImageFetch(t *testing.T) {
	t.Helper()
	orig := fetchImageBytes
	fetchImageBytes = func(ctx context.Context, rawURL string) ([]byte, string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, "", err
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, "", err
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("image fetch status %d", res.StatusCode)
		}
		data, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, "", err
		}
		return data, res.Header.Get("Content-Type"), nil
	}
	t.Cleanup(func() { fetchImageBytes = orig })
}

func stubLinkPreview(t *testing.T, preview linkpreview.Preview, err error) {
	t.Helper()
	orig := fetchLinkPreview
	fetchLinkPreview = func(string) (linkpreview.Preview, error) { return preview, err }
	t.Cleanup(func() { fetchLinkPreview = orig })
}

func TestDeliver_ExternalEmbedFromLinkPreview(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")
	stubLinkPreview(t, linkpreview.Preview{Title: "A Great Page", Description: "desc"}, nil)

	var createCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.createRecord":
			atomic.AddInt32(&createCount, 1)
			var body struct {
				Record PostRecord `json:"record"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			raw, _ := json.Marshal(body.Record.Embed)
			var embed externalEmbed
			_ = json.Unmarshal(raw, &embed)
			if embed.Type != "app.bsky.embed.external" || embed.External.Title != "A Great Page" {
				t.Fatalf("unexpected embed: %+v", embed)
			}
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/1", "cid": "cid1"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	post, _, err := svc.CreatePost(outbox.KindNote, "hello there, no images or links here")
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if createCount != 1 {
		t.Fatalf("expected 1 createRecord call, got %d", createCount)
	}

	got, err := svc.GetPost(outbox.PostSlug(post.ID))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bluesky == nil || got.Bluesky.Status != "posted" || got.Bluesky.URI == "" {
		t.Fatalf("expected posted state, got %+v", got.Bluesky)
	}
}

// TestDeliver_FacetsWiredToRecord confirms BuildPostText's link facets
// actually reach PostRecord.Facets in the request Bluesky receives — a link
// with no facet renders as inert plain text, not a tappable link.
func TestDeliver_FacetsWiredToRecord(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")
	stubLinkPreview(t, linkpreview.Preview{Title: "A Great Page"}, nil)

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.createRecord":
			var body struct {
				Record PostRecord `json:"record"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Record.Facets) != 1 {
				t.Fatalf("expected 1 facet on the record, got %d: %+v", len(body.Record.Facets), body.Record.Facets)
			}
			if body.Record.Facets[0].Features[0].URI != "https://example.com/x" {
				t.Fatalf("unexpected facet URI: %+v", body.Record.Facets[0])
			}
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/1", "cid": "cid1"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	post, _, err := svc.CreatePost(outbox.KindNote, "check this out: [a page](https://example.com/x)")
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
}

func TestDeliver_ImageEmbed(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")
	stubImageFetch(t)
	imgURL := stubImageServer(t, []byte("fake-jpeg-bytes"), "image/jpeg")

	var uploadCount, createCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.uploadBlob":
			atomic.AddInt32(&uploadCount, 1)
			writeJSON(w, map[string]interface{}{"blob": map[string]interface{}{
				"$type": "blob", "ref": map[string]string{"$link": "bafyimg"},
				"mimeType": "image/jpeg", "size": 15,
			}})
		case "/xrpc/com.atproto.repo.createRecord":
			atomic.AddInt32(&createCount, 1)
			var body struct {
				Record PostRecord `json:"record"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			raw, _ := json.Marshal(body.Record.Embed)
			var embed imageEmbed
			_ = json.Unmarshal(raw, &embed)
			if embed.Type != "app.bsky.embed.images" || len(embed.Images) != 1 || embed.Images[0].Alt != "a cat" {
				t.Fatalf("unexpected image embed: %+v", embed)
			}
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/2", "cid": "cid2"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	content := fmt.Sprintf("look at this ![a cat](%s)", imgURL)
	post, _, err := svc.CreatePost(outbox.KindNote, content)
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if uploadCount != 1 || createCount != 1 {
		t.Fatalf("expected 1 upload and 1 create, got upload=%d create=%d", uploadCount, createCount)
	}

	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky == nil || got.Bluesky.Status != "posted" || got.Bluesky.Truncated {
		t.Fatalf("expected posted, not truncated, got %+v", got.Bluesky)
	}
}

func TestDeliver_CapsAtFourImagesAndTruncates(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")
	stubImageFetch(t)
	imgURL := stubImageServer(t, []byte("bytes"), "image/png")

	var uploadCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.uploadBlob":
			atomic.AddInt32(&uploadCount, 1)
			writeJSON(w, map[string]interface{}{"blob": map[string]interface{}{
				"$type": "blob", "ref": map[string]string{"$link": "bafyimg"},
				"mimeType": "image/png", "size": 5,
			}})
		case "/xrpc/com.atproto.repo.createRecord":
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/3", "cid": "cid3"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	content := ""
	for i := 0; i < 5; i++ {
		content += fmt.Sprintf("![img%d](%s) ", i, imgURL)
	}
	post, _, err := svc.CreatePost(outbox.KindNote, content)
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if uploadCount != 4 {
		t.Fatalf("expected exactly 4 uploads (capped), got %d", uploadCount)
	}

	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky == nil || got.Bluesky.Status != "posted" || !got.Bluesky.Truncated {
		t.Fatalf("expected posted+truncated, got %+v", got.Bluesky)
	}
}

func TestDeliver_LinkPreviewFailureStillDelivers(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")
	stubLinkPreview(t, linkpreview.Preview{}, fmt.Errorf("fetch failed"))

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.createRecord":
			var body struct {
				Record PostRecord `json:"record"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			raw, _ := json.Marshal(body.Record.Embed)
			var embed externalEmbed
			_ = json.Unmarshal(raw, &embed)
			if embed.External.Title != "" {
				t.Fatalf("expected bare-uri embed with no title, got %+v", embed)
			}
			if embed.External.URI == "" {
				t.Fatalf("expected bare uri to still be set, got %+v", embed)
			}
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/4", "cid": "cid4"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	post, _, err := svc.CreatePost(outbox.KindNote, "a note with no images")
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky == nil || got.Bluesky.Status != "posted" {
		t.Fatalf("expected posted despite linkpreview failure, got %+v", got.Bluesky)
	}
}

func TestDeliver_ImageUploadFailureSurfacesAsError(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")
	stubImageFetch(t)
	imgURL := stubImageServer(t, []byte("bytes"), "image/png")

	var createCalled bool
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.uploadBlob":
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(w, map[string]string{"error": "InternalServerError", "message": "boom"})
		case "/xrpc/com.atproto.repo.createRecord":
			createCalled = true
			writeJSON(w, map[string]string{"uri": "at://x", "cid": "y"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	content := fmt.Sprintf("![alt](%s)", imgURL)
	post, _, err := svc.CreatePost(outbox.KindNote, content)
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err == nil {
		t.Fatal("expected Deliver to return an error")
	}
	if createCalled {
		t.Fatal("expected createRecord never to be called after an upload failure")
	}

	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky == nil || got.Bluesky.Status != "error" || got.Bluesky.Error == "" {
		t.Fatalf("expected error state, got %+v", got.Bluesky)
	}
}

func TestDeliver_UnauthorizedTriggersOneRefreshAndRetry(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")

	var refreshCount, createCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			atomic.AddInt32(&refreshCount, 1)
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.createRecord":
			n := atomic.AddInt32(&createCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				writeJSON(w, map[string]string{"error": "ExpiredToken", "message": "token expired"})
				return
			}
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/5", "cid": "cid5"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	post, _, err := svc.CreatePost(outbox.KindNote, "a note with no images")
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if refreshCount != 2 {
		t.Fatalf("expected exactly 2 refreshSession calls (initial + retry), got %d", refreshCount)
	}
	if createCount != 2 {
		t.Fatalf("expected exactly 2 createRecord calls (fail + retry), got %d", createCount)
	}
	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky == nil || got.Bluesky.Status != "posted" {
		t.Fatalf("expected posted after retry, got %+v", got.Bluesky)
	}
}

func TestDeliver_RefreshSessionFailureInvalidatesConnection(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xrpc/com.atproto.server.refreshSession" {
			w.WriteHeader(http.StatusUnauthorized)
			writeJSON(w, map[string]string{"error": "ExpiredToken", "message": "refresh token expired"})
			return
		}
		t.Fatalf("unexpected path %s", r.URL.Path)
	})

	post, _, err := svc.CreatePost(outbox.KindNote, "a note")
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err == nil {
		t.Fatal("expected Deliver to return an error")
	}

	auth, err := authStore.Get()
	if err != nil {
		t.Fatal(err)
	}
	if auth == nil || !auth.Invalid {
		t.Fatalf("expected stored auth marked invalid, got %+v", auth)
	}

	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky != nil {
		t.Fatalf("expected no per-post bluesky state written on a refresh failure, got %+v", got.Bluesky)
	}
}

func TestDeliver_TooLongTriggersOneRetryWithHarderBudget(t *testing.T) {
	svc, authStore := newDeliverTestSvc(t)
	seedAuth(t, authStore, "refresh-1")

	var createCount int32
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.refreshSession":
			refreshSessionOK(w)
		case "/xrpc/com.atproto.repo.createRecord":
			n := atomic.AddInt32(&createCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]string{"error": "InvalidRequest", "message": "text too long: 301 graphemes"})
				return
			}
			writeJSON(w, map[string]string{"uri": "at://did:plc:test/app.bsky.feed.post/6", "cid": "cid6"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	})

	post, _, err := svc.CreatePost(outbox.KindNote, "a note with no images")
	if err != nil {
		t.Fatal(err)
	}
	if err := Deliver(context.Background(), client, authStore, svc, post); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if createCount != 2 {
		t.Fatalf("expected exactly 2 createRecord calls (too-long + retry), got %d", createCount)
	}
	got, _ := svc.GetPost(outbox.PostSlug(post.ID))
	if got.Bluesky == nil || got.Bluesky.Status != "posted" {
		t.Fatalf("expected posted after too-long retry, got %+v", got.Bluesky)
	}
}

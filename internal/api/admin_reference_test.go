package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/newtosh/nitpub/internal/mastodon"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/sitecontent"
	"github.com/newtosh/nitpub/internal/store"
)

// newFakeReferenceInstance stands in for a reference Mastodon instance:
// app registration, token exchange, and search-resolve. Its search handler
// mirrors real Mastodon behavior (verified against a live instance): the
// returned status's "url" just echoes back the queried origin URL, it does
// NOT return a mastodon.social-hosted page — ResolvePermalink instead
// builds that from the account handle + local id this fake also returns.
func newFakeReferenceInstance(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"client_id": "cid", "client_secret": "csecret"})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "reftok"})
	})
	mux.HandleFunc("/api/v2/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"statuses": []map[string]any{{
				"id":      "999",
				"url":     q,
				"uri":     q,
				"account": map[string]string{"acct": "mockuser"},
			}},
		})
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func domainOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

// testReferenceHandler builds a Handler with a real outbox + site config
// and the reference-connect flow wired against client, auth pre-verified
// (same pattern as admin_federation_test.go's testAuth).
func testReferenceHandler(t *testing.T, client *mastodon.Client) (*Handler, *outbox.Service, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	authSvc, sid := testAuth(t, st)
	siteSvc, err := sitecontent.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(ob, authSvc, nil, siteSvc, nil, nil, nil, nil, nil, nil,
		"example.test", "http://example.test", "nit", nil, nil, "", false)

	referenceApps := mastodon.NewAppRegistrar(client, mastodon.NewAppStoreIn(st, store.BucketReferenceApps))
	h.SetReference(client, referenceApps, mastodon.NewReferenceAuthStore(st), true)
	return h, ob, sid
}

// runReferenceConnect drives AdminStartReferenceConnect then
// AdminReferenceCallback, following the redirect_url/state exactly as a
// real browser round-trip would, and returns the callback's response.
func runReferenceConnect(t *testing.T, h *Handler, sid string) *httptest.ResponseRecorder {
	t.Helper()
	startReq := httptest.NewRequest(http.MethodPost, "/api/admin/federation/reference/connect", nil)
	startReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	startRec := httptest.NewRecorder()
	h.AdminStartReferenceConnect(startRec, startReq)
	if startRec.Code != http.StatusOK {
		t.Fatalf("connect status = %d body = %s", startRec.Code, startRec.Body.String())
	}
	var resp startReferenceConnectResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	redirect, err := url.Parse(resp.RedirectURL)
	if err != nil {
		t.Fatal(err)
	}
	state := redirect.Query().Get("state")

	cbReq := httptest.NewRequest(http.MethodGet, "/api/admin/federation/reference/callback?code=authcode&state="+state, nil)
	cbRec := httptest.NewRecorder()
	h.AdminReferenceCallback(cbRec, cbReq)
	return cbRec
}

func TestReferenceConnectAndResolvePermalink(t *testing.T) {
	inst := newFakeReferenceInstance(t)
	remoteURL := "https://" + domainOf(t, inst) + "/@mockuser/999"
	client := mastodon.NewClientWithHTTP(inst.Client())
	h, ob, sid := testReferenceHandler(t, client)

	// Site config's reference instance must point at the fake server, not
	// the default mastodon.social.
	manifest, err := h.site.Load()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Federation.ReferenceInstance = domainOf(t, inst)
	if err := h.site.WriteManifest(manifest); err != nil {
		t.Fatal(err)
	}

	cbRec := runReferenceConnect(t, h, sid)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback status = %d body = %s", cbRec.Code, cbRec.Body.String())
	}
	loc := cbRec.Header().Get("Location")
	if loc != "/admin/federation?reference=connected" {
		t.Fatalf("callback redirect = %q", loc)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/admin/federation/reference/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	statusRec := httptest.NewRecorder()
	h.AdminGetReferenceStatus(statusRec, statusReq)
	var status referenceStatusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Connected || status.Instance != domainOf(t, inst) {
		t.Fatalf("status = %+v", status)
	}

	// A post shared before the instance was connected — AdminResolveReferencePermalinks
	// should retroactively resolve it.
	post, _, err := ob.CreatePost(outbox.KindNote, "hello fediverse")
	if err != nil {
		t.Fatal(err)
	}
	slug := outbox.PostSlug(post.ID)
	if _, err := ob.SetFederation(slug, outbox.FederationState{Shared: true}); err != nil {
		t.Fatal(err)
	}

	resolveReq := httptest.NewRequest(http.MethodPost, "/api/admin/federation/reference/resolve", nil)
	resolveReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	resolveRec := httptest.NewRecorder()
	h.AdminResolveReferencePermalinks(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d body = %s", resolveRec.Code, resolveRec.Body.String())
	}
	var result outbox.BackfillResult
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 || result.Skipped != 0 {
		t.Fatalf("result = %+v", result)
	}

	updated, err := ob.GetPublishedPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Federation == nil || updated.Federation.RemoteURL != remoteURL {
		t.Fatalf("federation = %+v", updated.Federation)
	}
}

// TestNewShareResolvesRemotePermalinkAsync covers the primary path (not
// the backfill endpoint above): a post shared through the normal
// create-and-federate flow should pick up its remote permalink on its own,
// no admin action needed, once a reference instance is connected.
func TestNewShareResolvesRemotePermalinkAsync(t *testing.T) {
	inst := newFakeReferenceInstance(t)
	remoteURL := "https://" + domainOf(t, inst) + "/@mockuser/999"
	client := mastodon.NewClientWithHTTP(inst.Client())
	h, ob, sid := testReferenceHandler(t, client)

	manifest, err := h.site.Load()
	if err != nil {
		t.Fatal(err)
	}
	manifest.Federation.ReferenceInstance = domainOf(t, inst)
	if err := h.site.WriteManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if cbRec := runReferenceConnect(t, h, sid); cbRec.Code != http.StatusFound {
		t.Fatalf("connect callback status = %d", cbRec.Code)
	}

	post, create, err := ob.CreatePost(outbox.KindNote, "hello fediverse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.federationPublisher().Complete(post, create, true); err != nil {
		t.Fatal(err)
	}

	slug := outbox.PostSlug(post.ID)
	deadline := time.Now().Add(2 * time.Second)
	for {
		updated, err := ob.GetPublishedPost(slug)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Federation != nil && updated.Federation.RemoteURL == remoteURL {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("federation = %+v after deadline", updated.Federation)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestReferenceConnectRequiresAuth(t *testing.T) {
	inst := newFakeReferenceInstance(t)
	client := mastodon.NewClientWithHTTP(inst.Client())
	h, _, _ := testReferenceHandler(t, client)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/federation/reference/connect", nil)
	rec := httptest.NewRecorder()
	h.AdminStartReferenceConnect(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newtosh/nitpub/internal/bluesky"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/sitecontent"
	"github.com/newtosh/nitpub/internal/store"
)

// newFakeBlueskyPDS stands in for a Bluesky PDS's createSession endpoint:
// it accepts "app-pass" as the only valid app password for any handle and
// rejects everything else with a 401, mirroring the real
// com.atproto.server.createSession contract closely enough for these
// handler tests.
func newFakeBlueskyPDS(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/com.atproto.server.createSession", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Password != "app-pass" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "AuthenticationRequired", "message": "invalid password"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"did":        "did:plc:testuser",
			"handle":     req.Identifier,
			"accessJwt":  "access-token",
			"refreshJwt": "refresh-token",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testBlueskyHandler(t *testing.T, client *bluesky.Client) (*Handler, string) {
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
		"example.test", "http://example.test", "nit", nil, nil, "", false, false)
	h.SetBluesky(client, bluesky.NewAuthStore(st))
	return h, sid
}

func TestConnectBlueskyPersistsHandleDIDRefreshJWT(t *testing.T) {
	srv := newFakeBlueskyPDS(t)
	client := bluesky.NewClient(srv.URL)
	h, sid := testBlueskyHandler(t, client)

	body := strings.NewReader(`{"handle":"alice.bsky.social","app_password":"app-pass"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/bluesky/connect", body)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminConnectBluesky(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("connect status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "app-pass") {
		t.Fatalf("response body leaked app_password: %s", rec.Body.String())
	}

	auth, err := h.blueskyAuth.Get()
	if err != nil {
		t.Fatal(err)
	}
	if auth == nil {
		t.Fatal("expected auth to be persisted")
	}
	if auth.Handle != "alice.bsky.social" || auth.DID != "did:plc:testuser" || auth.RefreshJWT != "refresh-token" {
		t.Fatalf("unexpected stored auth: %+v", auth)
	}
}

func TestConnectBlueskyInvalidAppPasswordPersistsNothing(t *testing.T) {
	srv := newFakeBlueskyPDS(t)
	client := bluesky.NewClient(srv.URL)
	h, sid := testBlueskyHandler(t, client)

	body := strings.NewReader(`{"handle":"alice.bsky.social","app_password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/bluesky/connect", body)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminConnectBluesky(rec, req)

	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("expected 4xx status, got %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "wrong") {
		t.Fatalf("response body leaked app_password: %s", rec.Body.String())
	}

	auth, err := h.blueskyAuth.Get()
	if err != nil {
		t.Fatal(err)
	}
	if auth != nil {
		t.Fatalf("expected nothing persisted on failed connect, got %+v", auth)
	}
}

func TestDisconnectBlueskyClearsAuthAndStatusReflectsIt(t *testing.T) {
	srv := newFakeBlueskyPDS(t)
	client := bluesky.NewClient(srv.URL)
	h, sid := testBlueskyHandler(t, client)

	// Status before connect: disconnected.
	statusReq := httptest.NewRequest(http.MethodGet, "/api/admin/bluesky/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	statusRec := httptest.NewRecorder()
	h.AdminGetBlueskyStatus(statusRec, statusReq)
	var status blueskyStatusResponse
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Connected {
		t.Fatalf("expected disconnected before connect, got %+v", status)
	}

	// Connect.
	connectBody := strings.NewReader(`{"handle":"alice.bsky.social","app_password":"app-pass"}`)
	connectReq := httptest.NewRequest(http.MethodPost, "/api/admin/bluesky/connect", connectBody)
	connectReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	connectRec := httptest.NewRecorder()
	h.AdminConnectBluesky(connectRec, connectReq)
	if connectRec.Code != http.StatusOK {
		t.Fatalf("connect status = %d body = %s", connectRec.Code, connectRec.Body.String())
	}

	// Status after connect: connected.
	statusReq2 := httptest.NewRequest(http.MethodGet, "/api/admin/bluesky/status", nil)
	statusReq2.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	statusRec2 := httptest.NewRecorder()
	h.AdminGetBlueskyStatus(statusRec2, statusReq2)
	var status2 blueskyStatusResponse
	if err := json.Unmarshal(statusRec2.Body.Bytes(), &status2); err != nil {
		t.Fatal(err)
	}
	if !status2.Connected || status2.Handle != "alice.bsky.social" || status2.NeedsReconnect {
		t.Fatalf("unexpected status after connect: %+v", status2)
	}

	// Disconnect.
	disconnectReq := httptest.NewRequest(http.MethodDelete, "/api/admin/bluesky/connect", nil)
	disconnectReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	disconnectRec := httptest.NewRecorder()
	h.AdminDisconnectBluesky(disconnectRec, disconnectReq)
	if disconnectRec.Code != http.StatusNoContent {
		t.Fatalf("disconnect status = %d body = %s", disconnectRec.Code, disconnectRec.Body.String())
	}

	// Status after disconnect: disconnected again.
	statusReq3 := httptest.NewRequest(http.MethodGet, "/api/admin/bluesky/status", nil)
	statusReq3.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	statusRec3 := httptest.NewRecorder()
	h.AdminGetBlueskyStatus(statusRec3, statusReq3)
	var status3 blueskyStatusResponse
	if err := json.Unmarshal(statusRec3.Body.Bytes(), &status3); err != nil {
		t.Fatal(err)
	}
	if status3.Connected {
		t.Fatalf("expected disconnected after disconnect, got %+v", status3)
	}
}

func TestConnectBlueskyRequiresAuth(t *testing.T) {
	srv := newFakeBlueskyPDS(t)
	client := bluesky.NewClient(srv.URL)
	h, _ := testBlueskyHandler(t, client)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/bluesky/connect", strings.NewReader(`{"handle":"a","app_password":"app-pass"}`))
	rec := httptest.NewRecorder()
	h.AdminConnectBluesky(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", rec.Code)
	}
}

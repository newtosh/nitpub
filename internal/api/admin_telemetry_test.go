package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func newTelemetryTestHandler(t *testing.T, st *store.Store, auth *Auth) *Handler {
	t.Helper()
	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	return NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)
}

func TestAdminGetTelemetryStatusRequiresAuth(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	h := newTelemetryTestHandler(t, st, testAuthUnconfigured(t, st))
	h.SetTelemetry(st, "http://register.test")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/telemetry", nil)
	rec := httptest.NewRecorder()
	h.AdminGetTelemetryStatus(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAdminGetTelemetryStatusUnavailable(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	auth, sid := testAuth(t, st)
	h := newTelemetryTestHandler(t, st, auth)
	// No SetTelemetry call: h.telemetryStore stays nil.

	req := httptest.NewRequest(http.MethodGet, "/api/admin/telemetry", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetTelemetryStatus(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when telemetry unwired", rec.Code)
	}
}

func TestAdminGetTelemetryStatusDefaultsDisabled(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	auth, sid := testAuth(t, st)
	h := newTelemetryTestHandler(t, st, auth)
	h.SetTelemetry(st, "http://register.test")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/telemetry", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetTelemetryStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp adminTelemetryStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Enabled {
		t.Fatal("expected enabled=false by default")
	}
}

func TestAdminSetTelemetryEnabledRegistersOnFirstEnable(t *testing.T) {
	var registerHit int
	registerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		registerHit++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "issued-token"})
	}))
	defer registerSrv.Close()

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	auth, sid := testAuth(t, st)
	h := newTelemetryTestHandler(t, st, auth)
	h.SetTelemetry(st, registerSrv.URL)

	body, _ := json.Marshal(adminTelemetrySetRequest{Enabled: true})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/telemetry", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminSetTelemetryEnabled(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if registerHit != 1 {
		t.Fatalf("register called %d times, want 1", registerHit)
	}

	enabled, err := st.TelemetryEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("expected telemetry enabled after successful registration")
	}

	// Second enable call: identity already exists, must not re-register.
	req2 := httptest.NewRequest(http.MethodPost, "/api/admin/telemetry", bytes.NewReader(body))
	req2.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec2 := httptest.NewRecorder()
	h.AdminSetTelemetryEnabled(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d", rec2.Code)
	}
	if registerHit != 1 {
		t.Fatalf("register called %d times on re-enable, want still 1", registerHit)
	}
}

func TestAdminSetTelemetryEnabledRegistrationFailureLeavesDisabled(t *testing.T) {
	registerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer registerSrv.Close()

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	auth, sid := testAuth(t, st)
	h := newTelemetryTestHandler(t, st, auth)
	h.SetTelemetry(st, registerSrv.URL)

	body, _ := json.Marshal(adminTelemetrySetRequest{Enabled: true})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/telemetry", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminSetTelemetryEnabled(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("expected non-200 on registration failure")
	}

	enabled, err := st.TelemetryEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("expected telemetry to stay disabled after registration failure")
	}
}

func TestAdminSetTelemetryEnabledFalseStopsFutureExports(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetTelemetryEnabled(true); err != nil {
		t.Fatal(err)
	}

	auth, sid := testAuth(t, st)
	h := newTelemetryTestHandler(t, st, auth)
	h.SetTelemetry(st, "http://register.test")

	body, _ := json.Marshal(adminTelemetrySetRequest{Enabled: false})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/telemetry", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminSetTelemetryEnabled(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	enabled, err := st.TelemetryEnabled()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("expected telemetry disabled after POST enabled=false")
	}
}

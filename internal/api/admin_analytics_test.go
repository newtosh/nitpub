package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newtosh/nitpub/internal/analytics"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func TestAdminGetAnalyticsRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", true)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAdminGetAnalyticsDisabled(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	// analyticsEnabled=false and no SetAnalytics call: h.analytics stays nil.
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when analytics disabled", rec.Code)
	}
}

func TestAdminGetAnalyticsEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v0/stats/total":
			_, _ = w.Write([]byte(`{"total": 7}`))
		default:
			_, _ = w.Write([]byte(`{"stats": []}`))
		}
	}))
	defer upstream.Close()

	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", true)
	h.SetAnalytics(analytics.New(upstream.URL, "tok", ""))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var stats analytics.Stats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.TotalPageviews != 7 {
		t.Fatalf("TotalPageviews = %d, want 7", stats.TotalPageviews)
	}
}

func TestAdminGetAnalyticsIncludesGoatCounterURL(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total": 1, "stats": []}`))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", true)
	h.SetAnalytics(analytics.New(upstream.URL, "tok", ""))
	h.SetAnalyticsPublicURL("https://stats.example.test")

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		GoatCounterURL string `json:"goatcounter_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.GoatCounterURL != "https://stats.example.test" {
		t.Fatalf("goatcounter_url = %q, want %q", resp.GoatCounterURL, "https://stats.example.test")
	}
}

func TestAdminGetAnalyticsOmitsGoatCounterURLWhenUnset(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total": 1, "stats": []}`))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", true)
	h.SetAnalytics(analytics.New(upstream.URL, "tok", ""))
	// SetAnalyticsPublicURL deliberately not called: no link configured.

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		GoatCounterURL string `json:"goatcounter_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.GoatCounterURL != "" {
		t.Fatalf("goatcounter_url = %q, want empty", resp.GoatCounterURL)
	}
}

func TestAdminGetAnalyticsWindowParam(t *testing.T) {
	var gotStart string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/stats/total" {
			gotStart = r.URL.Query().Get("start")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total": 1, "stats": []}`))
	}))
	defer upstream.Close()

	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", true)
	h.SetAnalytics(analytics.New(upstream.URL, "tok", ""))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics?window=24h", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if gotStart == "" {
		t.Fatal("expected a start= query param to reach GoatCounter")
	}
}

func TestAdminGetAnalyticsUpstreamError(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", true)
	h.SetAnalytics(analytics.New(upstream.URL, "tok", ""))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/analytics", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetAnalytics(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 when upstream errors", rec.Code)
	}
}

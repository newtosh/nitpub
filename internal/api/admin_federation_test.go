package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

func TestAdminGetFederationRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation", nil)
	rec := httptest.NewRecorder()
	h.AdminGetFederation(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAdminGetFederation(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminGetFederation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp federationInfoResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Actor != "nit" || resp.Domain != "example.test" {
		t.Fatalf("actor/domain = %+v", resp)
	}
	if resp.Acct != "acct:nit@example.test" {
		t.Fatalf("acct = %q", resp.Acct)
	}
	if resp.ActorURL != "http://example.test/actor" {
		t.Fatalf("actor_url = %q", resp.ActorURL)
	}
}

func TestAdminFederationDeliveriesRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	h := NewHandler(ob, testAuthUnconfigured(t, st), nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation/deliveries", nil)
	rec := httptest.NewRecorder()
	h.AdminFederationDeliveries(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestAdminFederationDeliveriesEmpty(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation/deliveries", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminFederationDeliveries(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Deliveries []federationDeliveryRow `json:"deliveries"`
		Total      int                     `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 || len(resp.Deliveries) != 0 {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestAdminFederationDeliveriesExcludesDraft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	if _, err := ob.SaveDraft(outbox.KindNote, "", "a draft, never published", ""); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation/deliveries", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminFederationDeliveries(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Deliveries []federationDeliveryRow `json:"deliveries"`
		Total      int                     `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 || len(resp.Deliveries) != 0 {
		t.Fatalf("expected a draft to be excluded from the delivery log, got %+v", resp)
	}
}

func TestAdminFederationDeliveriesClassifiesStatus(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	if _, _, err := ob.CreatePost(outbox.KindNote, "never attempted"); err != nil {
		t.Fatal(err)
	}
	deliveredPost, _, err := ob.CreatePost(outbox.KindNote, "delivered")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ob.SetFederation(outbox.PostSlug(deliveredPost.ID), outbox.FederationState{Shared: true}); err != nil {
		t.Fatal(err)
	}
	errorPost, _, err := ob.CreatePost(outbox.KindNote, "errored")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ob.SetFederation(outbox.PostSlug(errorPost.ID), outbox.FederationState{Shared: true, Error: "connection refused"}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation/deliveries", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminFederationDeliveries(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Deliveries []federationDeliveryRow `json:"deliveries"`
		Total      int                     `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 || len(resp.Deliveries) != 3 {
		t.Fatalf("resp = %+v", resp)
	}
	byStatus := map[string]int{}
	var errMsg string
	for _, row := range resp.Deliveries {
		byStatus[row.Status]++
		if row.Status == "error" {
			errMsg = row.Error
		}
		if row.Kind != "note" {
			t.Fatalf("kind = %q", row.Kind)
		}
	}
	if byStatus["pending"] != 1 || byStatus["delivered"] != 1 || byStatus["error"] != 1 {
		t.Fatalf("byStatus = %+v", byStatus)
	}
	if errMsg != "connection refused" {
		t.Fatalf("errMsg = %q", errMsg)
	}
}

func TestAdminFederationDeliveriesRespectsPagination(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ob := outbox.New(st, "http://example.test", "http://example.test/actor")
	auth, sid := testAuth(t, st)
	h := NewHandler(ob, auth, nil, nil, nil, nil, nil, nil, nil, nil, "example.test", "http://example.test", "nit", nil, nil, "", false)

	for i := 0; i < 3; i++ {
		if _, _, err := ob.CreatePost(outbox.KindNote, "post"); err != nil {
			t.Fatal(err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/federation/deliveries?limit=1&offset=0", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	h.AdminFederationDeliveries(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Deliveries []federationDeliveryRow `json:"deliveries"`
		Total      int                     `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 3 {
		t.Fatalf("total = %d, want 3", resp.Total)
	}
	if len(resp.Deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(resp.Deliveries))
	}
}

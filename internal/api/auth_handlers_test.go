package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/newtosh/nitpub/internal/auth"
	"github.com/newtosh/nitpub/internal/store"
)

func TestLoginAndSession(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "pw", false)

	body := bytes.NewBufferString(`{"username":"admin","password":"pw"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	rec := httptest.NewRecorder()
	a.Login(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("login status = %d body=%s", rec.Code, rec.Body.String())
	}
	c := rec.Result().Cookies()
	var sid string
	for _, ck := range c {
		if ck.Name == auth.SessionCookie {
			sid = ck.Value
		}
	}
	if sid == "" {
		t.Fatal("expected session cookie")
	}

	sreq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sreq.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	srec := httptest.NewRecorder()
	a.Session(srec, sreq)
	var resp struct {
		Authenticated bool `json:"authenticated"`
	}
	if err := json.NewDecoder(srec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Authenticated {
		t.Fatal("expected authenticated")
	}
}

func TestAuthCheckAuthenticatedAndNot(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	a, sid := testAuth(t, st)

	// No session cookie: forward_auth must see a non-2xx status to block.
	unauthedReq := httptest.NewRequest(http.MethodGet, "/api/admin/authcheck", nil)
	unauthedRec := httptest.NewRecorder()
	a.AuthCheck(unauthedRec, unauthedReq)
	if unauthedRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", unauthedRec.Code)
	}

	authedReq := httptest.NewRequest(http.MethodGet, "/api/admin/authcheck", nil)
	authedReq.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	authedRec := httptest.NewRecorder()
	a.AuthCheck(authedRec, authedReq)
	if authedRec.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d, want 204", authedRec.Code)
	}
}

func TestLoginBadPassword(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "pw", false)

	body := bytes.NewBufferString(`{"username":"admin","password":"wrong"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	rec := httptest.NewRecorder()
	a.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	a.Logout(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if a.Authenticated(req) {
		t.Fatal("expected logged out")
	}
}

func TestAppearanceDefault(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "pw", false)

	req := httptest.NewRequest(http.MethodGet, "/api/appearance", nil)
	rec := httptest.NewRecorder()
	a.Appearance(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ThemeID string `json:"theme_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.ThemeID != auth.DefaultThemeID {
		t.Fatalf("theme_id = %q want %q", resp.ThemeID, auth.DefaultThemeID)
	}
}

func TestUpdateSettingsTheme(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	body := bytes.NewBufferString(`{"theme_id":"tokyo-night"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", body)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	a.UpdateSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var settings auth.AdminSettings
	if err := json.NewDecoder(rec.Body).Decode(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.ThemeID != "tokyo-night" {
		t.Fatalf("theme_id = %q want tokyo-night", settings.ThemeID)
	}

	areq := httptest.NewRequest(http.MethodGet, "/api/appearance", nil)
	arec := httptest.NewRecorder()
	a.Appearance(arec, areq)
	var appearance struct {
		ThemeID string `json:"theme_id"`
	}
	if err := json.NewDecoder(arec.Body).Decode(&appearance); err != nil {
		t.Fatal(err)
	}
	if appearance.ThemeID != "tokyo-night" {
		t.Fatalf("appearance theme_id = %q want tokyo-night", appearance.ThemeID)
	}
}

func TestUpdateSettingsThemeUnauthorized(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "pw", false)

	body := bytes.NewBufferString(`{"theme_id":"ocean"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", body)
	rec := httptest.NewRecorder()
	a.UpdateSettings(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUpdateSettingsInvalidTheme(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	body := bytes.NewBufferString(`{"theme_id":"neon"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/settings", body)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: sid})
	rec := httptest.NewRecorder()
	a.UpdateSettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

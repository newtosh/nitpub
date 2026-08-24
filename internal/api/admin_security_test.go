package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/newtosh/nitpub/internal/store"
	"github.com/pquerna/otp/totp"
)

func withSession(req *http.Request, sid string) *http.Request {
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: sid})
	return req
}

func encodeJSON(t *testing.T, v any) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

// --- Change password (U2) ---

func TestChangePasswordHappyPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/password", encodeJSON(t, changePasswordRequest{
		CurrentPassword: "secret",
		NewPassword:     "new-secret",
	})), sid)
	rec := httptest.NewRecorder()
	a.ChangePassword(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "new-secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login with new password status = %d", loginRec.Code)
	}
}

// Covers AE1.
func TestChangePasswordWrongCurrentPassword(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/password", encodeJSON(t, changePasswordRequest{
		CurrentPassword: "wrong",
		NewPassword:     "new-secret",
	})), sid)
	rec := httptest.NewRecorder()
	a.ChangePassword(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}

	// Old password still works — stored password unchanged.
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("login with old password status = %d", loginRec.Code)
	}
}

func TestChangePasswordRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "secret", false)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/security/password", encodeJSON(t, changePasswordRequest{CurrentPassword: "secret", NewPassword: "x"}))
	rec := httptest.NewRecorder()
	a.ChangePassword(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- TOTP (U3) ---

func totpEnable(t *testing.T, a *Auth, sid string) totpEnableResponse {
	t.Helper()
	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/enable", encodeJSON(t, totpEnableRequest{CurrentPassword: "secret"})), sid)
	rec := httptest.NewRecorder()
	a.EnableTOTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp totpEnableResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestTOTPEnableConfirmHappyPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	enabled := totpEnable(t, a, sid)
	if enabled.Secret == "" {
		t.Fatal("expected secret in enable response")
	}
	code, err := totp.GenerateCode(enabled.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	confirmReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/confirm", encodeJSON(t, totpConfirmRequest{
		CurrentPassword: "secret",
		Code:            code,
	})), sid)
	confirmRec := httptest.NewRecorder()
	a.ConfirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusNoContent {
		t.Fatalf("confirm status = %d body=%s", confirmRec.Code, confirmRec.Body.String())
	}

	// Login now requires 2FA.
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 2FA-required status 200, got %d", loginRec.Code)
	}
}

// Covers AE2.
func TestTOTPConfirmInvalidCodeRollsBack(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	totpEnable(t, a, sid)

	confirmReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/confirm", encodeJSON(t, totpConfirmRequest{
		CurrentPassword: "secret",
		Code:            "000000",
	})), sid)
	confirmRec := httptest.NewRecorder()
	a.ConfirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusBadRequest {
		t.Fatalf("confirm status = %d", confirmRec.Code)
	}

	// Rollback fired — login no longer demands a code.
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("expected no-2FA login after rollback, got %d", loginRec.Code)
	}
}

// Proves the client-supplied secret is ignored — confirm succeeds against
// the server-held secret even if a caller tries to supply a different one.
func TestTOTPConfirmUsesServerSecretNotClientSupplied(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	enabled := totpEnable(t, a, sid)
	code, err := totp.GenerateCode(enabled.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	// The confirm request shape has no secret field at all — it cannot be
	// tampered with, by construction. Confirm with the right code succeeds.
	confirmReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/confirm", encodeJSON(t, totpConfirmRequest{
		CurrentPassword: "secret",
		Code:            code,
	})), sid)
	confirmRec := httptest.NewRecorder()
	a.ConfirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusNoContent {
		t.Fatalf("confirm status = %d body=%s", confirmRec.Code, confirmRec.Body.String())
	}
}

func TestTOTPDisable(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	enabled := totpEnable(t, a, sid)
	code, err := totp.GenerateCode(enabled.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	confirmReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/confirm", encodeJSON(t, totpConfirmRequest{CurrentPassword: "secret", Code: code})), sid)
	confirmRec := httptest.NewRecorder()
	a.ConfirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusNoContent {
		t.Fatalf("confirm status = %d", confirmRec.Code)
	}

	disableReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/disable", encodeJSON(t, totpDisableRequest{CurrentPassword: "secret"})), sid)
	disableRec := httptest.NewRecorder()
	a.DisableTOTP(disableRec, disableReq)
	if disableRec.Code != http.StatusNoContent {
		t.Fatalf("disable status = %d", disableRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("expected no-2FA login after disable, got %d", loginRec.Code)
	}
}

func TestTOTPCleanupOnAbandon(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	enabled := totpEnable(t, a, sid)

	cleanupReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/cleanup", encodeJSON(t, totpCleanupRequest{Secret: enabled.Secret})), sid)
	cleanupRec := httptest.NewRecorder()
	a.CleanupTOTP(cleanupRec, cleanupReq)
	if cleanupRec.Code != http.StatusNoContent {
		t.Fatalf("cleanup status = %d", cleanupRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusNoContent {
		t.Fatalf("expected no-2FA login after cleanup, got %d", loginRec.Code)
	}
}

// Cross-tab guard: a stale cleanup carrying an outdated secret must not
// wipe a different, newer enrollment.
func TestTOTPCleanupCrossTabGuardNoOpsOnStaleSecret(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	first := totpEnable(t, a, sid)
	second := totpEnable(t, a, sid) // simulates a second tab enabling again
	if first.Secret == second.Secret {
		t.Fatal("expected EnableTOTP to generate a fresh secret each call")
	}

	// Confirm the second (current) enrollment.
	code, err := totp.GenerateCode(second.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	confirmReq := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/confirm", encodeJSON(t, totpConfirmRequest{CurrentPassword: "secret", Code: code})), sid)
	confirmRec := httptest.NewRecorder()
	a.ConfirmTOTP(confirmRec, confirmReq)
	if confirmRec.Code != http.StatusNoContent {
		t.Fatalf("confirm status = %d", confirmRec.Code)
	}

	// The stale first tab's cleanup, carrying the now-superseded secret,
	// must no-op rather than disabling the just-confirmed enrollment.
	staleCleanup := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/cleanup", encodeJSON(t, totpCleanupRequest{Secret: first.Secret})), sid)
	staleRec := httptest.NewRecorder()
	a.CleanupTOTP(staleRec, staleCleanup)
	if staleRec.Code != http.StatusNoContent {
		t.Fatalf("cleanup status = %d", staleRec.Code)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", encodeJSON(t, loginRequest{Username: "admin", Password: "secret"}))
	loginRec := httptest.NewRecorder()
	a.Login(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 2FA still required (stale cleanup should have no-op'd), got %d", loginRec.Code)
	}
}

func TestTOTPEndpointsRequireAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "secret", false)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    any
	}{
		{"enable", a.EnableTOTP, totpEnableRequest{CurrentPassword: "secret"}},
		{"confirm", a.ConfirmTOTP, totpConfirmRequest{CurrentPassword: "secret", Code: "000000"}},
		{"disable", a.DisableTOTP, totpDisableRequest{CurrentPassword: "secret"}},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/x", encodeJSON(t, c.body))
		rec := httptest.NewRecorder()
		c.handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d", c.name, rec.Code)
		}
	}
}

func TestTOTPEndpointsWrongCurrentPassword(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    any
	}{
		{"enable", a.EnableTOTP, totpEnableRequest{CurrentPassword: "wrong"}},
		{"confirm", a.ConfirmTOTP, totpConfirmRequest{CurrentPassword: "wrong", Code: "000000"}},
		{"disable", a.DisableTOTP, totpDisableRequest{CurrentPassword: "wrong"}},
	}
	for _, c := range cases {
		req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/x", encodeJSON(t, c.body)), sid)
		rec := httptest.NewRecorder()
		c.handler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d", c.name, rec.Code)
		}
	}
}

func TestTOTPCleanupExemptFromReAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	enabled := totpEnable(t, a, sid)
	// No current_password field at all in this request shape — cleanup
	// must still succeed, proving it's exempt from re-auth (KTD4).
	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/cleanup", encodeJSON(t, totpCleanupRequest{Secret: enabled.Secret})), sid)
	rec := httptest.NewRecorder()
	a.CleanupTOTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestTOTPRateLimited(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	var last *httptest.ResponseRecorder
	for i := 0; i < 11; i++ {
		req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/totp/confirm", encodeJSON(t, totpConfirmRequest{CurrentPassword: "secret", Code: "000000"})), sid)
		rec := httptest.NewRecorder()
		a.ConfirmTOTP(rec, req)
		last = rec
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", last.Code)
	}
}

// --- Backup codes (U4) ---

func TestRegenerateBackupCodesHappyPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/backup-codes/regenerate", encodeJSON(t, backupCodesRegenerateRequest{CurrentPassword: "secret"})), sid)
	rec := httptest.NewRecorder()
	a.RegenerateBackupCodes(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var first backupCodesRegenerateResponse
	if err := json.NewDecoder(rec.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	if len(first.Codes) == 0 {
		t.Fatal("expected codes in response")
	}

	req2 := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/backup-codes/regenerate", encodeJSON(t, backupCodesRegenerateRequest{CurrentPassword: "secret"})), sid)
	rec2 := httptest.NewRecorder()
	a.RegenerateBackupCodes(rec2, req2)
	var second backupCodesRegenerateResponse
	if err := json.NewDecoder(rec2.Body).Decode(&second); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first.Codes) == fmt.Sprint(second.Codes) {
		t.Fatal("expected a distinct code set on regenerate")
	}
}

// Covers AE1.
func TestRegenerateBackupCodesWrongPassword(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/backup-codes/regenerate", encodeJSON(t, backupCodesRegenerateRequest{CurrentPassword: "wrong"})), sid)
	rec := httptest.NewRecorder()
	a.RegenerateBackupCodes(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRegenerateBackupCodesRequiresAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "secret", false)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/security/backup-codes/regenerate", encodeJSON(t, backupCodesRegenerateRequest{CurrentPassword: "secret"}))
	rec := httptest.NewRecorder()
	a.RegenerateBackupCodes(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- Passkey (U5) ---

func TestPasskeyEnrollLinkHappyPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/passkey/enroll-link", encodeJSON(t, passkeyEnrollLinkRequest{CurrentPassword: "secret"})), sid)
	rec := httptest.NewRecorder()
	a.PasskeyEnrollLink(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp passkeyEnrollLinkResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.URL == "" || resp.URL[:20] != "/author/enroll?token" {
		t.Fatalf("unexpected url: %q", resp.URL)
	}
}

func TestPasskeyDisable(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/passkey/disable", encodeJSON(t, passkeyDisableRequest{CurrentPassword: "secret"})), sid)
	rec := httptest.NewRecorder()
	a.DisablePasskey(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPasskeyEndpointsRequireAuth(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a := testAuthUnconfigured(t, st)
	_ = a.svc.InitAdmin("admin", "secret", false)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    any
	}{
		{"enroll-link", a.PasskeyEnrollLink, passkeyEnrollLinkRequest{CurrentPassword: "secret"}},
		{"disable", a.DisablePasskey, passkeyDisableRequest{CurrentPassword: "secret"}},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/security/passkey/x", encodeJSON(t, c.body))
		rec := httptest.NewRecorder()
		c.handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d", c.name, rec.Code)
		}
	}
}

// Covers AE1 (shared).
func TestPasskeyEndpointsWrongPassword(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	a, sid := testAuth(t, st)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		body    any
	}{
		{"enroll-link", a.PasskeyEnrollLink, passkeyEnrollLinkRequest{CurrentPassword: "wrong"}},
		{"disable", a.DisablePasskey, passkeyDisableRequest{CurrentPassword: "wrong"}},
	}
	for _, c := range cases {
		req := withSession(httptest.NewRequest(http.MethodPost, "/api/admin/security/passkey/x", encodeJSON(t, c.body)), sid)
		rec := httptest.NewRecorder()
		c.handler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d", c.name, rec.Code)
		}
	}
}

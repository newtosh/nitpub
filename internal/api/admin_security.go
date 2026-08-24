package api

import (
	"encoding/json"
	"net/http"

	"github.com/newtosh/nitpub/internal/auth"
)

// securityRateLimit gates every handler in this file — KTD10.
func (a *Auth) securityRateLimit(w http.ResponseWriter, r *http.Request) bool {
	if !a.securityLimiter.Allow(clientIP(r)) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return false
	}
	return true
}

// reAuth verifies the operator's current password against the stored admin
// record (KTD11) — the standalone check every handler in this file needs,
// since only ChangePassword verifies the current password internally as
// part of its own mutation.
func (a *Auth) reAuth(w http.ResponseWriter, r *http.Request, password string) bool {
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	if _, err := a.svc.CheckPassword(rec.Username, password); err != nil {
		http.Error(w, "current password incorrect", http.StatusBadRequest)
		return false
	}
	return true
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword lets the operator set a new password from the admin panel.
// force=false — ChangePassword verifies current_password itself.
func (a *Auth) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := a.svc.ChangePassword(req.CurrentPassword, req.NewPassword, false); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type totpEnableRequest struct {
	CurrentPassword string `json:"current_password"`
}

type totpEnableResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

// EnableTOTP generates and immediately persists a TOTP secret (KTD2) — the
// Vue panel treats this response as "awaiting confirmation," not completion.
func (a *Auth) EnableTOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req totpEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !a.reAuth(w, r, req.CurrentPassword) {
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	secret, url, err := a.svc.Store().EnableTOTP("nitpub", rec.Username)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(totpEnableResponse{Secret: secret, URL: url})
}

type totpConfirmRequest struct {
	CurrentPassword string `json:"current_password"`
	Code            string `json:"code"`
}

// ConfirmTOTP validates against the secret the store actually holds (KTD3),
// fetched here — never a client-supplied value. On an invalid code it rolls
// back via DisableTOTP (KTD2); if that rollback itself fails, it returns
// 500 rather than the normal invalid-code 400, since a 400 would tell the
// client "retry" while the store may still hold TOTPEnabled=true.
func (a *Auth) ConfirmTOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req totpConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !a.reAuth(w, r, req.CurrentPassword) {
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if err := auth.ConfirmTOTPSetup(rec.TOTPSecret, req.Code); err != nil {
		if disableErr := a.svc.Store().DisableTOTP(); disableErr != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		http.Error(w, "invalid TOTP code", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type totpDisableRequest struct {
	CurrentPassword string `json:"current_password"`
}

// DisableTOTP is the operator-initiated disable — re-auth required (KTD11).
func (a *Auth) DisableTOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req totpDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !a.reAuth(w, r, req.CurrentPassword) {
		return
	}
	if err := a.svc.Store().DisableTOTP(); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type totpCleanupRequest struct {
	Secret string `json:"secret"`
}

// CleanupTOTP is the client-triggered abandon path (KTD4) — exempt from
// re-auth, since it's housekeeping tied to the re-auth the operator already
// completed moments earlier at /totp/enable, not a fresh disable action.
// Only disables if the store's current secret still matches the panel's
// held copy, guarding the cross-tab race where a second enrollment already
// confirmed successfully with a different secret.
func (a *Auth) CleanupTOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req totpCleanupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if rec.TOTPSecret != "" && rec.TOTPSecret == req.Secret {
		if err := a.svc.Store().DisableTOTP(); err != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

type backupCodesRegenerateRequest struct {
	CurrentPassword string `json:"current_password"`
}

type backupCodesRegenerateResponse struct {
	Codes []string `json:"codes"`
}

// RegenerateBackupCodes returns the new codes once (KTD7 — re-auth is the
// only confirmation gate; AE3 — never retrievable again after this response).
func (a *Auth) RegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req backupCodesRegenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !a.reAuth(w, r, req.CurrentPassword) {
		return
	}
	codes, err := a.svc.Store().RegenerateBackupCodes()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(backupCodesRegenerateResponse{Codes: codes})
}

type passkeyEnrollLinkRequest struct {
	CurrentPassword string `json:"current_password"`
}

type passkeyEnrollLinkResponse struct {
	URL string `json:"url"`
}

// PasskeyEnrollLink mirrors cmd/nitpub/admin_cmd.go's `webauthn register`
// exactly, generating only the link — the link itself opens the existing,
// unchanged WebAuthnEnrollView.vue flow (KTD6).
func (a *Auth) PasskeyEnrollLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req passkeyEnrollLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !a.reAuth(w, r, req.CurrentPassword) {
		return
	}
	et, err := auth.NewEnrollToken(auth.NowUTC())
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}
	if err := a.svc.Store().PutEnrollToken(et); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(passkeyEnrollLinkResponse{URL: "/author/enroll?token=" + et.Token})
}

type passkeyDisableRequest struct {
	CurrentPassword string `json:"current_password"`
}

// DisablePasskey removes the enrolled passkey — re-auth required (KTD11).
func (a *Auth) DisablePasskey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.securityRateLimit(w, r) {
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req passkeyDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if !a.reAuth(w, r, req.CurrentPassword) {
		return
	}
	if err := a.svc.DisableWebAuthn(); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

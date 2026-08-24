package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/newtosh/nitpub/internal/auth"
)

var (
	webauthnLoginSessions   = map[string]webauthn.SessionData{}
	webauthnLoginSessionsMu sync.Mutex
	webauthnRegSessions     = map[string]webauthn.SessionData{}
	webauthnRegSessionsMu   sync.Mutex
)

// WebAuthnLoginBegin starts a passkey assertion for pending 2FA login.
func (a *Auth) WebAuthnLoginBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.rateLimit(w, r) {
		return
	}
	var body struct {
		PendingToken string `json:"pending_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	pending, err := a.svc.Store().GetPending(body.PendingToken)
	if err != nil || auth.NowUTC().After(pending.ExpiresAt) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil || !rec.Settings.WebAuthnEnabled {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	options, session, err := a.svc.BeginWebAuthnLogin(rec)
	if err != nil {
		http.Error(w, "webauthn error", http.StatusInternalServerError)
		return
	}
	webauthnLoginSessionsMu.Lock()
	webauthnLoginSessions[body.PendingToken] = *session
	webauthnLoginSessionsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(options)
}

// WebAuthnLoginFinish completes passkey login and creates a session.
func (a *Auth) WebAuthnLoginFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.rateLimit(w, r) {
		return
	}
	pendingToken := r.URL.Query().Get("pending")
	if pendingToken == "" {
		http.Error(w, "missing pending token", http.StatusBadRequest)
		return
	}
	webauthnLoginSessionsMu.Lock()
	session, ok := webauthnLoginSessions[pendingToken]
	if ok {
		delete(webauthnLoginSessions, pendingToken)
	}
	webauthnLoginSessionsMu.Unlock()
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := a.svc.FinishWebAuthnLogin(rec, r, session); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = a.svc.Store().DeletePending(pendingToken)
	persistent := r.URL.Query().Get("remember_me") == "1" || r.URL.Query().Get("remember_me") == "true"
	if err := a.svc.CreateSession(w, r, persistent); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// WebAuthnRegisterBegin starts enrollment with a one-time enroll token.
func (a *Auth) WebAuthnRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	et, err := a.svc.Store().GetEnrollToken(body.Token)
	if err != nil || et.Used || auth.NowUTC().After(et.ExpiresAt) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	options, session, err := a.svc.BeginWebAuthnRegistration(rec)
	if err != nil {
		http.Error(w, "webauthn error", http.StatusInternalServerError)
		return
	}
	webauthnRegSessionsMu.Lock()
	webauthnRegSessions[body.Token] = *session
	webauthnRegSessionsMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(options)
}

// WebAuthnRegisterFinish completes enrollment.
func (a *Auth) WebAuthnRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	webauthnRegSessionsMu.Lock()
	session, ok := webauthnRegSessions[token]
	if ok {
		delete(webauthnRegSessions, token)
	}
	webauthnRegSessionsMu.Unlock()
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	et, err := a.svc.Store().GetEnrollToken(token)
	if err != nil || et.Used || auth.NowUTC().After(et.ExpiresAt) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := a.svc.FinishWebAuthnRegistration(rec, r, session); err != nil {
		http.Error(w, "registration failed", http.StatusBadRequest)
		return
	}
	_ = a.svc.Store().MarkEnrollTokenUsed(token)
	w.WriteHeader(http.StatusNoContent)
}

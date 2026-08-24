package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/newtosh/nitpub/internal/auth"
	"github.com/newtosh/nitpub/internal/inbox"
)

const sessionCookie = auth.SessionCookie

// Auth provides admin session authentication.
type Auth struct {
	svc     *auth.Service
	limiter *inbox.RateLimiter
	// securityLimiter is separate from limiter (Login/Verify) so routine
	// security-panel usage (a mistyped password, a couple of invalid TOTP
	// codes) can't exhaust the same budget and lock the operator out of
	// Login itself, not just the security panel.
	securityLimiter *inbox.RateLimiter
}

func NewAuth(svc *auth.Service) *Auth {
	return &Auth{
		svc:             svc,
		limiter:         inbox.NewRateLimiter(10, 15*time.Minute),
		securityLimiter: inbox.NewRateLimiter(10, 15*time.Minute),
	}
}

func (a *Auth) Authenticated(r *http.Request) bool {
	return a.svc.Authenticated(r)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

func (a *Auth) rateLimit(w http.ResponseWriter, r *http.Request) bool {
	if !a.limiter.Allow(clientIP(r)) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return false
	}
	return true
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

type loginResponse struct {
	Status       string   `json:"status"`
	Methods      []string `json:"methods,omitempty"`
	PendingToken string   `json:"pending_token,omitempty"`
}

type verifyRequest struct {
	PendingToken string          `json:"pending_token"`
	Method       string          `json:"method"`
	Code         string          `json:"code,omitempty"`
	Assertion    json.RawMessage `json:"assertion,omitempty"`
	RememberMe   bool            `json:"remember_me"`
}

// Login handles username/password and optional 2FA handoff.
func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.rateLimit(w, r) {
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	rec, err := a.svc.CheckPassword(req.Username, req.Password)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !a.svc.Needs2FA(rec) {
		if err := a.svc.CreateSession(w, r, req.RememberMe); err != nil {
			http.Error(w, "session error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	pending, err := auth.NewPendingAuth(auth.NowUTC())
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	if err := a.svc.Store().PutPending(pending); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	var methods []string
	if rec.Settings.TOTPEnabled {
		methods = append(methods, "totp")
	}
	if rec.Settings.WebAuthnEnabled {
		methods = append(methods, "webauthn")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(loginResponse{
		Status:       "2fa_required",
		Methods:      methods,
		PendingToken: pending.Token,
	})
}

// Verify completes 2FA and issues a session.
func (a *Auth) Verify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.rateLimit(w, r) {
		return
	}
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	pending, err := a.svc.Store().GetPending(req.PendingToken)
	if err != nil || auth.NowUTC().After(pending.ExpiresAt) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rec, err := a.svc.Store().GetAdmin()
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ok := false
	switch req.Method {
	case "totp":
		ok = auth.VerifyAdminTOTP(rec, req.Code)
	case "backup":
		ok, err = a.svc.Store().VerifyBackupCode(req.Code)
		if err != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
	case "webauthn":
		// WebAuthn login uses dedicated endpoints; reject here.
		http.Error(w, "use webauthn login endpoints", http.StatusBadRequest)
		return
	default:
		http.Error(w, "invalid method", http.StatusBadRequest)
		return
	}
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = a.svc.Store().DeletePending(req.PendingToken)
	if err := a.svc.CreateSession(w, r, req.RememberMe); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Logout clears the session cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.svc.Logout(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// Session reports whether the caller has a valid session.
func (a *Auth) Session(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"authenticated": a.svc.Authenticated(r),
	})
}

// AuthCheck is a minimal endpoint for a reverse proxy's forward_auth check
// (e.g. Caddy protecting an internal tool, like a GoatCounter dashboard,
// with nitpub's own admin login instead of a separate credential). Unlike
// Session, which always returns 200, this returns a non-2xx status when
// unauthenticated so the proxy actually blocks the request through: 204
// with no body when authenticated, 401 otherwise. No side effects, safe
// to call on every proxied request.
func (a *Auth) AuthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Settings returns admin settings for authenticated callers.
func (a *Auth) Settings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	settings, err := a.svc.Settings()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

type appearanceResponse struct {
	ThemeID string `json:"theme_id"`
}

// Appearance returns the public palette id for page bootstrap.
func (a *Auth) Appearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	app, err := a.svc.PublicAppearance()
	if err != nil {
		app = auth.PublicAppearance{ThemeID: auth.DefaultThemeID}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(appearanceResponse{ThemeID: app.ThemeID})
}

type updateSettingsRequest struct {
	ThemeID *string `json:"theme_id"`
}

// UpdateSettings patches instance settings (theme_id in v1).
func (a *Auth) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.svc.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.ThemeID != nil {
		if err := a.svc.SetThemeID(*req.ThemeID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	settings, err := a.svc.Settings()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(settings)
}

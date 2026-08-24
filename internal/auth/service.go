package auth

import (
	"fmt"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/newtosh/nitpub/internal/store"
)

// WebAuthnUser adapts AdminRecord for go-webauthn.
type WebAuthnUser struct {
	rec *AdminRecord
}

func (u WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.rec.Username)
}

func (u WebAuthnUser) WebAuthnName() string {
	return u.rec.Username
}

func (u WebAuthnUser) WebAuthnDisplayName() string {
	return u.rec.Username
}

func (u WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.rec.WebAuthnCredentials
}

// Service coordinates admin authentication.
type Service struct {
	store    *Store
	webauthn *webauthn.WebAuthn
	domain   string

	// cookieDomain, when set, widens the session cookie beyond the exact
	// host (e.g. ".example.com") so a trusted subdomain — a Caddy
	// forward_auth check for a proxied internal tool — can read it too.
	// Empty by default: unchanged host-only cookie behavior. Widening
	// this is a deliberate security tradeoff (any subdomain that can set
	// cookies could then also read/spoof this one), so it's opt-in via
	// SetCookieDomain, never inferred from the site domain.
	cookieDomain string
}

func NewService(st *store.Store, domain, displayName string) (*Service, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: displayName,
		RPID:          domain,
		RPOrigins:     []string{originForDomain(domain)},
	})
	if err != nil {
		return nil, err
	}
	return &Service{
		store:    NewStore(st),
		webauthn: wa,
		domain:   domain,
	}, nil
}

// SetCookieDomain opts the session cookie into a wider Domain attribute.
// See the Service.cookieDomain field comment for the security tradeoff.
func (svc *Service) SetCookieDomain(d string) {
	svc.cookieDomain = d
}

func originForDomain(domain string) string {
	// Config supplies scheme via BaseURL at call sites; default https for RP.
	return "https://" + domain
}

// SetRPOrigin overrides origin when running in HTTP dev mode.
func (svc *Service) SetRPOrigin(origin string) {
	svc.webauthn.Config.RPOrigins = []string{origin}
}

func (svc *Service) Store() *Store { return svc.store }

func (svc *Service) CheckPassword(username, password string) (*AdminRecord, error) {
	rec, err := svc.store.GetAdmin()
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if rec.Username != username {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !VerifyPassword(password, rec.PasswordHash) {
		return nil, fmt.Errorf("invalid credentials")
	}
	return rec, nil
}

func (svc *Service) Needs2FA(rec *AdminRecord) bool {
	return rec.Settings.TOTPEnabled || rec.Settings.WebAuthnEnabled
}

func (svc *Service) CreateSession(w http.ResponseWriter, r *http.Request, persistent bool) error {
	id, err := NewSessionID()
	if err != nil {
		return err
	}
	now := NowUTC()
	sess := CreateSessionRecord(id, now, persistent)
	if err := svc.store.PutSession(sess); err != nil {
		return err
	}
	SetSessionCookie(w, r, id, sess.ExpiresAt, persistent, svc.cookieDomain)
	return nil
}

func (svc *Service) Authenticated(r *http.Request) bool {
	id, err := SessionIDFromRequest(r)
	if err != nil {
		return false
	}
	_, err = svc.store.ValidateSession(id, NowUTC())
	return err == nil
}

func (svc *Service) Logout(w http.ResponseWriter, r *http.Request) {
	id, err := SessionIDFromRequest(r)
	if err == nil {
		_ = svc.store.DeleteSession(id)
	}
	ClearSessionCookie(w, r, svc.cookieDomain)
}

func (svc *Service) BeginWebAuthnRegistration(rec *AdminRecord) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
	user := WebAuthnUser{rec: rec}
	return svc.webauthn.BeginRegistration(user)
}

func (svc *Service) FinishWebAuthnRegistration(rec *AdminRecord, r *http.Request, session webauthn.SessionData) error {
	user := WebAuthnUser{rec: rec}
	cred, err := svc.webauthn.FinishRegistration(user, session, r)
	if err != nil {
		return err
	}
	rec.WebAuthnCredentials = append(rec.WebAuthnCredentials, *cred)
	rec.Settings.WebAuthnEnabled = true
	return svc.store.SaveAdmin(rec)
}

func (svc *Service) BeginWebAuthnLogin(rec *AdminRecord) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
	user := WebAuthnUser{rec: rec}
	return svc.webauthn.BeginLogin(user)
}

func (svc *Service) FinishWebAuthnLogin(rec *AdminRecord, r *http.Request, session webauthn.SessionData) error {
	user := WebAuthnUser{rec: rec}
	_, err := svc.webauthn.FinishLogin(user, session, r)
	return err
}

func (svc *Service) DisableWebAuthn() error {
	rec, err := svc.store.GetAdmin()
	if err != nil {
		return err
	}
	rec.WebAuthnCredentials = nil
	rec.Settings.WebAuthnEnabled = false
	return svc.store.SaveAdmin(rec)
}

func (svc *Service) Reset2FA() error {
	rec, err := svc.store.GetAdmin()
	if err != nil {
		return err
	}
	rec.TOTPSecret = ""
	rec.WebAuthnCredentials = nil
	rec.Settings.TOTPEnabled = false
	rec.Settings.WebAuthnEnabled = false
	rec.BackupCodeHashes = nil
	return svc.store.SaveAdmin(rec)
}

func (svc *Service) Settings() (AdminSettings, error) {
	rec, err := svc.store.GetAdmin()
	if err != nil {
		return AdminSettings{}, err
	}
	return rec.Settings, nil
}

func (svc *Service) ThemeID() (string, error) {
	app, err := svc.PublicAppearance()
	return app.ThemeID, err
}

func (svc *Service) PublicAppearance() (PublicAppearance, error) {
	settings, err := svc.Settings()
	if err != nil {
		return PublicAppearance{ThemeID: DefaultThemeID}, err
	}
	return PublicAppearance{
		ThemeID: NormalizeThemeID(settings.ThemeID),
	}, nil
}

func (svc *Service) SetThemeID(id string) error {
	if _, ok := themeAliases[id]; !ok {
		if _, ok := validThemes[id]; !ok {
			return fmt.Errorf("unknown theme %q", id)
		}
	}
	normalized := NormalizeThemeID(id)
	rec, err := svc.store.GetAdmin()
	if err != nil {
		return err
	}
	rec.Settings.ThemeID = normalized
	return svc.store.SaveAdmin(rec)
}

// InitAdmin creates the sole admin account. With force, an existing admin
// record is replaced outright — a fresh account with no TOTP, WebAuthn, or
// backup codes carried over, not a rename of the existing one.
func (svc *Service) InitAdmin(username, password string, force bool) error {
	if !force {
		exists, err := svc.store.AdminExists()
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("admin already exists")
		}
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return svc.store.SaveAdmin(&AdminRecord{
		Username:     username,
		PasswordHash: hash,
		Settings:     AdminSettings{Flags: map[string]bool{}},
	})
}

func (svc *Service) ChangePassword(current, newPassword string, force bool) error {
	rec, err := svc.store.GetAdmin()
	if err != nil {
		return err
	}
	if !force {
		if !VerifyPassword(current, rec.PasswordHash) {
			return fmt.Errorf("current password incorrect")
		}
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	rec.PasswordHash = hash
	return svc.store.SaveAdmin(rec)
}

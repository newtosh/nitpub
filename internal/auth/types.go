package auth

import (
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

const adminKey = "admin"

// AdminSettings holds feature toggles for the admin account and instance.
type AdminSettings struct {
	TOTPEnabled     bool            `json:"totp_enabled"`
	WebAuthnEnabled bool            `json:"webauthn_enabled"`
	ThemeID         string          `json:"theme_id,omitempty"`
	Flags           map[string]bool `json:"flags,omitempty"`
}

// AdminRecord is the persisted single-admin credential bundle.
type AdminRecord struct {
	Username            string                `json:"username"`
	PasswordHash        string                `json:"password_hash"`
	TOTPSecret          string                `json:"totp_secret,omitempty"`
	WebAuthnCredentials []webauthn.Credential `json:"webauthn_credentials,omitempty"`
	BackupCodeHashes    []string              `json:"backup_code_hashes,omitempty"`
	Settings            AdminSettings         `json:"settings"`
}

// Session is a server-side admin session.
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// PendingAuth holds state between password verification and 2FA completion.
type PendingAuth struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// EnrollToken authorizes a one-time WebAuthn registration ceremony.
type EnrollToken struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

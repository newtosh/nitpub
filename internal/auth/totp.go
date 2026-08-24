package auth

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new base32 secret for TOTP enrollment.
func GenerateTOTPSecret(issuer, account string) (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Period:      30,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
		Rand:        rand.Reader,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP checks a 6-digit code against the stored secret.
func VerifyTOTP(secret, code string) bool {
	secret = strings.TrimSpace(secret)
	code = strings.TrimSpace(code)
	if secret == "" || code == "" {
		return false
	}
	return totp.Validate(code, secret)
}

// FormatTOTPSecretForDisplay groups the secret for manual entry.
func FormatTOTPSecretForDisplay(secret string) string {
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	raw, err := enc.DecodeString(secret)
	if err != nil {
		return secret
	}
	s := enc.EncodeToString(raw)
	var parts []string
	for i := 0; i < len(s); i += 4 {
		end := i + 4
		if end > len(s) {
			end = len(s)
		}
		parts = append(parts, s[i:end])
	}
	return strings.Join(parts, " ")
}

func (s *Store) EnableTOTP(issuer, account string) (secret, url string, err error) {
	secret, url, err = GenerateTOTPSecret(issuer, account)
	if err != nil {
		return "", "", err
	}
	rec, err := s.GetAdmin()
	if err != nil {
		return "", "", err
	}
	rec.TOTPSecret = secret
	rec.Settings.TOTPEnabled = true
	if err := s.SaveAdmin(rec); err != nil {
		return "", "", err
	}
	return secret, url, nil
}

func (s *Store) DisableTOTP() error {
	rec, err := s.GetAdmin()
	if err != nil {
		return err
	}
	rec.TOTPSecret = ""
	rec.Settings.TOTPEnabled = false
	return s.SaveAdmin(rec)
}

func VerifyAdminTOTP(rec *AdminRecord, code string) bool {
	if !rec.Settings.TOTPEnabled || rec.TOTPSecret == "" {
		return false
	}
	return VerifyTOTP(rec.TOTPSecret, code)
}

// ConfirmTOTPSetup verifies a code before keeping enrollment (optional CLI check).
func ConfirmTOTPSetup(secret, code string) error {
	if !VerifyTOTP(secret, code) {
		return fmt.Errorf("invalid TOTP code")
	}
	return nil
}

// NowUTC is a test seam for time.
var NowUTC = func() time.Time { return time.Now().UTC() }

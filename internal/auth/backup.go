package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const backupCodeCount = 10

// GenerateBackupCodes creates plaintext codes and bcrypt hashes.
func GenerateBackupCodes() (plain []string, hashes []string, err error) {
	for i := 0; i < backupCodeCount; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		code := hex.EncodeToString(b)
		plain = append(plain, code)
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		hashes = append(hashes, string(hash))
	}
	return plain, hashes, nil
}

// VerifyBackupCode checks and consumes a matching backup code on the admin record.
func (s *Store) VerifyBackupCode(code string) (bool, error) {
	rec, err := s.GetAdmin()
	if err != nil {
		return false, err
	}
	for i, hash := range rec.BackupCodeHashes {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil {
			rec.BackupCodeHashes = append(rec.BackupCodeHashes[:i], rec.BackupCodeHashes[i+1:]...)
			if err := s.SaveAdmin(rec); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) RegenerateBackupCodes() ([]string, error) {
	rec, err := s.GetAdmin()
	if err != nil {
		return nil, err
	}
	plain, hashes, err := GenerateBackupCodes()
	if err != nil {
		return nil, err
	}
	rec.BackupCodeHashes = hashes
	if err := s.SaveAdmin(rec); err != nil {
		return nil, err
	}
	return plain, nil
}

func FormatBackupCodes(codes []string) string {
	var out string
	for i, c := range codes {
		if i > 0 {
			out += "\n"
		}
		out += c
	}
	return out
}

func BackupCodesHelp() string {
	return fmt.Sprintf("Save these %d backup codes in a safe place. Each works once.", backupCodeCount)
}

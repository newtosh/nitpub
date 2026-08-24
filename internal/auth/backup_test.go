package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBackupCodeSingleUse(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()
	hash, _ := HashPassword("pw")
	if err := s.SaveAdmin(&AdminRecord{Username: "a", PasswordHash: hash}); err != nil {
		t.Fatal(err)
	}
	code := "deadbeefcafebabe"
	h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	rec, _ := s.GetAdmin()
	rec.BackupCodeHashes = []string{string(h)}
	if err := s.SaveAdmin(rec); err != nil {
		t.Fatal(err)
	}
	ok, err := s.VerifyBackupCode(code)
	if err != nil || !ok {
		t.Fatalf("first use: ok=%v err=%v", ok, err)
	}
	ok, err = s.VerifyBackupCode(code)
	if err != nil || ok {
		t.Fatalf("reuse: ok=%v err=%v", ok, err)
	}
}

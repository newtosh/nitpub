package auth

import (
	"path/filepath"
	"testing"

	"github.com/newtosh/nitpub/internal/store"
)

func testStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(st), func() { _ = st.Close() }
}

func TestAdminRoundTrip(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()

	if exists, _ := s.AdminExists(); exists {
		t.Fatal("expected no admin")
	}
	rec := &AdminRecord{
		Username:     "admin",
		PasswordHash: "hash",
		Settings:     AdminSettings{TOTPEnabled: true},
	}
	if err := s.SaveAdmin(rec); err != nil {
		t.Fatal(err)
	}
	exists, err := s.AdminExists()
	if err != nil || !exists {
		t.Fatalf("AdminExists: exists=%v err=%v", exists, err)
	}
	got, err := s.GetAdmin()
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "admin" || !got.Settings.TOTPEnabled {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestAdminNotConfigured(t *testing.T) {
	s, cleanup := testStore(t)
	defer cleanup()
	_, err := s.GetAdmin()
	if err == nil {
		t.Fatal("expected error")
	}
}

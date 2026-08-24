package api

import (
	"testing"

	"github.com/newtosh/nitpub/internal/auth"
	"github.com/newtosh/nitpub/internal/store"
)

func testAuth(t *testing.T, st *store.Store) (*Auth, string) {
	t.Helper()
	svc, err := auth.NewService(st, "example.test", "nitpub")
	if err != nil {
		t.Fatal(err)
	}
	svc.SetRPOrigin("http://example.test")
	if err := svc.InitAdmin("admin", "secret", false); err != nil {
		t.Fatal(err)
	}
	id, err := auth.NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	sess := auth.CreateSessionRecord(id, auth.NowUTC(), true)
	if err := svc.Store().PutSession(sess); err != nil {
		t.Fatal(err)
	}
	return NewAuth(svc), id
}

func testAuthUnconfigured(t *testing.T, st *store.Store) *Auth {
	t.Helper()
	svc, err := auth.NewService(st, "example.test", "nitpub")
	if err != nil {
		t.Fatal(err)
	}
	svc.SetRPOrigin("http://example.test")
	return NewAuth(svc)
}

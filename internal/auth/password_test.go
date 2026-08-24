package auth

import "testing"

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("correct horse battery", hash) {
		t.Fatal("expected verify ok")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("expected verify fail")
	}
}

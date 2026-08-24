package actor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
)

func testConfig() config.Config {
	return config.Config{
		Domain:  "example.test",
		Actor:   "user",
		BaseURL: "http://example.test",
	}
}

func TestWebFingerHappyPath(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ap := apstore.New(st, "http://example.test/actor")
	svc, err := LoadOrCreate(testConfig(), ap, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:user@example.test", nil)
	rec := httptest.NewRecorder()
	svc.ServeWebFinger(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var doc jrd
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Subject != "acct:user@example.test" {
		t.Fatalf("subject = %q", doc.Subject)
	}
	if len(doc.Links) != 1 || doc.Links[0].Href != "http://example.test/actor" {
		t.Fatalf("links = %+v", doc.Links)
	}
}

func TestWebFingerUnknownResource(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ap := apstore.New(st, "http://example.test/actor")
	svc, err := LoadOrCreate(testConfig(), ap, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:other@example.test", nil)
	rec := httptest.NewRecorder()
	svc.ServeWebFinger(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestWebFingerCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	cfg := config.Config{
		Domain:  "nitpub.com",
		Actor:   "nit",
		BaseURL: "https://nitpub.com",
	}
	ap := apstore.New(st, "https://nitpub.com/actor")
	svc, err := LoadOrCreate(cfg, ap, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/.well-known/webfinger?resource=acct:Nit@nitpub.com", nil)
	rec := httptest.NewRecorder()
	svc.ServeWebFinger(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestActorDocument(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ap := apstore.New(st, "http://example.test/actor")
	svc, err := LoadOrCreate(testConfig(), ap, nil)
	if err != nil {
		t.Fatal(err)
	}

	act := svc.Actor()
	if string(act.ID) != "http://example.test/actor" {
		t.Fatalf("id = %q", act.ID)
	}
	if string(act.PublicKey.ID) != "http://example.test/actor#main-key" {
		t.Fatalf("key id = %q", act.PublicKey.ID)
	}
	if act.PublicKey.PublicKeyPem == "" {
		t.Fatal("missing public key pem")
	}
	if !strings.Contains(act.PublicKey.PublicKeyPem, "BEGIN PUBLIC KEY") {
		t.Fatalf("unexpected public key pem format: %q", act.PublicKey.PublicKeyPem[:40])
	}
}

func TestActorServeJSONLD(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ap := apstore.New(st, "http://example.test/actor")
	svc, err := LoadOrCreate(testConfig(), ap, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/actor", nil)
	rec := httptest.NewRecorder()
	svc.ServeActor(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/activity+json" {
		t.Fatalf("content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"@context"`) {
		t.Fatalf("missing @context in actor JSON-LD: %s", body)
	}
	if !strings.Contains(body, "w3id.org/security/v1") {
		t.Fatal("missing security context in actor JSON-LD")
	}
	if !strings.Contains(body, `"manuallyApprovesFollowers":false`) {
		t.Fatalf("expected open follows in actor JSON-LD: %s", body)
	}
	if !strings.Contains(body, `"discoverable":true`) {
		t.Fatalf("expected discoverable actor JSON-LD: %s", body)
	}
	if !strings.Contains(body, `"type":"PropertyValue"`) || !strings.Contains(body, `rel=`) || !strings.Contains(body, `me nofollow`) {
		t.Fatalf("expected website attachment with rel=me: %s", body)
	}
}

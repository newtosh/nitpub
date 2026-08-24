package federation

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	draft "github.com/go-fed/httpsig"
)

func testActorAndKey(t *testing.T) (vocab.Actor, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubDER})
	id := vocab.IRI("https://example.test/actor")
	act := vocab.Actor{
		ID: id,
		PublicKey: vocab.PublicKey{
			ID:           vocab.IRI("https://example.test/actor#main-key"),
			Owner:        id,
			PublicKeyPem: string(pubPEM),
		},
	}
	return act, key
}

func TestSignDraftIncludesDigest(t *testing.T) {
	act, key := testActorAndKey(t)
	signer := NewSigner(act, key)
	body := []byte(`{"type":"Create"}`)
	req := httptest.NewRequest(http.MethodPost, "https://remote.test/inbox", bytes.NewReader(body))
	req.Header.Set("Host", "remote.test")
	req.Header.Set("Content-Type", client.ContentTypeJsonActivity)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	if err := signer.SignDraft(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("Digest") == "" {
		t.Fatal("missing Digest header")
	}
	if req.Header.Get("Signature") == "" {
		t.Fatal("missing Signature header")
	}

	v, err := draft.NewVerifier(req)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(act.PublicKey.PublicKeyPem))
	pk, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(pk, draft.RSA_SHA256); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestDeliverUnreachableInbox(t *testing.T) {
	act, key := testActorAndKey(t)
	signer := NewSigner(act, key)
	cl := client.New()
	err := Deliver(cl, signer, "http://127.0.0.1:1/inbox", map[string]string{"type": "Create"})
	if err == nil {
		t.Fatal("expected delivery error")
	}
}

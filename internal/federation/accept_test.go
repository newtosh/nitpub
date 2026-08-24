package federation

import (
	"encoding/json"
	"testing"

	vocab "github.com/go-ap/activitypub"
)

func TestBuildAccept(t *testing.T) {
	follow := map[string]any{
		"id":     "https://remote.test/follows/1",
		"type":   "Follow",
		"actor":  "https://remote.test/users/alice",
		"object": "https://example.test/actor",
	}
	accept, err := BuildAccept("https://example.test/actor", "https://example.test", follow)
	if err != nil {
		t.Fatal(err)
	}
	if vocab.IRI(accept.Actor.GetLink()) != "https://example.test/actor" {
		t.Fatalf("actor = %v", accept.Actor)
	}
	if accept.Object == nil {
		t.Fatal("missing object")
	}
	if len(accept.To) != 1 || vocab.IRI(accept.To[0].GetLink()) != "https://remote.test/users/alice" {
		t.Fatalf("to = %v", accept.To)
	}

	body, err := MarshalActivity(accept)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["@context"]; !ok {
		t.Fatalf("missing @context: %s", body)
	}
	obj, ok := doc["object"].(map[string]any)
	if !ok {
		t.Fatalf("object should be embedded Follow: %s", body)
	}
	if obj["type"] != "Follow" || obj["id"] != "https://remote.test/follows/1" {
		t.Fatalf("unexpected embedded follow: %v", obj)
	}
}

func TestBuildAcceptRequiresFollowID(t *testing.T) {
	_, err := BuildAccept("https://example.test/actor", "https://example.test", map[string]any{"type": "Follow"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildAcceptRequiresActorAndObject(t *testing.T) {
	_, err := BuildAccept("https://example.test/actor", "https://example.test", map[string]any{
		"id":   "https://remote.test/follows/1",
		"type": "Follow",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

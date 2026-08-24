package federation

import (
	"testing"

	vocab "github.com/go-ap/activitypub"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/store"
)

func TestResendAcceptsUsesStoredFollowID(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	actorIRI := "https://example.test/actor"
	ap := apstore.New(st, vocab.IRI(actorIRI))
	if err := ap.AddFollower(apstore.Follower{
		ActorIRI: "https://remote.test/users/alice",
		InboxIRI: "https://remote.test/users/alice/inbox",
		FollowID: "https://remote.test/follows/1",
	}); err != nil {
		t.Fatal(err)
	}

	var delivered any
	sent, err := ResendAccepts(ap, actorIRI, "https://example.test", func(_ string, activity any) error {
		delivered = activity
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sent != 1 || delivered == nil {
		t.Fatalf("sent=%d delivered=%v", sent, delivered)
	}
}

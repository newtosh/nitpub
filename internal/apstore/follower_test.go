package apstore

import "testing"

func TestFollowerDeliveryInboxPrefersShared(t *testing.T) {
	f := Follower{
		ActorIRI:       "https://mastodon.social/users/alice",
		InboxIRI:       "https://mastodon.social/users/alice/inbox",
		SharedInboxIRI: "https://mastodon.social/inbox",
	}
	if got := f.DeliveryInbox(); got != "https://mastodon.social/inbox" {
		t.Fatalf("DeliveryInbox() = %q", got)
	}
}

func TestFollowerDeliveryInboxDerivesSharedFromActor(t *testing.T) {
	f := Follower{
		ActorIRI: "https://mastodon.social/users/alice",
		InboxIRI: "https://mastodon.social/users/alice/inbox",
	}
	if got := f.DeliveryInbox(); got != "https://mastodon.social/inbox" {
		t.Fatalf("DeliveryInbox() = %q", got)
	}
}

func TestUniqueDeliveryInboxesDedupes(t *testing.T) {
	inboxes := UniqueDeliveryInboxes([]Follower{
		{ActorIRI: "https://mastodon.social/users/alice", InboxIRI: "https://mastodon.social/users/alice/inbox"},
		{ActorIRI: "https://mastodon.social/users/bob", InboxIRI: "https://mastodon.social/users/bob/inbox"},
	})
	if len(inboxes) != 1 || inboxes[0] != "https://mastodon.social/inbox" {
		t.Fatalf("UniqueDeliveryInboxes() = %#v", inboxes)
	}
}

package federation

import (
	"fmt"

	"github.com/newtosh/nitpub/internal/apstore"
)

// ResendAccepts delivers Accept activities to all stored followers.
func ResendAccepts(ap *apstore.AP, actorIRI, baseURL string, deliver func(inboxURL string, activity any) error) (int, error) {
	followers, err := ap.ListFollowers()
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, follower := range followers {
		follow, err := followActivityForResend(ap, follower, actorIRI)
		if err != nil {
			return sent, err
		}
		accept, err := BuildAccept(actorIRI, baseURL, follow)
		if err != nil {
			return sent, fmt.Errorf("follower %s: %w", follower.ActorIRI, err)
		}
		if err := deliver(follower.InboxIRI, accept); err != nil {
			return sent, fmt.Errorf("follower %s: %w", follower.ActorIRI, err)
		}
		sent++
	}
	return sent, nil
}

func followActivityForResend(ap *apstore.AP, follower apstore.Follower, actorIRI string) (map[string]any, error) {
	if follower.FollowID != "" {
		return map[string]any{
			"id":     follower.FollowID,
			"type":   "Follow",
			"actor":  follower.ActorIRI,
			"object": actorIRI,
		}, nil
	}
	follow, err := ap.FindFollowActivity(follower.ActorIRI)
	if err != nil {
		return nil, err
	}
	if follow != nil {
		return follow, nil
	}
	return nil, fmt.Errorf("no stored Follow activity for %s", follower.ActorIRI)
}

package federation

import (
	"context"
	"log"
	"net/url"
	"strings"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"

	"github.com/newtosh/nitpub/internal/apstore"
)

const maxKeyFetchAttempts = 5

// ProcessKeyFetchQueue runs one drain pass (for tests).
func StartKeyFetchWorker(ctx context.Context, ap *apstore.AP, cl *client.C) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := processKeyFetchQueue(ap, cl); err != nil {
					log.Printf("key-fetch worker: %v", err)
				}
			}
		}
	}()
}

func processKeyFetchQueue(ap *apstore.AP, cl *client.C) error {
	pending, err := ap.ListPendingKeyFetches()
	if err != nil {
		return err
	}
	for keyID, state := range pending {
		if state.Attempts >= maxKeyFetchAttempts {
			_ = ap.RemoveKeyFetch(keyID)
			continue
		}
		actorIRI := keyIDToActorIRI(keyID)
		if actorIRI == "" {
			_ = ap.RemoveKeyFetch(keyID)
			continue
		}
		item, err := cl.LoadIRI(vocab.IRI(actorIRI))
		if err != nil {
			_ = ap.IncrementKeyFetchAttempt(keyID)
			continue
		}
		var act vocab.Actor
		if err := vocab.OnActor(item, func(a *vocab.Actor) error {
			act = *a
			return nil
		}); err != nil {
			_ = ap.IncrementKeyFetchAttempt(keyID)
			continue
		}
		if err := CacheRemoteActor(ap, act); err != nil {
			_ = ap.IncrementKeyFetchAttempt(keyID)
			continue
		}
		_ = ap.RemoveKeyFetch(keyID)
	}
	return nil
}

func keyIDToActorIRI(keyID string) string {
	u, err := url.Parse(keyID)
	if err != nil {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func ProcessKeyFetchQueue(ctx context.Context, ap *apstore.AP, cl *client.C) {
	_ = ctx
	_ = processKeyFetchQueue(ap, cl)
}

// KeyIDActorIRI exposes actor IRI resolution for tests.
func KeyIDActorIRI(keyID string) string {
	return strings.TrimSpace(keyIDToActorIRI(keyID))
}

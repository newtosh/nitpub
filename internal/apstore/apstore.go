package apstore

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	"github.com/openshift/osin"
	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// AP implements auth/oauth storage lookups for federation.
type AP struct {
	db            *bolt.DB
	localActorIRI activitypub.IRI
}

func New(st *store.Store, localActorIRI activitypub.IRI) *AP {
	return &AP{db: st.DB(), localActorIRI: localActorIRI}
}

func (a *AP) Load(iri activitypub.IRI, _ ...filters.Check) (activitypub.Item, error) {
	key := []byte(string(iri))
	var raw []byte
	err := a.db.View(func(tx *bolt.Tx) error {
		raw = tx.Bucket([]byte(store.BucketActor)).Get(key)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.NotFoundf("actor %s", iri)
	}
	var act activitypub.Actor
	if err := json.Unmarshal(raw, &act); err != nil {
		return nil, err
	}
	return act, nil
}

func (a *AP) LoadAccess(_ string) (*osin.AccessData, error) {
	return nil, errors.NotFoundf("oauth not configured")
}

func (a *AP) SaveActor(iri activitypub.IRI, act activitypub.Actor) error {
	raw, err := json.Marshal(act)
	if err != nil {
		return err
	}
	return a.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketActor)).Put([]byte(string(iri)), raw)
	})
}

func (a *AP) GetMeta(key string) ([]byte, error) {
	var raw []byte
	err := a.db.View(func(tx *bolt.Tx) error {
		raw = tx.Bucket([]byte(store.BucketMeta)).Get([]byte(key))
		return nil
	})
	return raw, err
}

func (a *AP) PutMeta(key string, value []byte) error {
	return a.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketMeta)).Put([]byte(key), value)
	})
}

func (a *AP) ListFollowers() ([]Follower, error) {
	var out []Follower
	err := a.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketFollowers)).ForEach(func(k, v []byte) error {
			var f Follower
			if err := json.Unmarshal(v, &f); err != nil {
				return err
			}
			out = append(out, f)
			return nil
		})
	})
	return out, err
}

func (a *AP) AddFollower(f Follower) error {
	raw, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return a.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketFollowers)).Put([]byte(f.ActorIRI), raw)
	})
}

func (a *AP) SaveInboxActivity(id string, raw []byte) error {
	return a.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketInbox)).Put([]byte(id), raw)
	})
}

// FindFollowActivity returns the most recent Follow activity from a remote actor.
func (a *AP) FindFollowActivity(actorIRI string) (map[string]any, error) {
	var found map[string]any
	err := a.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.BucketInbox))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, raw []byte) error {
			var activity map[string]any
			if err := json.Unmarshal(raw, &activity); err != nil {
				return nil
			}
			if activity["type"] != "Follow" {
				return nil
			}
			if activityPubIRI(activity["actor"]) != actorIRI {
				return nil
			}
			found = activity
			return nil
		})
	})
	return found, err
}

func activityPubIRI(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if id, ok := x["id"].(string); ok {
			return id
		}
	}
	return ""
}

func (a *AP) EnqueueKeyFetch(keyID string) error {
	if keyID == "" {
		return nil
	}
	return a.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.BucketKeyFetch))
		if b.Get([]byte(keyID)) != nil {
			return nil
		}
		return b.Put([]byte(keyID), []byte(`{"attempts":0}`))
	})
}

func (a *AP) ListPendingKeyFetches() (map[string]KeyFetchState, error) {
	out := make(map[string]KeyFetchState)
	err := a.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketKeyFetch)).ForEach(func(k, v []byte) error {
			var state KeyFetchState
			if err := json.Unmarshal(v, &state); err != nil {
				state = KeyFetchState{}
			}
			out[string(k)] = state
			return nil
		})
	})
	return out, err
}

func (a *AP) RemoveKeyFetch(keyID string) error {
	return a.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketKeyFetch)).Delete([]byte(keyID))
	})
}

func (a *AP) IncrementKeyFetchAttempt(keyID string) error {
	return a.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.BucketKeyFetch))
		var state KeyFetchState
		if raw := b.Get([]byte(keyID)); len(raw) > 0 {
			_ = json.Unmarshal(raw, &state)
		}
		state.Attempts++
		raw, err := json.Marshal(state)
		if err != nil {
			return err
		}
		return b.Put([]byte(keyID), raw)
	})
}

// KeyFetchState tracks async remote key fetch retries.
type KeyFetchState struct {
	Attempts int `json:"attempts"`
}

// Follower is a remote actor that follows the local actor.
type Follower struct {
	ActorIRI       string `json:"actor_iri"`
	InboxIRI       string `json:"inbox_iri"`
	SharedInboxIRI string `json:"shared_inbox_iri,omitempty"`
	FollowID       string `json:"follow_id,omitempty"`
}

// DeliveryInbox returns the inbox URL for fan-out Create activities.
// Mastodon and most fediverse software expect public posts on the shared inbox.
func (f Follower) DeliveryInbox() string {
	if f.SharedInboxIRI != "" {
		return f.SharedInboxIRI
	}
	if u, err := url.Parse(f.ActorIRI); err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host + "/inbox"
	}
	return f.InboxIRI
}

// UniqueDeliveryInboxes returns deduplicated shared inboxes for follower fan-out.
func UniqueDeliveryInboxes(followers []Follower) []string {
	seen := make(map[string]struct{}, len(followers))
	out := make([]string, 0, len(followers))
	for _, f := range followers {
		inbox := f.DeliveryInbox()
		if inbox == "" {
			continue
		}
		if _, ok := seen[inbox]; ok {
			continue
		}
		seen[inbox] = struct{}{}
		out = append(out, inbox)
	}
	return out
}

func (a *AP) LocalActorIRI() activitypub.IRI {
	return a.localActorIRI
}

func FormatAcct(username, domain string) string {
	return fmt.Sprintf("acct:%s@%s", username, domain)
}

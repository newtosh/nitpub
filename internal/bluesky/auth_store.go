package bluesky

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// authKey is fixed — nitpub connects to exactly one Bluesky account at a
// time, mirroring mastodon.ReferenceAuthStore.
var authKey = []byte("current")

// Auth is the admin's connected Bluesky account. Only the long-lived
// refreshJwt is persisted — the short-lived accessJwt is re-derived via
// Client.RefreshSession when needed, never stored.
type Auth struct {
	DID        string `json:"did"`
	Handle     string `json:"handle"`
	RefreshJWT string `json:"refreshJwt"`
	// Invalid is set by a later unit when RefreshSession fails using this
	// refreshJwt (KTD12) — surfaced to the admin as needs_reconnect.
	Invalid bool `json:"invalid"`
}

// AuthStore persists the single current Auth, if any.
type AuthStore struct {
	db *bolt.DB
}

func NewAuthStore(st *store.Store) *AuthStore {
	return &AuthStore{db: st.DB()}
}

// Get returns the current auth, or nil if nothing is connected.
func (s *AuthStore) Get() (*Auth, error) {
	var auth Auth
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketBlueskyAuth)).Get(authKey)
		if len(raw) == 0 {
			return nil
		}
		found = true
		return json.Unmarshal(raw, &auth)
	})
	if err != nil || !found {
		return nil, err
	}
	return &auth, nil
}

func (s *AuthStore) Put(auth Auth) error {
	raw, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketBlueskyAuth)).Put(authKey, raw)
	})
}

func (s *AuthStore) Delete() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketBlueskyAuth)).Delete(authKey)
	})
}

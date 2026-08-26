package mastodon

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// referenceAuthKey is fixed — nitpub connects to exactly one reference
// instance at a time (see FederationConfig.ReferenceInstance).
var referenceAuthKey = []byte("current")

// ReferenceAuth is the admin's standing OAuth grant on the reference
// instance, used to resolve a shared post's remote permalink there.
type ReferenceAuth struct {
	Instance string `json:"instance"`
	Token    string `json:"token"`
}

// ReferenceAuthStore persists the single current ReferenceAuth, if any.
type ReferenceAuthStore struct {
	db *bolt.DB
}

func NewReferenceAuthStore(st *store.Store) *ReferenceAuthStore {
	return &ReferenceAuthStore{db: st.DB()}
}

// Get returns the current grant, or nil if nothing is connected.
func (s *ReferenceAuthStore) Get() (*ReferenceAuth, error) {
	var auth ReferenceAuth
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketReferenceAuth)).Get(referenceAuthKey)
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

func (s *ReferenceAuthStore) Put(auth ReferenceAuth) error {
	raw, err := json.Marshal(auth)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketReferenceAuth)).Put(referenceAuthKey, raw)
	})
}

func (s *ReferenceAuthStore) Delete() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketReferenceAuth)).Delete(referenceAuthKey)
	})
}

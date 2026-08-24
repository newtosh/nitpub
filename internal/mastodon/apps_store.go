package mastodon

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// AppStore persists per-domain OAuth app registrations (KTD3).
type AppStore struct {
	db *bolt.DB
}

func NewAppStore(st *store.Store) *AppStore {
	return &AppStore{db: st.DB()}
}

func (s *AppStore) get(domain string) (*AppRegistration, error) {
	var reg AppRegistration
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketCommentApps)).Get([]byte(domain))
		if len(raw) == 0 {
			return fmt.Errorf("no cached app for %s", domain)
		}
		return json.Unmarshal(raw, &reg)
	})
	if err != nil {
		return nil, err
	}
	return &reg, nil
}

func (s *AppStore) put(reg *AppRegistration) error {
	raw, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketCommentApps)).Put([]byte(reg.Domain), raw)
	})
}

package mastodon

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// AppStore persists per-domain OAuth app registrations (KTD3), scoped to a
// single bucket — callers registering apps for different purposes against
// the same instance domain (comment auth vs. the admin reference-instance
// connect flow) need separate buckets, since each app is registered with
// its own redirect_uri and Mastodon rejects an exchange whose redirect_uri
// doesn't match what the app was registered with.
type AppStore struct {
	db     *bolt.DB
	bucket string
}

func NewAppStore(st *store.Store) *AppStore {
	return &AppStore{db: st.DB(), bucket: store.BucketCommentApps}
}

// NewAppStoreIn is like NewAppStore but backed by an explicit bucket.
func NewAppStoreIn(st *store.Store, bucket string) *AppStore {
	return &AppStore{db: st.DB(), bucket: bucket}
}

func (s *AppStore) get(domain string) (*AppRegistration, error) {
	var reg AppRegistration
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(s.bucket)).Get([]byte(domain))
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
		return tx.Bucket([]byte(s.bucket)).Put([]byte(reg.Domain), raw)
	})
}

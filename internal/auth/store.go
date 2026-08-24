package auth

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// Store persists admin credentials and auth state in bbolt.
type Store struct {
	db *bolt.DB
}

func NewStore(st *store.Store) *Store {
	return &Store{db: st.DB()}
}

func (s *Store) AdminExists() (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketAdmin)).Get([]byte(adminKey))
		exists = len(raw) > 0
		return nil
	})
	return exists, err
}

func (s *Store) GetAdmin() (*AdminRecord, error) {
	var rec AdminRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketAdmin)).Get([]byte(adminKey))
		if len(raw) == 0 {
			return fmt.Errorf("admin not configured")
		}
		return json.Unmarshal(raw, &rec)
	})
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *Store) SaveAdmin(rec *AdminRecord) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketAdmin)).Put([]byte(adminKey), raw)
	})
}

func (s *Store) PutSession(sess *Session) error {
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketSessions)).Put([]byte(sess.ID), raw)
	})
}

func (s *Store) GetSession(id string) (*Session, error) {
	var sess Session
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketSessions)).Get([]byte(id))
		if len(raw) == 0 {
			return fmt.Errorf("session not found")
		}
		return json.Unmarshal(raw, &sess)
	})
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) DeleteSession(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketSessions)).Delete([]byte(id))
	})
}

func (s *Store) PutPending(p *PendingAuth) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketPendingAuth)).Put([]byte(p.Token), raw)
	})
}

func (s *Store) GetPending(token string) (*PendingAuth, error) {
	var p PendingAuth
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketPendingAuth)).Get([]byte(token))
		if len(raw) == 0 {
			return fmt.Errorf("pending auth not found")
		}
		return json.Unmarshal(raw, &p)
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) DeletePending(token string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketPendingAuth)).Delete([]byte(token))
	})
}

func (s *Store) PutEnrollToken(t *EnrollToken) error {
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketEnrollTokens)).Put([]byte(t.Token), raw)
	})
}

func (s *Store) GetEnrollToken(token string) (*EnrollToken, error) {
	var t EnrollToken
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketEnrollTokens)).Get([]byte(token))
		if len(raw) == 0 {
			return fmt.Errorf("enroll token not found")
		}
		return json.Unmarshal(raw, &t)
	})
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) MarkEnrollTokenUsed(token string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.BucketEnrollTokens))
		raw := b.Get([]byte(token))
		if len(raw) == 0 {
			return fmt.Errorf("enroll token not found")
		}
		var t EnrollToken
		if err := json.Unmarshal(raw, &t); err != nil {
			return err
		}
		t.Used = true
		out, err := json.Marshal(&t)
		if err != nil {
			return err
		}
		return b.Put([]byte(token), out)
	})
}

// CleanupExpired removes expired sessions, pending auth, and enroll tokens.
func (s *Store) CleanupExpired(now time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{store.BucketSessions, store.BucketPendingAuth, store.BucketEnrollTokens} {
			b := tx.Bucket([]byte(bucket))
			var toDelete [][]byte
			_ = b.ForEach(func(k, v []byte) error {
				var expires time.Time
				switch bucket {
				case store.BucketSessions:
					var sess Session
					if json.Unmarshal(v, &sess) == nil {
						expires = sess.ExpiresAt
					}
				case store.BucketPendingAuth:
					var p PendingAuth
					if json.Unmarshal(v, &p) == nil {
						expires = p.ExpiresAt
					}
				case store.BucketEnrollTokens:
					var t EnrollToken
					if json.Unmarshal(v, &t) == nil {
						expires = t.ExpiresAt
					}
				}
				if !expires.IsZero() && now.After(expires) {
					toDelete = append(toDelete, append([]byte(nil), k...))
				}
				return nil
			})
			for _, k := range toDelete {
				_ = b.Delete(k)
			}
		}
		return nil
	})
}

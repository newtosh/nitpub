package commentauth

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// Store persists comment sessions and pending comment-auth records in bbolt.
// Deliberately separate from internal/auth.Store — a comment session
// identifies an anonymous visitor's Mastodon handle, never an admin (KTD1).
type Store struct {
	db *bolt.DB
}

func NewStore(st *store.Store) *Store {
	return &Store{db: st.DB()}
}

func (s *Store) PutSession(sess *CommentSession) error {
	raw, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketCommentSessions)).Put([]byte(sess.ID), raw)
	})
}

func (s *Store) GetSession(id string) (*CommentSession, error) {
	var sess CommentSession
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketCommentSessions)).Get([]byte(id))
		if len(raw) == 0 {
			return fmt.Errorf("comment session not found")
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
		return tx.Bucket([]byte(store.BucketCommentSessions)).Delete([]byte(id))
	})
}

func (s *Store) PutPending(p *PendingCommentAuth) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketPendingCommentAuth)).Put([]byte(p.Token), raw)
	})
}

func (s *Store) GetPending(token string) (*PendingCommentAuth, error) {
	var p PendingCommentAuth
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketPendingCommentAuth)).Get([]byte(token))
		if len(raw) == 0 {
			return fmt.Errorf("pending comment auth not found")
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
		return tx.Bucket([]byte(store.BucketPendingCommentAuth)).Delete([]byte(token))
	})
}

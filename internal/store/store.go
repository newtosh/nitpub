package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
	bolterrors "go.etcd.io/bbolt/errors"
)

const defaultOpenTimeout = 3 * time.Second

// ErrDatabaseLocked is returned when another nitpub process holds the database.
var ErrDatabaseLocked = errors.New("database locked by another process")

const (
	dbFileName = "nitpub.db"

	bucketActor        = "actor"
	bucketOutbox       = "outbox"
	bucketInbox        = "inbox"
	bucketFollowers    = "followers"
	bucketKeyFetch     = "key_fetch_queue"
	bucketPosts        = "posts"
	bucketMeta         = "meta"
	bucketAdmin        = "admin"
	bucketSessions     = "sessions"
	bucketPendingAuth  = "pending_auth"
	bucketEnrollTokens = "enroll_tokens"

	bucketReplies           = "replies"
	bucketRepliesByActivity = "replies_by_activity"
	bucketRepliesByObjectID = "replies_by_object_id"
	bucketTrustedActors     = "trusted_actors"
	bucketBlockedActors     = "blocked_actors"

	bucketCommentSessions    = "comment_sessions"
	bucketPendingCommentAuth = "pending_comment_auth"
	bucketCommentApps        = "comment_apps"

	// bucketReferenceApps and bucketReferenceAuth back the admin-optional
	// "connect a reference Mastodon instance" flow used to resolve a
	// shared post's remote permalink. Separate buckets from the comment
	// flow's above: AppStore keys an OAuth app registration by domain
	// alone, and this flow registers its own app (different redirect_uri,
	// admin-held token rather than a per-visitor one) against potentially
	// the same instance domain a real commenter also uses.
	bucketReferenceApps = "reference_apps"
	bucketReferenceAuth = "reference_auth"

	// bucketBlueskyAuth backs the admin-optional Bluesky crosspost
	// connect flow (single connected account, no OAuth app registration
	// needed since Bluesky auth is app-password based).
	bucketBlueskyAuth = "bluesky_auth"
)

var requiredBuckets = []string{
	bucketActor,
	bucketOutbox,
	bucketInbox,
	bucketFollowers,
	bucketKeyFetch,
	bucketPosts,
	bucketMeta,
	bucketAdmin,
	bucketSessions,
	bucketPendingAuth,
	bucketEnrollTokens,
	bucketReplies,
	bucketRepliesByActivity,
	bucketRepliesByObjectID,
	bucketTrustedActors,
	bucketBlockedActors,
	bucketCommentSessions,
	bucketPendingCommentAuth,
	bucketCommentApps,
	bucketReferenceApps,
	bucketReferenceAuth,
	bucketBlueskyAuth,
}

// Store wraps a bbolt database with nitpub's bucket layout.
type Store struct {
	db   *bolt.DB
	path string
}

// Open creates the data directory if needed and opens (or creates) the database.
func Open(dataDir string) (*Store, error) {
	return OpenWithTimeout(dataDir, defaultOpenTimeout)
}

// OpenWithTimeout is like Open but waits up to timeout for an exclusive database lock.
func OpenWithTimeout(dataDir string, timeout time.Duration) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}

	path := filepath.Join(dataDir, dbFileName)
	opts := &bolt.Options{Timeout: timeout}
	db, err := bolt.Open(path, 0o600, opts)
	if err != nil {
		if errors.Is(err, bolterrors.ErrTimeout) {
			return nil, fmt.Errorf("%w: is the nitpub service running? stop it or use --offline", ErrDatabaseLocked)
		}
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db, path: path}
	if err := s.ensureBuckets(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range requiredBuckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

// DB exposes the underlying bbolt handle for domain packages.
func (s *Store) DB() *bolt.DB {
	return s.db
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.path
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// Bucket names for callers.
const (
	BucketActor        = bucketActor
	BucketOutbox       = bucketOutbox
	BucketInbox        = bucketInbox
	BucketFollowers    = bucketFollowers
	BucketKeyFetch     = bucketKeyFetch
	BucketPosts        = bucketPosts
	BucketMeta         = bucketMeta
	BucketAdmin        = bucketAdmin
	BucketSessions     = bucketSessions
	BucketPendingAuth  = bucketPendingAuth
	BucketEnrollTokens = bucketEnrollTokens

	BucketReplies           = bucketReplies
	BucketRepliesByActivity = bucketRepliesByActivity
	BucketRepliesByObjectID = bucketRepliesByObjectID
	BucketTrustedActors     = bucketTrustedActors
	BucketBlockedActors     = bucketBlockedActors

	BucketCommentSessions    = bucketCommentSessions
	BucketPendingCommentAuth = bucketPendingCommentAuth
	BucketCommentApps        = bucketCommentApps

	BucketReferenceApps = bucketReferenceApps
	BucketReferenceAuth = bucketReferenceAuth

	BucketBlueskyAuth = bucketBlueskyAuth
)

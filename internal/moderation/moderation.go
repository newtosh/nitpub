// Package moderation gates inbound ActivityPub replies behind a pending
// queue, with trusted/blocked actor lists that bypass or short-circuit it.
package moderation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/microcosm-cc/bluemonday"
	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// Status is the moderation state of a stored reply.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	// StatusSkipped is a lightweight third action distinct from Rejected: set
	// aside without publishing, without also blocking the actor. Like
	// Approved/Rejected, reversible back to Pending (see SetReplyStatus).
	StatusSkipped Status = "skipped"
)

const (
	maxContentBytes    = 5000
	maxAuthorNameBytes = 256
	maxObjectIDBytes   = 2000

	// preSanitizeContentBytes bounds the raw, unsanitized input handed to the
	// HTML parser — generous enough that legitimate content sanitizing down
	// to maxContentBytes is never affected, but bounded so a remote actor
	// can't force unbounded HTML-parse cost with an arbitrarily large body.
	preSanitizeContentBytes = maxContentBytes * 4

	// orderingWidth matches the max decimal digit count of a positive int64
	// (time.Now().UnixNano()), so real-time and backfill ordering values sort
	// correctly against each other under bbolt's lexical (byte) key comparison.
	orderingWidth = 19
)

// replyHTMLPolicy retains a minimal safe HTML subset for reply content,
// mirroring the allow-list shape of outbox's mastodonHTMLPolicy
// (internal/outbox/federation_content.go) adapted for inbound reply content.
var replyHTMLPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"p", "br", "span",
		"a", "del", "pre", "code",
		"em", "strong", "b", "i", "u",
		"ul", "ol", "li", "blockquote",
	)
	p.AllowAttrs("href", "rel", "class").OnElements("a")
	p.AllowAttrs("class").OnElements("span")
	p.AllowStandardURLs() // rejects non-http(s)/mailto schemes such as javascript:
	return p
}()

// plainTextPolicy strips all HTML, used for the reply author's display name
// — display names are not expected to carry markup.
var plainTextPolicy = bluemonday.StrictPolicy()

// Reply is a single inbound ActivityPub reply, gated by moderation status.
type Reply struct {
	Key        string `json:"key"`
	ActivityID string `json:"activity_id"`
	PostSlug   string `json:"post_slug"`
	Actor      string `json:"actor"`
	Content    string `json:"content"`
	AuthorName string `json:"author_name"`
	// URL is the reply's browsable origin link (e.g. its Mastodon status
	// page), used to let readers open the original post. Best-effort: may
	// be empty for older backfilled entries or non-conforming senders.
	URL string `json:"url"`
	// AvatarURL is the actor's profile icon, when the sender exposed one.
	// Best-effort like URL: may be empty, and display is gated by the
	// admin-controlled show_avatars_default site setting.
	AvatarURL string `json:"avatar_url"`
	// ObjectID is this reply's own ActivityPub object id (obj["id"] at
	// ingestion) — distinct from URL (which prefers the human-facing "url"
	// field). ObjectID is what a further reply's inReplyTo references, so
	// it's the join key for nested-reply threading (R-thread1).
	ObjectID string `json:"object_id"`
	// InReplyTo is the raw inReplyTo target: either the root post's URL
	// (top-level reply) or another reply's ObjectID (nested reply). Empty
	// only for pre-threading entries migrated before this field existed.
	InReplyTo string `json:"in_reply_to"`
	// Nested is true when this reply targets another reply rather than the
	// post itself — set explicitly by the caller at resolution time (which
	// already knows definitively which path matched), not inferred later
	// from InReplyTo's shape.
	Nested bool `json:"nested"`
	// ParentActor and ParentAuthorName are a denormalized snapshot of the
	// immediate parent reply's identity, captured at resolution time (the
	// caller already has the parent record in hand) — set only when Nested,
	// so a triaging admin can see who a nested reply is actually addressing
	// without a second lookup.
	ParentActor      string `json:"parent_actor,omitempty"`
	ParentAuthorName string `json:"parent_author_name,omitempty"`
	ReceivedAt       string `json:"received_at"`
	Status           Status `json:"status"`
	// Verified is true when Actor was set from the HTTP-signature-verified
	// remoteActor at live ingestion (KTD3), false for entries migrated by
	// the one-time backfill — whose raw stored activity never carried a
	// verified signature, only the unverified activity["actor"] body field.
	Verified bool `json:"verified"`

	// orderingValue overrides the ordering segment of the composite key when
	// set (used by tests and by the backfill path's monotonic counter). Not
	// persisted.
	orderingValue string `json:"-"`
}

// TrustedActor and BlockedActor are stored as simple presence markers.
type TrustedActor struct {
	Actor string `json:"actor"`
}

type BlockedActor struct {
	Actor string `json:"actor"`
}

// Service wraps moderation storage over the shared bbolt database.
type Service struct {
	db *bolt.DB
}

func New(st *store.Store) *Service {
	return &Service{db: st.DB()}
}

func compositeKey(postSlug, orderingValue, activityID string) string {
	sum := sha256.Sum256([]byte(activityID))
	return fmt.Sprintf("%s:%s:%s", postSlug, orderingValue, hex.EncodeToString(sum[:]))
}

func sanitizeReply(r *Reply) {
	// Bound the raw input before it ever reaches the HTML parser — an
	// unbounded body would let a remote actor force arbitrarily expensive
	// sanitizer parse cost on the synchronous inbox path. The bound is
	// generous (4x the final cap) so it never affects legitimately-sized
	// content; only pathological oversized input hits it.
	if len(r.Content) > preSanitizeContentBytes {
		r.Content = truncateBytes(r.Content, preSanitizeContentBytes)
	}
	// Sanitize before the final truncation, not after: truncating raw
	// (unsanitized) input first can cut mid-tag and hand the sanitizer
	// malformed markup, producing unpredictable output. Sanitizing first
	// always gives the policy well-formed input; only the resulting safe
	// output is then bounded to the size cap.
	r.Content = replyHTMLPolicy.Sanitize(r.Content)
	if len(r.Content) > maxContentBytes {
		r.Content = truncateBytes(r.Content, maxContentBytes)
	}
	r.AuthorName = plainTextPolicy.Sanitize(r.AuthorName)
	if len(r.AuthorName) > maxAuthorNameBytes {
		r.AuthorName = truncateBytes(r.AuthorName, maxAuthorNameBytes)
	}
	r.ParentAuthorName = plainTextPolicy.Sanitize(r.ParentAuthorName)
	if len(r.ParentAuthorName) > maxAuthorNameBytes {
		r.ParentAuthorName = truncateBytes(r.ParentAuthorName, maxAuthorNameBytes)
	}
	if !isHTTPURL(r.URL) {
		r.URL = ""
	}
	if !isHTTPURL(r.AvatarURL) {
		r.AvatarURL = ""
	}
	if len(r.ObjectID) > maxObjectIDBytes {
		r.ObjectID = truncateBytes(r.ObjectID, maxObjectIDBytes)
	}
	if len(r.InReplyTo) > maxObjectIDBytes {
		r.InReplyTo = truncateBytes(r.InReplyTo, maxObjectIDBytes)
	}
}

// isHTTPURL rejects everything but http(s) — in particular javascript: and
// other schemes that would be unsafe to render as a clickable link.
func isHTTPURL(s string) bool {
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	b := []byte(s)[:max]
	// Avoid truncating in the middle of a multi-byte UTF-8 rune.
	for len(b) > 0 && !isUTF8Boundary(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return string(b)
}

func isUTF8Boundary(b byte) bool {
	// Continuation bytes have the high bits 10xxxxxx.
	return b&0xC0 != 0x80
}

// SaveReply sanitizes, truncates, and persists a reply, using time.Now()
// (zero-padded to orderingWidth) as the ordering value unless the caller
// (tests, backfill) already set one via the unexported orderingValue field.
func (s *Service) SaveReply(r Reply) error {
	if r.orderingValue == "" {
		r.orderingValue = fmt.Sprintf("%0*d", orderingWidth, time.Now().UnixNano())
	}
	return s.saveReplyWithOrdering(r)
}

// saveReplyWithOrdering is SaveReply with an explicit, already-set ordering
// value — used directly by tests exercising key-collision behavior and by
// the backfill path's monotonic counter (KTD2).
func (s *Service) saveReplyWithOrdering(r Reply) error {
	sanitizeReply(&r)
	key := compositeKey(r.PostSlug, r.orderingValue, r.ActivityID)
	r.Key = key
	if r.ReceivedAt == "" {
		r.ReceivedAt = time.Now().UTC().Format(time.RFC3339)
	}

	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		idx := tx.Bucket([]byte(store.BucketRepliesByActivity))
		replies := tx.Bucket([]byte(store.BucketReplies))
		objIdx := tx.Bucket([]byte(store.BucketRepliesByObjectID))

		// Duplicate delivery of an already-stored activity: no-op, regardless
		// of what key a fresh (later-timestamped) computation would produce.
		if idx.Get([]byte(r.ActivityID)) != nil {
			return nil
		}

		// Cross-activity collision: this exact composite key is already
		// occupied by a different activity ID. Error rather than silently
		// overwrite the earlier entry (KTD2).
		if existing := replies.Get([]byte(key)); existing != nil {
			return fmt.Errorf("moderation: composite key %q already occupied by a different activity (saving %q): possible key collision", key, r.ActivityID)
		}

		if err := replies.Put([]byte(key), raw); err != nil {
			return err
		}
		if r.ObjectID != "" {
			// Best-effort join index for nested-reply lookups (findByObjectID) —
			// not the source of truth, so a rare id collision just means the
			// later write wins as the resolvable parent.
			if err := objIdx.Put([]byte(r.ObjectID), []byte(key)); err != nil {
				return err
			}
		}
		return idx.Put([]byte(r.ActivityID), []byte(key))
	})
}

// FindByObjectID resolves a reply's own ActivityPub object id (as referenced
// by a further reply's inReplyTo) to the stored Reply, if we have one. Used
// to thread a reply-to-a-reply onto the same post and inherit its parent's
// PostSlug — returns (nil, nil) when not found, not an error, since an
// unresolvable parent (reply to something we never saw) is an expected,
// silently-dropped case, not a fault.
func (s *Service) FindByObjectID(objectID string) (*Reply, error) {
	if objectID == "" {
		return nil, nil
	}
	var found *Reply
	err := s.db.View(func(tx *bolt.Tx) error {
		key := tx.Bucket([]byte(store.BucketRepliesByObjectID)).Get([]byte(objectID))
		if key == nil {
			return nil
		}
		raw := tx.Bucket([]byte(store.BucketReplies)).Get(key)
		if raw == nil {
			return nil
		}
		var r Reply
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		found = &r
		return nil
	})
	return found, err
}

// RepliesForPost returns all replies for a post, ordered by receipt time.
func (s *Service) RepliesForPost(slug string) ([]Reply, error) {
	prefix := []byte(slug + ":")
	out := make([]Reply, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(store.BucketReplies)).Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var r Reply
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			out = append(out, r)
		}
		return nil
	})
	return out, err
}

// ApprovedRepliesForPost returns only approved replies for a post, ordered
// by receipt time — the shape the public API (U4) serves.
func (s *Service) ApprovedRepliesForPost(slug string) ([]Reply, error) {
	all, err := s.RepliesForPost(slug)
	if err != nil {
		return nil, err
	}
	out := make([]Reply, 0, len(all))
	for _, r := range all {
		if r.Status == StatusApproved {
			out = append(out, r)
		}
	}
	return out, nil
}

// ApprovedReplyCount returns the number of approved replies for a post —
// for a reader-facing "N replies" indicator on post lists, where fetching
// full reply bodies per post would be wasteful.
func (s *Service) ApprovedReplyCount(slug string) (int, error) {
	replies, err := s.ApprovedRepliesForPost(slug)
	if err != nil {
		return 0, err
	}
	return len(replies), nil
}

// PendingReplies returns all pending replies across every post, for the
// admin queue (U3).
func (s *Service) PendingReplies() ([]Reply, error) {
	return s.repliesWhere(func(r Reply) bool { return r.Status == StatusPending })
}

// ReviewedReplies returns every already-actioned reply (approved, rejected,
// or skipped) across every post, for the admin queue's "Reviewed" view —
// where an admin can find and revert a past decision.
func (s *Service) ReviewedReplies() ([]Reply, error) {
	return s.repliesWhere(func(r Reply) bool { return r.Status != StatusPending })
}

func (s *Service) repliesWhere(match func(Reply) bool) ([]Reply, error) {
	out := make([]Reply, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketReplies)).ForEach(func(_, v []byte) error {
			var r Reply
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			if match(r) {
				out = append(out, r)
			}
			return nil
		})
	})
	return out, err
}

// ErrReplyNotFound is returned by SetReplyStatus when the key has no stored
// reply.
var ErrReplyNotFound = errors.New("moderation: reply not found")

// ErrInvalidStatusTransition is returned by SetReplyStatus for any
// transition other than pending -> {approved, rejected, skipped} (an action)
// or {approved, rejected, skipped} -> pending (a revert) — lateral moves
// between two actioned states must go through pending first, so "revert"
// always means the same thing everywhere it's offered in the UI.
var ErrInvalidStatusTransition = errors.New("moderation: invalid reply status transition")

// SetReplyStatus transitions a reply (by composite key) between pending and
// an actioned status (approved, rejected, skipped) in either direction —
// action from pending, or revert back to pending from any actioned status.
func (s *Service) SetReplyStatus(key string, status Status) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.BucketReplies))
		raw := b.Get([]byte(key))
		if raw == nil {
			return ErrReplyNotFound
		}
		var r Reply
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		if !validStatusTransition(r.Status, status) {
			return ErrInvalidStatusTransition
		}
		r.Status = status
		updated, err := json.Marshal(r)
		if err != nil {
			return err
		}
		return b.Put([]byte(key), updated)
	})
}

func validStatusTransition(from, to Status) bool {
	if from == StatusPending {
		return to == StatusApproved || to == StatusRejected || to == StatusSkipped
	}
	return to == StatusPending
}

// IsTrusted reports whether an actor URI is on the trusted (allow-list) list.
func (s *Service) IsTrusted(actor string) (bool, error) {
	return s.hasActor(store.BucketTrustedActors, actor)
}

// IsBlocked reports whether an actor URI is on the blocked list.
func (s *Service) IsBlocked(actor string) (bool, error) {
	return s.hasActor(store.BucketBlockedActors, actor)
}

// ClassifyActor reports trusted/blocked status in a single transaction —
// used on the ingestion hot path (KTD3) instead of two separate IsBlocked
// and IsTrusted calls.
func (s *Service) ClassifyActor(actor string) (trusted, blocked bool, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		blocked = tx.Bucket([]byte(store.BucketBlockedActors)).Get([]byte(actor)) != nil
		trusted = tx.Bucket([]byte(store.BucketTrustedActors)).Get([]byte(actor)) != nil
		return nil
	})
	return trusted, blocked, err
}

func (s *Service) hasActor(bucket, actor string) (bool, error) {
	var found bool
	err := s.db.View(func(tx *bolt.Tx) error {
		found = tx.Bucket([]byte(bucket)).Get([]byte(actor)) != nil
		return nil
	})
	return found, err
}

// AddTrusted adds an actor URI to the trusted (allow-list) list.
func (s *Service) AddTrusted(actor string) error { return s.putActor(store.BucketTrustedActors, actor) }

// RemoveTrusted removes an actor URI from the trusted list.
func (s *Service) RemoveTrusted(actor string) error {
	return s.deleteActor(store.BucketTrustedActors, actor)
}

// AddBlocked adds an actor URI to the blocked list.
func (s *Service) AddBlocked(actor string) error { return s.putActor(store.BucketBlockedActors, actor) }

// RemoveBlocked removes an actor URI from the blocked list.
func (s *Service) RemoveBlocked(actor string) error {
	return s.deleteActor(store.BucketBlockedActors, actor)
}

func (s *Service) putActor(bucket, actor string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Put([]byte(actor), []byte(`{}`))
	})
}

func (s *Service) deleteActor(bucket, actor string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Delete([]byte(actor))
	})
}

// ListTrusted returns all trusted actor URIs.
func (s *Service) ListTrusted() ([]string, error) { return s.listActors(store.BucketTrustedActors) }

// ListBlocked returns all blocked actor URIs.
func (s *Service) ListBlocked() ([]string, error) { return s.listActors(store.BucketBlockedActors) }

func (s *Service) listActors(bucket string) ([]string, error) {
	out := make([]string, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).ForEach(func(k, _ []byte) error {
			out = append(out, string(k))
			return nil
		})
	})
	return out, err
}

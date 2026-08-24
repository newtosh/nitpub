package moderation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/store"
)

const backfillMarkerKey = "moderation_backfill_v1_done"

// RunBackfillOnce migrates pre-existing raw reply activities — stored before
// this feature existed, in the generic inbox bucket — into the pending
// moderation queue. It is idempotent and safe to call on every startup: a
// completed pass sets a meta marker and short-circuits; an incomplete pass
// (process died mid-scan) leaves the marker unset, so a later call resumes
// safely, re-processing already-migrated entries as no-ops via SaveReply's
// activity-ID deduplication (KTD6).
//
// Each migrated entry is written in its own bolt.Update transaction (mirroring
// SaveReply's per-call shape), never one transaction spanning the whole scan,
// so the single-writer lock is held only per-record — this must run
// synchronously before the HTTP listener starts accepting connections
// (KTD6), which callers ensure by invoking it before the server begins
// serving.
// postExists reports whether a post slug resolves to a known local post —
// callers pass a closure wrapping the outbox service's own legacy-base-URL-
// tolerant lookup (outbox.Service.GetPost), so backfill recognizes replies
// to posts whose stored IRI predates a domain migration, not just posts
// under the current baseURL.
func (s *Service) RunBackfillOnce(postExists func(slug string) bool) error {
	done, err := s.backfillMarkerSet()
	if err != nil {
		return fmt.Errorf("moderation: check backfill marker: %w", err)
	}
	if done {
		return nil
	}

	entries, err := s.scanInboxForReplies(postExists)
	if err != nil {
		return fmt.Errorf("moderation: scan inbox for replies: %w", err)
	}

	// Seed the ordering counter once at pass start (KTD2) so migrated
	// replies for the same post retain stable relative order, rather than
	// depending on remote-supplied (untrusted) published timestamps.
	counter := time.Now().UnixNano()
	for _, e := range entries {
		r := Reply{
			ActivityID:    e.activityID,
			PostSlug:      e.postSlug,
			Actor:         e.actor,
			Content:       e.content,
			URL:           e.url,
			ObjectID:      e.objectID,
			InReplyTo:     e.inReplyTo,
			Status:        StatusPending,
			Verified:      false,
			orderingValue: fmt.Sprintf("%0*d", orderingWidth, counter),
		}
		if err := s.saveReplyWithOrdering(r); err != nil {
			return fmt.Errorf("moderation: backfill entry %q: %w", e.activityID, err)
		}
		counter++
	}

	// Write the marker only after the full pass completes without error, in
	// a transaction separate from and after the last SaveReply call (KTD6) —
	// never before or during the scan.
	return s.setBackfillMarker()
}

func (s *Service) backfillMarkerSet() (bool, error) {
	var set bool
	err := s.db.View(func(tx *bolt.Tx) error {
		set = tx.Bucket([]byte(store.BucketMeta)).Get([]byte(backfillMarkerKey)) != nil
		return nil
	})
	return set, err
}

func (s *Service) setBackfillMarker() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketMeta)).Put([]byte(backfillMarkerKey), []byte("1"))
	})
}

type rawReplyEntry struct {
	activityID string
	postSlug   string
	actor      string
	content    string
	url        string
	objectID   string
	inReplyTo  string
}

// scanInboxForReplies reads the entire inbox bucket in one view transaction
// (read-only; safe to run alongside concurrent writers) and filters to
// activities whose object.inReplyTo targets a local post. Unlike live
// ingestion's exact-baseURL-prefix check (safe there because remote software
// always addresses replies to our current domain), backfilled entries can
// predate a domain migration: their raw inReplyTo may carry a stale base
// URL that no longer matches cfg.BaseURL even though the post itself still
// exists (its own stored IRI already gets rewritten on migration — see
// outbox.Service.RewritePostBaseURLs). So this extracts the slug from any
// "/posts/<slug>"-shaped URL and confirms it resolves to a known local post
// via postExists, rather than requiring an exact current-baseURL prefix.
func (s *Service) scanInboxForReplies(postExists func(slug string) bool) ([]rawReplyEntry, error) {
	var entries []rawReplyEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketInbox)).ForEach(func(k, v []byte) error {
			var activity map[string]any
			if err := json.Unmarshal(v, &activity); err != nil {
				return nil // skip unparseable entries rather than fail the whole pass
			}
			obj, _ := activity["object"].(map[string]any)
			if obj == nil {
				return nil
			}
			inReply, ok := obj["inReplyTo"].(string)
			if !ok || !strings.Contains(inReply, "/posts/") {
				return nil
			}
			slug := outbox.PostSlug(inReply)
			if postExists == nil || !postExists(slug) {
				return nil
			}
			id, _ := activity["id"].(string)
			if id == "" {
				id = string(k)
			}
			content, _ := obj["content"].(string)
			objectID, _ := obj["id"].(string)
			entries = append(entries, rawReplyEntry{
				activityID: id,
				postSlug:   slug,
				actor:      backfillActorIRI(activity["actor"]),
				content:    content,
				url:        backfillObjectURL(obj),
				objectID:   objectID,
				inReplyTo:  inReply,
			})
			return nil
		})
	})
	return entries, err
}

// backfillObjectURL mirrors inbox.replyObjectURL for the raw stored
// activities the backfill pass reads directly from the inbox bucket.
func backfillObjectURL(obj map[string]any) string {
	if u, ok := obj["url"].(string); ok && u != "" {
		return u
	}
	if id, ok := obj["id"].(string); ok {
		return id
	}
	return ""
}

func backfillActorIRI(v any) string {
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

package outbox

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// MarshalActivityPub emits ActivityPub JSON-LD with a proper "@context" key.
// go-ap/jsonld's own encoder only ever writes the field name "context" (no
// "@") — verified against its source, not a config option we're missing.
// Inbox-delivered activities get away with this because Mastodon's inbox
// processing is lenient about it, but a remote server independently
// fetching an object by URL (e.g. Mastodon's resolve=true search hitting
// GET /posts/{id}) does strict JSON-LD parsing and silently fails to
// recognize a document without a literal "@context". Mirrors
// internal/actor/actor.go's marshalActivityPub — same underlying gap,
// shared fix.
func MarshalActivityPub(v any) ([]byte, error) {
	body, err := jsonld.Marshal(v)
	if err != nil {
		return nil, err
	}
	body = bytes.Replace(body, []byte(`"context":`), []byte(`"@context":`), 1)
	return body, nil
}

// Kind distinguishes note vs article posts.
type Kind string

const (
	KindNote    Kind = "note"
	KindArticle Kind = "article"
)

// FederationState records whether a post was shared to ActivityPub followers.
type FederationState struct {
	Shared   bool       `json:"shared"`
	SharedAt *time.Time `json:"shared_at,omitempty"`
	Error    string     `json:"error,omitempty"`
	// RemoteURL is the post's local permalink on the configured reference
	// Mastodon instance (resolved via that instance's search API at share
	// time) — one instance's mirror of the post, not a universal fediverse
	// URL. Empty when resolution hasn't run or failed.
	RemoteURL string `json:"remote_url,omitempty"`
}

// Status values for Post.Status. An absent/empty Status means published —
// every pre-existing stored post has no status key, so this keeps them
// published with no migration or backfill required.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

// Post is a stored native post.
type Post struct {
	ID         string           `json:"id"`
	Kind       Kind             `json:"kind"`
	Title      *string          `json:"title,omitempty"`
	Content    string           `json:"content"`
	Status     string           `json:"status,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  *time.Time       `json:"updated_at,omitempty"`
	Federation *FederationState `json:"federation,omitempty"`
}

// IsDraft reports whether the post is an unpublished draft. An absent/empty
// Status is treated as published (KTD1).
func (p Post) IsDraft() bool {
	return p.Status == StatusDraft
}

// Service manages outbox storage and federated representations.
type Service struct {
	db       *bolt.DB
	baseURL  string
	actorIRI string
}

func New(st *store.Store, baseURL, actorIRI string) *Service {
	return &Service{db: st.DB(), baseURL: baseURL, actorIRI: actorIRI}
}

// BaseURL returns the configured site origin for permalinks and feeds.
func (s *Service) BaseURL() string { return s.baseURL }

func (s *Service) CreatePost(kind Kind, content string) (*Post, *vocab.Create, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil, fmt.Errorf("content is required")
	}
	if kind != KindNote && kind != KindArticle {
		return nil, nil, fmt.Errorf("invalid kind %q", kind)
	}
	if kind == KindNote {
		if err := ValidateNoteContent(content); err != nil {
			return nil, nil, err
		}
	}

	id := fmt.Sprintf("%s/posts/%s", s.baseURL, uuid.NewString())
	post := &Post{ID: id, Kind: kind, Content: content, CreatedAt: time.Now().UTC()}
	activity := s.buildCreateActivity(*post, post.CreatedAt)

	rawPost, err := json.Marshal(post)
	if err != nil {
		return nil, nil, err
	}
	rawActivity, err := json.Marshal(activity)
	if err != nil {
		return nil, nil, err
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket([]byte(store.BucketPosts)).Put([]byte(id), rawPost); err != nil {
			return err
		}
		return tx.Bucket([]byte(store.BucketOutbox)).Put([]byte(id), rawActivity)
	})
	if err != nil {
		return nil, nil, err
	}
	return post, activity, nil
}

func (s *Service) nativeObject(post *Post) *vocab.Object {
	obj := vocab.ObjectNew(vocab.NoteType)
	if post.Kind == KindArticle {
		obj = vocab.ObjectNew(vocab.ArticleType)
	}
	obj.ID = vocab.IRI(post.ID)
	obj.Published = post.CreatedAt
	obj.AttributedTo = vocab.IRI(s.actorIRI)
	obj.URL = vocab.IRI(s.Permalink(*post))
	// The wrapping Create activity already declares this, but a remote
	// server fetching the object directly (e.g. Mastodon's resolve=true
	// search hitting /posts/{id}) never sees that activity — it only sees
	// this bare object. Without its own "to", there's nothing telling the
	// fetcher the post is public, so it can't be recognized as a
	// resolvable public status.
	obj.To = vocab.ItemCollection{vocab.PublicNS}

	if post.Kind == KindNote {
		html, err := NoteFederationHTML(post.Content)
		if err != nil {
			html = mastodonHTMLPolicy.Sanitize(post.Content)
		}
		obj.Content = vocab.NaturalLanguageValuesNew()
		_ = obj.Content.Append(vocab.NilLangRef, vocab.Content(html))
		src := vocab.NaturalLanguageValuesNew()
		_ = src.Append(vocab.NilLangRef, vocab.Content(post.Content))
		obj.Source = vocab.Source{
			Content:   src,
			MediaType: vocab.MimeType("text/markdown"),
		}
		return obj
	}

	obj.Content = vocab.NaturalLanguageValuesNew()
	_ = obj.Content.Append(vocab.NilLangRef, vocab.Content(post.Content))
	return obj
}

// buildCreateActivity constructs the ActivityPub Create activity for a post
// using an explicit publish timestamp rather than the post's own CreatedAt
// (KTD8/KTD3) — shared by CreatePost's direct-publish path and PublishDraft,
// so a draft that sat for days federates with its actual publish time, not
// its original autosave/creation moment. Takes post by value so the caller's
// stored CreatedAt is never mutated.
func (s *Service) buildCreateActivity(post Post, published time.Time) *vocab.Create {
	post.CreatedAt = published
	obj := s.nativeObject(&post)
	activity := vocab.CreateNew(vocab.IRI(post.ID+"/activity"), obj)
	activity.Actor = vocab.IRI(s.actorIRI)
	activity.To = vocab.ItemCollection{vocab.PublicNS}
	activity.Published = published
	activity.Context = vocab.ActivityBaseURI
	return activity
}

// combineArticleContent mirrors web/src/lib/contentKinds.ts's function of the
// same name — an article's canonical stored shape embeds the title as the
// first line, a blank line, then the body (KTD7). Used to reconstruct that
// shape from a draft's separate Title/Content fields before publish.
func combineArticleContent(title, body string) string {
	t := strings.TrimSpace(title)
	b := strings.TrimSpace(body)
	if t == "" && b == "" {
		return ""
	}
	if t == "" {
		return b
	}
	if b == "" {
		return t
	}
	return t + "\n\n" + b
}

// PublishDraft transitions an existing draft to published, building its
// Create activity for the first time and writing it to the outbox. Reuses
// the same row (no second Post created) — R5.
func (s *Service) PublishDraft(slug string) (*Post, *vocab.Create, error) {
	var post Post
	var activity *vocab.Create
	err := s.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(store.BucketPosts))
		key, err := s.lookupPostKeyTx(tx, slug)
		if err != nil {
			return err
		}
		raw := postsBucket.Get(key)
		if raw == nil {
			return fmt.Errorf("post not found")
		}
		if err := json.Unmarshal(raw, &post); err != nil {
			return err
		}
		if !post.IsDraft() {
			return fmt.Errorf("post is already published")
		}

		content := post.Content
		if post.Kind == KindArticle {
			title := ""
			if post.Title != nil {
				title = *post.Title
			}
			content = combineArticleContent(title, post.Content)
		}
		content = strings.TrimSpace(content)
		if content == "" {
			return fmt.Errorf("content is required")
		}
		if post.Kind == KindNote {
			if err := ValidateNoteContent(content); err != nil {
				return err
			}
		}

		now := time.Now().UTC()
		post.Content = content
		post.Title = nil
		// Clear to the zero value rather than StatusPublished — absence
		// means published (KTD1), matching CreatePost's shape so a post's
		// JSON representation doesn't depend on whether it went through the
		// draft path.
		post.Status = ""
		post.UpdatedAt = &now

		activity = s.buildCreateActivity(post, now)

		rawPost, err := json.Marshal(post)
		if err != nil {
			return err
		}
		if err := postsBucket.Put([]byte(post.ID), rawPost); err != nil {
			return err
		}
		rawActivity, err := json.Marshal(activity)
		if err != nil {
			return err
		}
		return tx.Bucket([]byte(store.BucketOutbox)).Put([]byte(post.ID), rawActivity)
	})
	if err != nil {
		return nil, nil, err
	}
	return &post, activity, nil
}

// PostSlug returns the UUID segment of a post IRI for permalink URLs.
func PostSlug(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

func postIDSuffix(slug string) string {
	return "/posts/" + slug
}

// lookupPostKey finds the bbolt key for a post slug (current or legacy base URL).
func (s *Service) lookupPostKey(slug string) ([]byte, error) {
	var found []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		key, err := s.lookupPostKeyTx(tx, slug)
		if err != nil {
			return err
		}
		found = key
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// lookupPostKeyTx is lookupPostKey's transaction-scoped variant, for callers
// that need the read and a subsequent write inside the same db.Update — e.g.
// SaveDraft's already-published check must not race a concurrent PublishDraft.
func (s *Service) lookupPostKeyTx(tx *bolt.Tx, slug string) ([]byte, error) {
	if slug == "" {
		return nil, fmt.Errorf("post not found")
	}
	bucket := tx.Bucket([]byte(store.BucketPosts))
	if bucket == nil {
		return nil, fmt.Errorf("post not found")
	}
	preferred := []byte(s.baseURL + postIDSuffix(slug))
	if bucket.Get(preferred) != nil {
		return preferred, nil
	}
	suffix := postIDSuffix(slug)
	var found []byte
	err := bucket.ForEach(func(k, _ []byte) error {
		if strings.HasSuffix(string(k), suffix) {
			found = append([]byte(nil), k...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("post not found")
	}
	return found, nil
}

// SaveDraft persists partial title/content as a draft Post, without the
// validations that gate Publish and without touching BucketOutbox (KTD2).
// existingSlug empty creates a new draft; non-empty updates the existing one
// in place. Rejects if the target slug is already published (R2).
func (s *Service) SaveDraft(kind Kind, title, content, existingSlug string) (*Post, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" && content == "" {
		return nil, fmt.Errorf("title or content is required")
	}
	if kind != KindNote && kind != KindArticle {
		return nil, fmt.Errorf("invalid kind %q", kind)
	}

	var titlePtr *string
	if title != "" {
		titlePtr = &title
	}

	var result Post
	err := s.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(store.BucketPosts))
		now := time.Now().UTC()

		if existingSlug == "" {
			result = Post{
				ID:        fmt.Sprintf("%s/posts/%s", s.baseURL, uuid.NewString()),
				Kind:      kind,
				Title:     titlePtr,
				Content:   content,
				Status:    StatusDraft,
				CreatedAt: now,
				UpdatedAt: &now,
			}
		} else {
			key, err := s.lookupPostKeyTx(tx, existingSlug)
			if err != nil {
				return err
			}
			raw := postsBucket.Get(key)
			if raw == nil {
				return fmt.Errorf("post not found")
			}
			if err := json.Unmarshal(raw, &result); err != nil {
				return err
			}
			if !result.IsDraft() {
				return fmt.Errorf("post is already published")
			}
			result.Kind = kind
			result.Title = titlePtr
			result.Content = content
			result.UpdatedAt = &now
		}

		rawPost, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return postsBucket.Put([]byte(result.ID), rawPost)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// RewritePostBaseURLs rekeys posts and outbox activities to the current baseURL.
func (s *Service) RewritePostBaseURLs() (int, error) {
	var n int
	err := s.db.Update(func(tx *bolt.Tx) error {
		posts := tx.Bucket([]byte(store.BucketPosts))
		outboxB := tx.Bucket([]byte(store.BucketOutbox))
		if posts == nil {
			return nil
		}

		type item struct {
			oldKey string
			post   Post
		}
		var items []item
		if err := posts.ForEach(func(k, v []byte) error {
			var p Post
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			slug := PostSlug(p.ID)
			newID := s.baseURL + postIDSuffix(slug)
			if string(k) != newID || p.ID != newID {
				items = append(items, item{oldKey: string(k), post: p})
			}
			return nil
		}); err != nil {
			return err
		}

		for _, it := range items {
			slug := PostSlug(it.post.ID)
			newID := s.baseURL + postIDSuffix(slug)
			it.post.ID = newID

			rawPost, err := json.Marshal(&it.post)
			if err != nil {
				return err
			}

			obj := s.nativeObject(&it.post)
			activity := vocab.CreateNew(vocab.IRI(newID+"/activity"), obj)
			activity.Actor = vocab.IRI(s.actorIRI)
			activity.To = vocab.ItemCollection{vocab.PublicNS}
			activity.Published = it.post.CreatedAt
			activity.Context = vocab.ActivityBaseURI

			rawActivity, err := json.Marshal(activity)
			if err != nil {
				return err
			}

			if err := posts.Delete([]byte(it.oldKey)); err != nil {
				return err
			}
			if outboxB != nil {
				_ = outboxB.Delete([]byte(it.oldKey))
			}
			if err := posts.Put([]byte(newID), rawPost); err != nil {
				return err
			}
			if outboxB != nil {
				if err := outboxB.Put([]byte(newID), rawActivity); err != nil {
					return err
				}
			}
			n++
		}
		return nil
	})
	return n, err
}

// Permalink returns the public HTML URL for a post.
func (s *Service) Permalink(p Post) string {
	return s.baseURL + "/p/" + PostSlug(p.ID)
}

// GetPost loads a post by its slug (UUID segment of the stored IRI).
// GetPost returns a post by slug regardless of status. Callers reachable by
// unauthenticated requests must use GetPublishedPost instead (R1).
func (s *Service) GetPost(slug string) (*Post, error) {
	key, err := s.lookupPostKey(slug)
	if err != nil {
		return nil, err
	}
	var post Post
	err = s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketPosts)).Get(key)
		if raw == nil {
			return fmt.Errorf("post not found")
		}
		return json.Unmarshal(raw, &post)
	})
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// GetPublishedPost is GetPost's published-only variant, for the public
// single-post lookup (R1) — returns "not found" for a draft's slug.
func (s *Service) GetPublishedPost(slug string) (*Post, error) {
	post, err := s.GetPost(slug)
	if err != nil {
		return nil, err
	}
	if post.IsDraft() {
		return nil, fmt.Errorf("post not found")
	}
	return post, nil
}

// GetPublishedObject returns a published post's own ActivityPub object,
// keyed by the same slug as GetPublishedPost. A post's AS2 "id" must be a
// real, fetchable endpoint — remote servers dereference it directly (e.g.
// Mastodon's resolve=true search, or any Fetch of a Like/Announce target)
// rather than only ever receiving it inline via inbox delivery.
//
// Rebuilds the object live via nativeObject rather than reusing the Create
// stored in the outbox at publish time — the stored Create is a frozen
// snapshot (its own object's shape is whatever nativeObject produced back
// when the post was created or last edited), so a since-fixed bug in
// nativeObject's output (e.g. a missing field remote servers require)
// would otherwise stay broken forever for every post that predates the
// fix. This endpoint's whole job is to always be dereferenceable and
// correct, unlike the outbox's audit-trail role — nativeObject is already
// rebuilt the same way at delivery time (prepareFederatedDelivery/Update),
// so this isn't a new pattern.
func (s *Service) GetPublishedObject(slug string) (*vocab.Object, error) {
	post, err := s.GetPublishedPost(slug)
	if err != nil {
		return nil, err
	}
	obj := s.nativeObject(post)
	// Only set here, not in nativeObject: this object is being served
	// standalone at its own URL, so it's the JSON-LD document root and
	// needs its own context. Nested inside a Create activity's "object"
	// field (nativeObject's other use), the wrapping activity's context
	// already covers it.
	obj.Context = vocab.ActivityBaseURI
	return obj, nil
}

// UpdatePost changes an existing post's kind and content.
func (s *Service) UpdatePost(slug string, kind Kind, content string) (*Post, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if kind != KindNote && kind != KindArticle {
		return nil, fmt.Errorf("invalid kind %q", kind)
	}
	if kind == KindNote {
		if err := ValidateNoteContent(content); err != nil {
			return nil, err
		}
	}
	if slug == "" {
		return nil, fmt.Errorf("post not found")
	}

	key, err := s.lookupPostKey(slug)
	if err != nil {
		return nil, err
	}
	canonicalID := s.baseURL + postIDSuffix(slug)
	var updated Post
	err = s.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(store.BucketPosts))
		raw := postsBucket.Get(key)
		if raw == nil {
			return fmt.Errorf("post not found")
		}
		if err := json.Unmarshal(raw, &updated); err != nil {
			return err
		}
		if updated.IsDraft() {
			return fmt.Errorf("post is a draft; use SaveDraft/PublishDraft instead")
		}

		now := time.Now().UTC()
		updated.Kind = kind
		updated.Content = content
		updated.UpdatedAt = &now
		updated.ID = canonicalID

		rawPost, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		if string(key) != canonicalID {
			if err := postsBucket.Delete(key); err != nil {
				return err
			}
			if ob := tx.Bucket([]byte(store.BucketOutbox)); ob != nil {
				_ = ob.Delete(key)
			}
		}
		if err := postsBucket.Put([]byte(canonicalID), rawPost); err != nil {
			return err
		}

		obj := s.nativeObject(&updated)
		activity := vocab.CreateNew(vocab.IRI(canonicalID+"/activity"), obj)
		activity.Actor = vocab.IRI(s.actorIRI)
		activity.To = vocab.ItemCollection{vocab.PublicNS}
		activity.Published = updated.CreatedAt
		activity.Context = vocab.ActivityBaseURI

		rawActivity, err := json.Marshal(activity)
		if err != nil {
			return err
		}
		return tx.Bucket([]byte(store.BucketOutbox)).Put([]byte(canonicalID), rawActivity)
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// SetFederationRemoteURL records the post's resolved permalink on the
// reference instance, leaving the rest of the federation state untouched —
// unlike SetFederation, which replaces the whole record, this runs later
// (asynchronously, after delivery) and must not clobber a Shared/SharedAt
// already written by that earlier call.
func (s *Service) SetFederationRemoteURL(slug, remoteURL string) error {
	if slug == "" {
		return fmt.Errorf("post not found")
	}
	key, err := s.lookupPostKey(slug)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(store.BucketPosts))
		raw := postsBucket.Get(key)
		if raw == nil {
			return fmt.Errorf("post not found")
		}
		var post Post
		if err := json.Unmarshal(raw, &post); err != nil {
			return err
		}
		if post.Federation == nil {
			post.Federation = &FederationState{}
		}
		post.Federation.RemoteURL = remoteURL
		rawPost, err := json.Marshal(post)
		if err != nil {
			return err
		}
		return postsBucket.Put(key, rawPost)
	})
}

// SetFederation updates federation delivery metadata for a post.
func (s *Service) SetFederation(slug string, state FederationState) (*Post, error) {
	if slug == "" {
		return nil, fmt.Errorf("post not found")
	}
	key, err := s.lookupPostKey(slug)
	if err != nil {
		return nil, err
	}
	var updated Post
	err = s.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(store.BucketPosts))
		raw := postsBucket.Get(key)
		if raw == nil {
			return fmt.Errorf("post not found")
		}
		if err := json.Unmarshal(raw, &updated); err != nil {
			return err
		}
		stateCopy := state
		updated.Federation = &stateCopy
		rawPost, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		return postsBucket.Put(key, rawPost)
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// DeletePost removes a post and its outbox activity by slug.
func (s *Service) DeletePost(slug string) error {
	if slug == "" {
		return fmt.Errorf("post not found")
	}
	key, err := s.lookupPostKey(slug)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		postsBucket := tx.Bucket([]byte(store.BucketPosts))
		if postsBucket.Get(key) == nil {
			return fmt.Errorf("post not found")
		}
		if err := postsBucket.Delete(key); err != nil {
			return err
		}
		if ob := tx.Bucket([]byte(store.BucketOutbox)); ob != nil {
			_ = ob.Delete(key)
		}
		return nil
	})
}

// ListPosts returns published posts only. Every existing caller (feed
// generation, search-index rebuild, federation backfill) keeps calling this
// unchanged; the default now excludes drafts (R1, KTD4).
func (s *Service) ListPosts() ([]Post, error) {
	posts, _, err := s.listPostsSorted(false)
	return posts, err
}

// ListPostsPaginated returns a slice of published posts and the total count.
func (s *Service) ListPostsPaginated(limit, offset int) ([]Post, int, error) {
	return s.paginated(false, limit, offset)
}

// ListPostsForAuthor returns every post, drafts included — for the
// admin-authenticated Author list and Recent-posts sheet.
func (s *Service) ListPostsForAuthor() ([]Post, error) {
	posts, _, err := s.listPostsSorted(true)
	return posts, err
}

// ListPostsForAuthorPaginated is ListPostsForAuthor's paginated form.
func (s *Service) ListPostsForAuthorPaginated(limit, offset int) ([]Post, int, error) {
	return s.paginated(true, limit, offset)
}

func (s *Service) paginated(includeDrafts bool, limit, offset int) ([]Post, int, error) {
	posts, total, err := s.listPostsSorted(includeDrafts)
	if err != nil {
		return nil, 0, err
	}
	if limit <= 0 && offset <= 0 {
		return posts, total, nil
	}
	if offset > len(posts) {
		return []Post{}, total, nil
	}
	end := len(posts)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return posts[offset:end], total, nil
}

func (s *Service) listPostsSorted(includeDrafts bool) ([]Post, int, error) {
	var posts []Post
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(store.BucketPosts)).ForEach(func(_, v []byte) error {
			var p Post
			if err := json.Unmarshal(v, &p); err != nil {
				return err
			}
			if !includeDrafts && p.IsDraft() {
				return nil
			}
			posts = append(posts, p)
			return nil
		})
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].CreatedAt.After(posts[j].CreatedAt)
	})
	return posts, len(posts), nil
}

type timedCreate struct {
	published time.Time
	item      vocab.Item
}

func (s *Service) OutboxCollection() (*vocab.OrderedCollection, error) {
	col := vocab.OrderedCollectionNew(vocab.IRI(s.baseURL + "/outbox"))
	col.Context = jsonLDContext()
	var items []timedCreate
	err := s.db.View(func(tx *bolt.Tx) error {
		posts := tx.Bucket([]byte(store.BucketPosts))
		return tx.Bucket([]byte(store.BucketOutbox)).ForEach(func(k, v []byte) error {
			if posts != nil {
				if raw := posts.Get(k); raw != nil {
					var post Post
					if err := json.Unmarshal(raw, &post); err == nil && !FederationDelivered(post) {
						return nil
					}
				}
			}
			var act vocab.Create
			if err := json.Unmarshal(v, &act); err != nil {
				return err
			}
			items = append(items, timedCreate{published: act.Published, item: act})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].published.After(items[j].published)
	})
	var ordered vocab.ItemCollection
	for _, it := range items {
		ordered = append(ordered, it.item)
	}
	col.OrderedItems = ordered
	col.TotalItems = uint(len(ordered))
	return col, nil
}

func jsonLDContext() vocab.ItemCollection {
	ctx := make(vocab.ItemCollection, len(vocab.JsonLDContext))
	for i, iri := range vocab.JsonLDContext {
		ctx[i] = iri
	}
	return ctx
}

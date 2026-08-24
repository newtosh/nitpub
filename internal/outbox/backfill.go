package outbox

import (
	"encoding/json"
	"fmt"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

// BackfillResult summarizes a federation backfill run.
type BackfillResult struct {
	Sent    int      `json:"sent"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// LoadCreate loads the stored Create activity for a post ID.
func (s *Service) LoadCreate(postID string) (*vocab.Create, error) {
	var create vocab.Create
	err := s.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketOutbox)).Get([]byte(postID))
		if raw == nil {
			return fmt.Errorf("post not found")
		}
		return json.Unmarshal(raw, &create)
	})
	if err != nil {
		return nil, err
	}
	return &create, nil
}

// PrepareFederatedDelivery builds the Create activity sent to follower inboxes.
func (s *Service) PrepareFederatedDelivery(post Post) (*vocab.Create, error) {
	return s.prepareFederatedDelivery(post)
}

func (s *Service) prepareFederatedDelivery(post Post) (*vocab.Create, error) {
	create, err := s.LoadCreate(post.ID)
	if err != nil {
		return nil, err
	}
	obj := s.nativeObject(&post)
	activity := vocab.CreateNew(create.ID, obj)
	activity.Actor = create.Actor
	activity.To = create.To
	activity.Published = post.CreatedAt
	activity.Context = vocab.ActivityBaseURI
	return FederatedActivity(&post, activity)
}

func (s *Service) prepareFederatedUpdate(post Post) (*vocab.Update, error) {
	create, err := s.LoadCreate(post.ID)
	if err != nil {
		return nil, err
	}
	obj := s.nativeObject(&post)
	if post.Kind == KindArticle {
		contentHTML := ArticleFederationContentHTML(post.ID, post.Content)
		note := vocab.ObjectNew(vocab.NoteType)
		note.Content = vocab.NaturalLanguageValuesNew()
		note.ID = vocab.IRI(post.ID + "/federated")
		_ = note.Content.Append(vocab.NilLangRef, vocab.Content(contentHTML))
		note.URL = vocab.IRI(post.ID)
		note.Published = post.CreatedAt
		note.AttributedTo = create.Actor
		obj = note
	}
	update := vocab.UpdateNew(vocab.IRI(fmt.Sprintf("%s/activity#update/%s", post.ID, uuid.NewString())), obj)
	update.Actor = create.Actor
	update.To = create.To
	update.Context = vocab.ActivityBaseURI
	return update, nil
}

// BackfillFederation delivers Create activities for posts that were never successfully federated.
// Activities reuse stable IDs from the outbox so remote servers can deduplicate retries.
func (s *Service) BackfillFederation(deliver func(activity any) error) (BackfillResult, error) {
	posts, err := s.ListPosts()
	if err != nil {
		return BackfillResult{}, err
	}

	var result BackfillResult
	for _, post := range posts {
		if !NeedsFederationBackfill(post) {
			result.Skipped++
			continue
		}
		create, err := s.prepareFederatedDelivery(post)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", PostSlug(post.ID), err))
			continue
		}
		if deliver == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: delivery not configured", PostSlug(post.ID)))
			continue
		}
		if err := deliver(create); err != nil {
			slug := PostSlug(post.ID)
			_, _ = s.SetFederation(slug, FederationState{
				Shared: true,
				Error:  err.Error(),
			})
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", slug, err))
			continue
		}
		now := time.Now().UTC()
		if _, err := s.SetFederation(PostSlug(post.ID), FederationState{
			Shared:   true,
			SharedAt: &now,
		}); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", PostSlug(post.ID), err))
			continue
		}
		result.Sent++
	}
	return result, nil
}

// RedeliverSharedPosts pushes Update activities for posts already marked federated.
func (s *Service) RedeliverSharedPosts(deliver func(activity any) error) (BackfillResult, error) {
	posts, err := s.ListPosts()
	if err != nil {
		return BackfillResult{}, err
	}

	var result BackfillResult
	for _, post := range posts {
		if !FederationDelivered(post) {
			result.Skipped++
			continue
		}
		update, err := s.prepareFederatedUpdate(post)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", PostSlug(post.ID), err))
			continue
		}
		if deliver == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: delivery not configured", PostSlug(post.ID)))
			continue
		}
		if err := deliver(update); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", PostSlug(post.ID), err))
			continue
		}
		result.Sent++
	}
	return result, nil
}

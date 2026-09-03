package outbox

import (
	"fmt"
	"strings"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

func TestCreateNote(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, create, err := svc.CreatePost(KindNote, "hello fediverse")
	if err != nil {
		t.Fatal(err)
	}
	if post.Kind != KindNote {
		t.Fatalf("kind = %q", post.Kind)
	}
	fed, err := FederatedActivity(post, create)
	if err != nil {
		t.Fatal(err)
	}
	if fed.Object == nil {
		t.Fatal("missing federated object")
	}
	_ = vocab.OnObject(fed.Object, func(o *vocab.Object) error {
		if o.Type != vocab.NoteType {
			t.Fatalf("type = %q", o.Type)
		}
		return nil
	})
}

func TestCreateArticleFederatesAsNote(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, create, err := svc.CreatePost(KindArticle, stringsRepeat("word ", 100))
	if err != nil {
		t.Fatal(err)
	}
	fed, err := FederatedActivity(post, create)
	if err != nil {
		t.Fatal(err)
	}
	_ = vocab.OnObject(fed.Object, func(o *vocab.Object) error {
		if o.Type != vocab.NoteType {
			t.Fatalf("federated type = %q, want Note", o.Type)
		}
		return nil
	})
	_ = vocab.OnObject(create.Object, func(o *vocab.Object) error {
		if o.Type != vocab.ArticleType {
			t.Fatalf("native type = %q, want Article", o.Type)
		}
		return nil
	})
}

func TestRejectEmptyContent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	_, _, err = svc.CreatePost(KindNote, "   ")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestUpdatePost(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	created, _, err := svc.CreatePost(KindNote, "original")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(created.ID)
	updated, err := svc.UpdatePost(slug, KindArticle, "revised body")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "revised body" {
		t.Fatalf("content = %q", updated.Content)
	}
	if updated.Kind != KindArticle {
		t.Fatalf("kind = %q", updated.Kind)
	}
	if updated.UpdatedAt == nil {
		t.Fatal("expected updated_at")
	}
	if _, err := svc.UpdatePost("missing", KindNote, "x"); err == nil {
		t.Fatal("expected error for missing post")
	}
}

func TestGetPost(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	created, _, err := svc.CreatePost(KindNote, "find me")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(created.ID)
	got, err := svc.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "find me" {
		t.Fatalf("content = %q", got.Content)
	}
	if _, err := svc.GetPost("missing"); err == nil {
		t.Fatal("expected error for missing post")
	}
}

func TestGetPostLegacyBaseURL(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	old := New(st, "http://162.243.245.101", "http://162.243.245.101/actor")
	created, _, err := old.CreatePost(KindNote, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(created.ID)

	current := New(st, "https://nitpub.com", "https://nitpub.com/actor")
	got, err := current.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "legacy" {
		t.Fatalf("content = %q", got.Content)
	}
}

func TestRewritePostBaseURLs(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	old := New(st, "http://old.test", "http://old.test/actor")
	created, _, err := old.CreatePost(KindNote, "migrate me")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(created.ID)

	current := New(st, "https://new.test", "https://new.test/actor")
	n, err := current.RewritePostBaseURLs()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("migrated = %d, want 1", n)
	}
	got, err := current.GetPost(slug)
	if err != nil {
		t.Fatal(err)
	}
	wantID := "https://new.test/posts/" + slug
	if got.ID != wantID {
		t.Fatalf("id = %q, want %q", got.ID, wantID)
	}
}

func TestOutboxOrdering(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	for i := 0; i < 3; i++ {
		_, _, err := svc.CreatePost(KindNote, "post")
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	posts, err := svc.ListPosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 3 {
		t.Fatalf("count = %d", len(posts))
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestSaveDraftPartialTitleOnly(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, err := svc.SaveDraft(KindArticle, "Untitled thoughts", "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if post.Status != StatusDraft {
		t.Fatalf("status = %q", post.Status)
	}
	if post.Title == nil || *post.Title != "Untitled thoughts" {
		t.Fatalf("title = %v", post.Title)
	}
}

func TestSaveDraftPartialContentOnly(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, err := svc.SaveDraft(KindNote, "", "half a thought", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if post.Status != StatusDraft {
		t.Fatalf("status = %q", post.Status)
	}
	if post.Content != "half a thought" {
		t.Fatalf("content = %q", post.Content)
	}
}

func TestSaveDraftRejectsBlank(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	// ListPostsForAuthor, not ListPosts — a stray draft row from a broken
	// blank-input guard would be a draft, invisible to the published-only
	// ListPosts either way, which would make that count comparison vacuous.
	before, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveDraft(KindNote, "", "", ""); err == nil {
		t.Fatal("expected error for blank title and content")
	}
	after, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("expected no post created, before=%d after=%d", len(before), len(after))
	}
}

func TestSaveDraftUpdatesSameRow(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	first, err := svc.SaveDraft(KindNote, "", "first pass", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(first.ID)
	time.Sleep(2 * time.Millisecond)
	second, err := svc.SaveDraft(KindNote, "", "second pass", slug)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same ID, got %q vs %q", second.ID, first.ID)
	}
	if second.UpdatedAt == nil || !second.UpdatedAt.After(*first.UpdatedAt) {
		t.Fatalf("expected UpdatedAt to advance, first=%v second=%v", first.UpdatedAt, second.UpdatedAt)
	}
	posts, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected one draft row, got %d", len(posts))
	}
}

// TestSaveDraftIdempotentFirstSave proves the actual bug this design fixes:
// a client-generated slug reused across the *first* two SaveDraft calls
// (simulating a lost response — the client never learned the row existed,
// so it retries with the same slug it already picked) updates the same row
// instead of the old "empty slug always creates" behavior producing a
// second, orphaned draft.
func TestSaveDraftIdempotentFirstSave(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	clientSlug := uuid.NewString()
	first, err := svc.SaveDraft(KindNote, "", "attempt one", clientSlug)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := svc.SaveDraft(KindNote, "", "attempt one, retried", clientSlug)
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID != first.ID {
		t.Fatalf("expected the retry to target the same row, got %q vs %q", retry.ID, first.ID)
	}
	posts, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected exactly one draft row, got %d", len(posts))
	}
}

func TestSaveDraftRejectsMalformedSlug(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	for _, slug := range []string{"", "not-a-uuid", "../etc/passwd"} {
		if _, err := svc.SaveDraft(KindNote, "", "some content", slug); err == nil {
			t.Fatalf("expected SaveDraft to reject slug %q", slug)
		}
	}
}

func TestSaveDraftNeverTouchesOutbox(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, err := svc.SaveDraft(KindNote, "", "draft content", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	err = st.DB().View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketOutbox)).Get([]byte(post.ID))
		if raw != nil {
			t.Fatal("expected no outbox entry for an unpublished draft")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdatePostRejectsDraft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "still a draft", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(draft.ID)
	if _, err := svc.UpdatePost(slug, KindNote, "trying to federate via UpdatePost"); err == nil {
		t.Fatal("expected UpdatePost to reject a draft slug")
	}
	err = st.DB().View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketOutbox)).Get([]byte(draft.ID))
		if raw != nil {
			t.Fatal("UpdatePost must not federate a draft")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishDraftTransitionsAndFederates(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "a valid note draft", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(draft.ID)
	published, activity, err := svc.PublishDraft(slug)
	if err != nil {
		t.Fatal(err)
	}
	if published.IsDraft() {
		t.Fatalf("status = %q, expected published", published.Status)
	}
	if activity == nil {
		t.Fatal("expected a Create activity")
	}
	err = st.DB().View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte(store.BucketOutbox)).Get([]byte(published.ID))
		if raw == nil {
			t.Fatal("expected outbox entry after publish")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishDraftDoesNotCreateSecondRow(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "one row only", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	before, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	published, _, err := svc.PublishDraft(PostSlug(draft.ID))
	if err != nil {
		t.Fatal(err)
	}
	if published.ID != draft.ID {
		t.Fatalf("expected same ID, got %q vs %q", published.ID, draft.ID)
	}
	after, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("expected same post count, before=%d after=%d", len(before), len(after))
	}
}

func TestPublishDraftRejectsEmptyContent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	// A note has no title concept once published (KTD7) — a "title"-only note
	// draft has nothing to combine, leaving no publishable content.
	draft, err := svc.SaveDraft(KindNote, "in-progress title, no body", "", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PublishDraft(PostSlug(draft.ID)); err == nil {
		t.Fatal("expected error publishing a draft with no body content")
	}
}

func TestPublishDraftOnAlreadyPublishedIsError(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	created, _, err := svc.CreatePost(KindNote, "already live")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PublishDraft(PostSlug(created.ID)); err == nil {
		t.Fatal("expected error publishing an already-published slug")
	}
}

func TestPublishDraftNoteMatchesDirectCreate(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")

	direct, directActivity, err := svc.CreatePost(KindNote, "same content, direct create")
	if err != nil {
		t.Fatal(err)
	}
	directFed, err := FederatedActivity(direct, directActivity)
	if err != nil {
		t.Fatal(err)
	}

	draft, err := svc.SaveDraft(KindNote, "", "same content, direct create", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	published, publishedActivity, err := svc.PublishDraft(PostSlug(draft.ID))
	if err != nil {
		t.Fatal(err)
	}
	publishedFed, err := FederatedActivity(published, publishedActivity)
	if err != nil {
		t.Fatal(err)
	}

	var directType, publishedType, directContent, publishedContent string
	_ = vocab.OnObject(directFed.Object, func(o *vocab.Object) error {
		directType = fmt.Sprintf("%v", o.Type)
		directContent = o.Content.String()
		return nil
	})
	_ = vocab.OnObject(publishedFed.Object, func(o *vocab.Object) error {
		publishedType = fmt.Sprintf("%v", o.Type)
		publishedContent = o.Content.String()
		return nil
	})
	if directType != publishedType {
		t.Fatalf("activity object type differs: direct=%q published=%q", directType, publishedType)
	}
	if directContent != publishedContent {
		t.Fatalf("activity object content differs: direct=%q published=%q", directContent, publishedContent)
	}
}

func TestPublishDraftArticleRecombinesTitleAndContent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")

	direct, _, err := svc.CreatePost(KindArticle, "My Headline\n\nThe article body.")
	if err != nil {
		t.Fatal(err)
	}

	draft, err := svc.SaveDraft(KindArticle, "My Headline", "The article body.", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	published, _, err := svc.PublishDraft(PostSlug(draft.ID))
	if err != nil {
		t.Fatal(err)
	}
	if published.Content != direct.Content {
		t.Fatalf("recombined content = %q, want %q", published.Content, direct.Content)
	}
}

func TestPublishDraftUsesFreshPublishTimestamp(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "sat around for a while", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	draftCreatedAt := draft.CreatedAt
	time.Sleep(2 * time.Millisecond)

	published, activity, err := svc.PublishDraft(PostSlug(draft.ID))
	if err != nil {
		t.Fatal(err)
	}
	if !activity.Published.After(draftCreatedAt) {
		t.Fatalf("expected activity Published (%v) after draft's original CreatedAt (%v)", activity.Published, draftCreatedAt)
	}
	if !published.CreatedAt.Equal(draftCreatedAt) {
		t.Fatalf("expected stored CreatedAt to stay the draft's original value, got %v want %v", published.CreatedAt, draftCreatedAt)
	}
}

func TestListPostsExcludesDraftsIncludesPublished(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "a draft", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	published, _, err := svc.CreatePost(KindNote, "a published post")
	if err != nil {
		t.Fatal(err)
	}

	posts, err := svc.ListPosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].ID != published.ID {
		t.Fatalf("expected only the published post, got %+v", posts)
	}

	paginated, total, err := svc.ListPostsPaginated(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(paginated) != 1 || paginated[0].ID != published.ID {
		t.Fatalf("expected only the published post in paginated results, got %+v (total=%d)", paginated, total)
	}
	_ = draft
}

func TestListPostsForAuthorIncludesDrafts(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "a draft", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	published, _, err := svc.CreatePost(KindNote, "a published post")
	if err != nil {
		t.Fatal(err)
	}

	posts, err := svc.ListPostsForAuthor()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected both draft and published, got %d", len(posts))
	}

	paginated, total, err := svc.ListPostsForAuthorPaginated(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(paginated) != 2 {
		t.Fatalf("expected total=2, got total=%d len=%d", total, len(paginated))
	}
	_ = draft
	_ = published
}

func TestGetPublishedPostHidesDraft(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	draft, err := svc.SaveDraft(KindNote, "", "a draft", uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(draft.ID)

	if _, err := svc.GetPublishedPost(slug); err == nil {
		t.Fatal("expected not-found for a draft via GetPublishedPost")
	}
	if _, err := svc.GetPost(slug); err != nil {
		t.Fatalf("expected GetPost (author-scoped) to still find the draft: %v", err)
	}
}

func TestSaveDraftRejectsAlreadyPublished(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	published, _, err := svc.CreatePost(KindNote, "already live")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(published.ID)
	if _, err := svc.SaveDraft(KindNote, "", "trying to draft-save a live post", slug); err == nil {
		t.Fatal("expected error saving a draft against a published slug")
	}
}

// TestBuildQuoteContentGolden locks the composed markdown shape (R4, KTD2):
// linked source, blockquoted excerpt, commentary, then via, each on its own
// paragraph with no stray blank lines.
func TestBuildQuoteContentGolden(t *testing.T) {
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Excerpt:    "The excerpt text.",
		Commentary: "My take on this.",
		Via:        "Some Friend",
	}
	got, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	want := "[example.com](https://example.com/article)\n\n> The excerpt text.\n\nMy take on this.\n\n(via Some Friend)"
	if got != want {
		t.Fatalf("BuildQuoteContent =\n%q\nwant\n%q", got, want)
	}
}

func TestBuildQuoteContentOmitsBlankCommentary(t *testing.T) {
	fields := QuoteFields{
		SourceURL: "https://example.com/article",
		Excerpt:   "The excerpt text.",
		Via:       "Some Friend",
	}
	got, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	want := "[example.com](https://example.com/article)\n\n> The excerpt text.\n\n(via Some Friend)"
	if got != want {
		t.Fatalf("BuildQuoteContent =\n%q\nwant\n%q", got, want)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Fatal("expected no stray blank lines when commentary is blank")
	}
}

func TestBuildQuoteContentOmitsBlankVia(t *testing.T) {
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Excerpt:    "The excerpt text.",
		Commentary: "My take on this.",
	}
	got, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	want := "[example.com](https://example.com/article)\n\n> The excerpt text.\n\nMy take on this."
	if got != want {
		t.Fatalf("BuildQuoteContent =\n%q\nwant\n%q", got, want)
	}
	if strings.Contains(got, "via") {
		t.Fatal("expected no via line when Via is blank")
	}
}

func TestBuildQuoteContentRequiresSourceURL(t *testing.T) {
	_, err := BuildQuoteContent(QuoteFields{Excerpt: "text"})
	if err == nil {
		t.Fatal("expected error for missing SourceURL")
	}
}

func TestBuildQuoteContentRequiresExcerpt(t *testing.T) {
	_, err := BuildQuoteContent(QuoteFields{SourceURL: "https://example.com"})
	if err == nil {
		t.Fatal("expected error for missing Excerpt")
	}
}

// TestBuildQuoteContentRejectsUnsafeSourceURL proves SourceURL is validated
// as an absolute http(s) URL server-side (not just relied on client-side
// input validation, which isn't an API boundary) — a javascript: URL or
// relative path must never reach the composed markdown link.
func TestBuildQuoteContentRejectsUnsafeSourceURL(t *testing.T) {
	for _, sourceURL := range []string{
		"javascript:alert(1)",
		"/relative/path",
		"not a url at all",
		"",
	} {
		if _, err := BuildQuoteContent(QuoteFields{SourceURL: sourceURL, Excerpt: "text"}); err == nil {
			t.Fatalf("expected BuildQuoteContent to reject SourceURL %q", sourceURL)
		}
	}
}

// TestBuildQuoteContentBlockquotesMultiParagraphExcerpt proves every line of
// a multi-paragraph excerpt (including the blank separator) is prefixed
// with "> " — otherwise CommonMark ends the blockquote at the first
// unprefixed blank line, and the trailing paragraph renders as ordinary
// post content indistinguishable from commentary.
func TestBuildQuoteContentBlockquotesMultiParagraphExcerpt(t *testing.T) {
	fields := QuoteFields{
		SourceURL: "https://example.com/article",
		Excerpt:   "First paragraph.\n\nSecond paragraph.",
	}
	got, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	want := "[example.com](https://example.com/article)\n\n> First paragraph.\n>\n> Second paragraph."
	if got != want {
		t.Fatalf("BuildQuoteContent =\n%q\nwant\n%q", got, want)
	}
}

// TestBuildQuoteContentEscapesLinkText proves a Title containing markdown
// link-terminating characters (e.g. a fetched page's <title> containing
// "]") can't redirect the rendered link's href away from SourceURL.
func TestBuildQuoteContentEscapesLinkText(t *testing.T) {
	fields := QuoteFields{
		SourceURL: "https://example.com/article",
		Title:     "Click here](https://evil.example)",
		Excerpt:   "text",
	}
	got, err := BuildQuoteContent(fields)
	if err != nil {
		t.Fatal(err)
	}
	want := `[Click here\](https://evil.example)](https://example.com/article)` + "\n\n> text"
	if got != want {
		t.Fatalf("BuildQuoteContent =\n%q\nwant\n%q", got, want)
	}
}

func TestCreateQuotePost(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	fields := QuoteFields{
		SourceURL: "https://example.com/article",
		Excerpt:   "The excerpt text.",
		Via:       "Some Friend",
	}
	post, create, err := svc.CreateQuotePost(fields)
	if err != nil {
		t.Fatal(err)
	}
	if post.Kind != KindQuote {
		t.Fatalf("kind = %q", post.Kind)
	}
	if post.Quote == nil || *post.Quote != fields {
		t.Fatalf("Quote = %#v, want %#v", post.Quote, fields)
	}
	wantContent, _ := BuildQuoteContent(fields)
	if post.Content != wantContent {
		t.Fatalf("Content = %q, want %q", post.Content, wantContent)
	}

	// Regression: kind "quote" is accepted, not rejected as invalid.
	fed, err := FederatedActivity(post, create)
	if err != nil {
		t.Fatal(err)
	}
	_ = vocab.OnObject(fed.Object, func(o *vocab.Object) error {
		if o.Type != vocab.NoteType {
			t.Fatalf("federated type = %q, want Note", o.Type)
		}
		return nil
	})
}

func TestCreateQuotePostRejectsMissingFields(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	if _, _, err := svc.CreateQuotePost(QuoteFields{Excerpt: "text only"}); err == nil {
		t.Fatal("expected error for missing SourceURL")
	}
}

func TestUpdateQuotePost(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	post, _, err := svc.CreatePost(KindNote, "a plain note")
	if err != nil {
		t.Fatal(err)
	}
	slug := PostSlug(post.ID)

	fields := QuoteFields{
		SourceURL:  "https://example.com/updated",
		Excerpt:    "Updated excerpt.",
		Commentary: "Updated take.",
	}
	updated, err := svc.UpdateQuotePost(slug, fields)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Kind != KindQuote {
		t.Fatalf("kind = %q", updated.Kind)
	}
	if updated.Quote == nil || *updated.Quote != fields {
		t.Fatalf("Quote = %#v, want %#v", updated.Quote, fields)
	}
}

// TestSaveDraftRejectsQuoteKind: SaveDraft's shape has no room for
// QuoteFields and quote posts never autosave from the compose UI, so
// accepting KindQuote here would let a caller create a Kind: "quote" draft
// with no composed content, then publish it via PublishDraft — which
// likewise never runs BuildQuoteContent — bypassing R6/R7's
// quotePostsEnabled gate entirely.
func TestSaveDraftRejectsQuoteKind(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	if _, err := svc.SaveDraft(KindQuote, "", "still drafting the quote", ""); err == nil {
		t.Fatal("expected SaveDraft to reject kind=quote")
	}
}

// TestQuotePostFederatesFullContentSanitized covers KTD6/U1 step 6: a
// KindQuote post's serialized ActivityPub object carries sanitized HTML
// (not raw markdown), and the federated activity body carries the full
// composed content rather than a truncated excerpt-plus-link (the KindArticle
// shape).
func TestQuotePostFederatesFullContentSanitized(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Excerpt:    "The excerpt text.",
		Commentary: "My commentary paragraph.",
		Via:        "Some Friend",
	}
	post, create, err := svc.CreateQuotePost(fields)
	if err != nil {
		t.Fatal(err)
	}

	// The native object (what GetPublishedObject serves) must carry
	// sanitized HTML, not the raw markdown passthrough article-kind posts
	// get.
	obj, err := svc.GetPublishedObject(PostSlug(post.ID))
	if err != nil {
		t.Fatal(err)
	}
	var contentHTML string
	_ = vocab.OnObject(obj, func(o *vocab.Object) error {
		contentHTML = string(o.Content.First())
		return nil
	})
	if strings.Contains(contentHTML, "> The excerpt text.") {
		t.Fatalf("expected sanitized HTML, got raw markdown passthrough: %q", contentHTML)
	}
	if !strings.Contains(contentHTML, "<blockquote>") {
		t.Fatalf("expected sanitized HTML to contain a blockquote, got %q", contentHTML)
	}
	if !strings.Contains(contentHTML, "My commentary paragraph.") {
		t.Fatalf("expected sanitized HTML to contain commentary, got %q", contentHTML)
	}
	if !strings.Contains(contentHTML, "via Some Friend") {
		t.Fatalf("expected sanitized HTML to contain the via line, got %q", contentHTML)
	}

	// The federated activity body must carry the full composed content, not
	// a truncated excerpt (the KindArticle shape ArticleFederationContentHTML
	// produces).
	fed, err := FederatedActivity(post, create)
	if err != nil {
		t.Fatal(err)
	}
	var fedHTML string
	_ = vocab.OnObject(fed.Object, func(o *vocab.Object) error {
		fedHTML = string(o.Content.First())
		return nil
	})
	if !strings.Contains(fedHTML, "My commentary paragraph.") {
		t.Fatalf("expected federated body to carry full commentary, got %q", fedHTML)
	}
	if !strings.Contains(fedHTML, "via Some Friend") {
		t.Fatalf("expected federated body to carry the via line, got %q", fedHTML)
	}
	if strings.Contains(fedHTML, "Read more on") {
		t.Fatalf("expected federated body to skip the truncated article shape, got %q", fedHTML)
	}
}

// TestQuotePostFederationOmitsViaWhenBlank covers AE4 end-to-end: a via-less
// quote post carries no via text in its composed content, its serialized
// native object HTML, or its federated activity HTML.
func TestQuotePostFederationOmitsViaWhenBlank(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	fields := QuoteFields{
		SourceURL:  "https://example.com/article",
		Excerpt:    "The excerpt text.",
		Commentary: "My commentary paragraph.",
	}
	post, create, err := svc.CreateQuotePost(fields)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(post.Content, "via") {
		t.Fatalf("composed content leaked a via line with Via blank: %q", post.Content)
	}

	obj, err := svc.GetPublishedObject(PostSlug(post.ID))
	if err != nil {
		t.Fatal(err)
	}
	var contentHTML string
	_ = vocab.OnObject(obj, func(o *vocab.Object) error {
		contentHTML = string(o.Content.First())
		return nil
	})
	if strings.Contains(contentHTML, "via") {
		t.Fatalf("native object HTML leaked via text with Via blank: %q", contentHTML)
	}

	fed, err := FederatedActivity(post, create)
	if err != nil {
		t.Fatal(err)
	}
	var fedHTML string
	_ = vocab.OnObject(fed.Object, func(o *vocab.Object) error {
		fedHTML = string(o.Content.First())
		return nil
	})
	if strings.Contains(fedHTML, "via") {
		t.Fatalf("federated activity HTML leaked via text with Via blank: %q", fedHTML)
	}
}

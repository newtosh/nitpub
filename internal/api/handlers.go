package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/newtosh/nitpub/internal/analytics"
	"github.com/newtosh/nitpub/internal/icons"
	"github.com/newtosh/nitpub/internal/media"
	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
	"github.com/newtosh/nitpub/internal/search"
	"github.com/newtosh/nitpub/internal/sitecontent"
)

// Handler serves the minimal post API for the PWA.
type Handler struct {
	outbox *outbox.Service
	auth   *Auth
	media  *media.Service
	// icons is set via SetIcons after construction, not threaded through
	// NewHandler's already-long positional argument list — it's optional
	// (nil is a valid "icon serving unavailable" state, same as media) and
	// self-contained enough not to need constructor-time wiring.
	icons              *icons.Service
	site               *sitecontent.Service
	search             *search.Index
	moderation         *moderation.Service
	rebuildSearch      func()
	deliver            func(activity any) error
	resendAccepts      func() (int, error)
	backfillFederation func() (outbox.BackfillResult, error)
	redeliverShared    func() (outbox.BackfillResult, error)
	federationActor    string
	federationDomain   string
	federationBaseURL  string
	followerCount      func() int
	siteTitle          string
	// analytics mirrors icons: set via SetAnalytics after construction,
	// nil when analytics_enabled is off in config.toml (deploy-time-only,
	// see internal/config). analyticsEnabled is threaded through the
	// constructor like siteTitle since ServeSite needs it even when the
	// service itself is nil.
	analytics        *analytics.Service
	analyticsEnabled bool
	// analyticsPublicURL, set via SetAnalyticsPublicURL, is the public
	// GoatCounter dashboard link surfaced in the analytics response —
	// see the config.AnalyticsPublicURL field comment. Empty by default:
	// no link shown.
	analyticsPublicURL string
}

func NewHandler(
	ob *outbox.Service,
	auth *Auth,
	mediaSvc *media.Service,
	siteSvc *sitecontent.Service,
	searchIdx *search.Index,
	rebuildSearch func(),
	deliver func(activity any) error,
	resendAccepts func() (int, error),
	backfillFederation func() (outbox.BackfillResult, error),
	redeliverShared func() (outbox.BackfillResult, error),
	domain string,
	baseURL string,
	actor string,
	followerCount func() int,
	mod *moderation.Service,
	siteTitle string,
	analyticsEnabled bool,
) *Handler {
	return &Handler{
		outbox:             ob,
		auth:               auth,
		media:              mediaSvc,
		site:               siteSvc,
		search:             searchIdx,
		moderation:         mod,
		rebuildSearch:      rebuildSearch,
		deliver:            deliver,
		resendAccepts:      resendAccepts,
		backfillFederation: backfillFederation,
		redeliverShared:    redeliverShared,
		federationDomain:   domain,
		federationBaseURL:  baseURL,
		federationActor:    actor,
		followerCount:      followerCount,
		siteTitle:          siteTitle,
		analyticsEnabled:   analyticsEnabled,
	}
}

// SetIcons wires the icon-serving service in after construction (see the
// Handler.icons field comment for why).
func (h *Handler) SetIcons(svc *icons.Service) {
	h.icons = svc
}

// SetAnalytics wires the GoatCounter proxy service in after construction
// (see the Handler.analytics field comment for why).
func (h *Handler) SetAnalytics(svc *analytics.Service) {
	h.analytics = svc
}

// SetAnalyticsPublicURL wires the public GoatCounter dashboard link in
// after construction (see the Handler.analyticsPublicURL field comment).
func (h *Handler) SetAnalyticsPublicURL(url string) {
	h.analyticsPublicURL = url
}

type createPostRequest struct {
	Kind     string `json:"kind"`
	Content  string `json:"content"`
	Federate *bool  `json:"federate,omitempty"`
}

func (h *Handler) ServePosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPosts(w, r)
	case http.MethodPost:
		h.createPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPost(w, r)
	case http.MethodPut:
		h.updatePost(w, r)
	case http.MethodDelete:
		h.deletePost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// postWithReplyCount is the public post response shape: the domain Post
// plus a reader-facing approved-reply count. Kept as an API-layer DTO
// (rather than a field on outbox.Post) so the outbox package stays
// unaware of moderation, matching the existing package-boundary pattern
// (see publicReply in public_replies.go).
type postWithReplyCount struct {
	outbox.Post
	ReplyCount int `json:"reply_count"`
}

func (h *Handler) replyCountFor(slug string) int {
	if h.moderation == nil {
		return 0
	}
	n, err := h.moderation.ApprovedReplyCount(slug)
	if err != nil {
		return 0
	}
	return n
}

func (h *Handler) withReplyCount(post outbox.Post) postWithReplyCount {
	return postWithReplyCount{Post: post, ReplyCount: h.replyCountFor(outbox.PostSlug(post.ID))}
}

func (h *Handler) withReplyCounts(posts []outbox.Post) []postWithReplyCount {
	out := make([]postWithReplyCount, len(posts))
	for i, p := range posts {
		out[i] = h.withReplyCount(p)
	}
	return out
}

func (h *Handler) getPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("id")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	var post *outbox.Post
	var err error
	if h.auth.Authenticated(r) {
		post, err = h.outbox.GetPost(slug)
	} else {
		post, err = h.outbox.GetPublishedPost(slug)
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.withReplyCount(*post))
}

// ServePostObject serves a published post's own ActivityPub object at its
// canonical AS2 id (e.g. {baseURL}/posts/{id}) — distinct from GetPost's
// human-facing /api/posts/{id} JSON. Without this, the id a post's own
// object advertises as its "id"/"url" dereferences to nothing (the SPA
// catch-all serves index.html for any path, AS2 fetcher or not), so a
// remote server trying to resolve the post directly — e.g. Mastodon's
// resolve=true status search when a visitor signs in to comment — always
// fails, even though the post already delivered fine via inbox push.
func (h *Handler) ServePostObject(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("id")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	obj, err := h.outbox.GetPublishedObject(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, err := outbox.MarshalActivityPub(obj)
	if err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/activity+json")
	_, _ = w.Write(body)
}

type updatePostRequest struct {
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

func (h *Handler) updatePost(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := r.PathValue("id")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	var req updatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	kind := outbox.Kind(strings.ToLower(req.Kind))
	post, err := h.outbox.UpdatePost(slug, kind, req.Content)
	if err != nil {
		if err.Error() == "post not found" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(post)
	h.rebuildSearchIndex()
}

func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := r.PathValue("id")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	if err := h.outbox.DeletePost(slug); err != nil {
		if err.Error() == "post not found" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	h.rebuildSearchIndex()
}

func (h *Handler) listPosts(w http.ResponseWriter, r *http.Request) {
	authed := h.auth.Authenticated(r)
	limit, hasLimit := parseIntQuery(r, "limit")
	offset, hasOffset := parseIntQuery(r, "offset")
	if !hasLimit && !hasOffset {
		var posts []outbox.Post
		var err error
		if authed {
			posts, err = h.outbox.ListPostsForAuthor()
		} else {
			posts, err = h.outbox.ListPosts()
		}
		if err != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(h.withReplyCounts(posts))
		return
	}
	if !hasLimit || limit <= 0 {
		limit = 20
	}
	if !hasOffset || offset < 0 {
		offset = 0
	}
	var posts []outbox.Post
	var total int
	var err error
	if authed {
		posts, total, err = h.outbox.ListPostsForAuthorPaginated(limit, offset)
	} else {
		posts, total, err = h.outbox.ListPostsPaginated(limit, offset)
	}
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"posts": h.withReplyCounts(posts),
		"total": total,
	})
}

func parseIntQuery(r *http.Request, key string) (int, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}
	return n, true
}

func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req createPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	kind := outbox.Kind(strings.ToLower(req.Kind))
	post, create, err := h.outbox.CreatePost(kind, req.Content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	federate := h.resolveFederate(req.Federate)
	post, err = h.completeFederation(post, create, federate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(post)
	h.rebuildSearchIndex()
}

type saveDraftRequest struct {
	Kind    string `json:"kind"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
	Slug    string `json:"slug,omitempty"`
}

// SaveDraft handles POST /api/posts/drafts — autosaves partial title/content
// as a draft, creating it on first call and updating the same row on
// subsequent calls when Slug is set (R3, R4).
func (h *Handler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req saveDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	kind := outbox.Kind(strings.ToLower(req.Kind))
	post, err := h.outbox.SaveDraft(kind, req.Title, req.Content, req.Slug)
	if err != nil {
		if err.Error() == "post not found" {
			http.NotFound(w, r)
			return
		}
		if err.Error() == "post is already published" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(post)
}

type publishDraftRequest struct {
	Kind     string `json:"kind,omitempty"`
	Title    string `json:"title,omitempty"`
	Content  string `json:"content,omitempty"`
	Federate *bool  `json:"federate,omitempty"`
}

// PublishDraft handles POST /api/posts/{id}/publish — transitions an
// existing draft to published (R5). An optional JSON body carries the
// caller's just-typed title/content so Publish never republishes a stale
// prior autosave, and an explicit federate choice so a per-post "share to
// fediverse" toggle survives the draft path instead of falling back to the
// site default.
func (h *Handler) PublishDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := r.PathValue("id")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	var req publishDraftRequest
	hasBody := r.ContentLength != 0
	if hasBody {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
	}
	// A caller-supplied body always flushes, even if a field ended up empty
	// (e.g. the author cleared the title) — checking Title/Content non-empty
	// instead would silently skip that flush and republish stale content.
	if hasBody {
		kind := outbox.Kind(strings.ToLower(req.Kind))
		if _, err := h.outbox.SaveDraft(kind, req.Title, req.Content, slug); err != nil {
			if err.Error() == "post not found" {
				http.NotFound(w, r)
				return
			}
			if err.Error() == "post is already published" {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	post, create, err := h.outbox.PublishDraft(slug)
	if err != nil {
		if err.Error() == "post not found" {
			http.NotFound(w, r)
			return
		}
		if err.Error() == "post is already published" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	federate := h.resolveFederate(req.Federate)
	post, err = h.completeFederation(post, create, federate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(post)
	h.rebuildSearchIndex()
}

func (h *Handler) rebuildSearchIndex() {
	if h.rebuildSearch != nil {
		h.rebuildSearch()
	}
}

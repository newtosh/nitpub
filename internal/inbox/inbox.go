package inbox

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	vocab "github.com/go-ap/activitypub"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/federation"
	"github.com/newtosh/nitpub/internal/moderation"
	"github.com/newtosh/nitpub/internal/outbox"
)

// DeliverActivity posts a signed activity to one remote inbox.
type DeliverActivity func(inboxURL string, activity any) error

// Handler processes inbound ActivityPub activities.
type Handler struct {
	verify     *federation.Verifier
	ap         *apstore.AP
	outbox     *outbox.Service
	moderation *moderation.Service
	deliver    DeliverActivity
	actorIRI   string
	baseURL    string
	limiter    *RateLimiter
	// moderationEnabled reports whether incoming replies are gated by the
	// pending queue — read fresh per-request (not cached at startup) so an
	// admin toggling the setting takes effect immediately. nil means always
	// enabled, matching the config's own nil-means-true default.
	moderationEnabled func() bool
}

func NewHandler(verify *federation.Verifier, ap *apstore.AP, ob *outbox.Service, deliver DeliverActivity, actorIRI, baseURL string, mod *moderation.Service, moderationEnabled func() bool) *Handler {
	return &Handler{
		verify:            verify,
		ap:                ap,
		outbox:            ob,
		moderation:        mod,
		deliver:           deliver,
		actorIRI:          actorIRI,
		baseURL:           baseURL,
		limiter:           NewRateLimiter(30, time.Minute),
		moderationEnabled: moderationEnabled,
	}
}

func (h *Handler) moderationOn() bool {
	return h.moderationEnabled == nil || h.moderationEnabled()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := clientIP(r)
	if !h.limiter.Allow(ip) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	remoteActor, err := h.verify.VerifyRequest(r)
	if err != nil {
		log.Printf("inbox: signature verification failed: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	activity, err := federation.UnmarshalActivity(body)
	if err != nil {
		http.Error(w, "bad activity", http.StatusBadRequest)
		return
	}

	typ, _ := activity["type"].(string)
	switch typ {
	case "Follow":
		h.handleFollow(w, remoteActor, activity)
	case "Create":
		h.handleCreate(w, remoteActor, activity)
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) handleFollow(w http.ResponseWriter, remote vocab.Actor, activity map[string]any) {
	inbox := remoteInbox(remote)
	if inbox == "" {
		if actorObj, ok := activity["actor"].(string); ok {
			inbox = remoteInboxFromActorIRI(actorObj)
		}
	}
	if inbox == "" {
		http.Error(w, "missing inbox", http.StatusBadRequest)
		return
	}
	f := apstore.Follower{
		ActorIRI: string(remote.ID),
		InboxIRI: inbox,
	}
	if remote.Endpoints != nil {
		if shared := remoteInboxFromItem(remote.Endpoints.SharedInbox); shared != "" {
			f.SharedInboxIRI = shared
		}
	}
	if followID, ok := activity["id"].(string); ok {
		f.FollowID = followID
		raw, err := json.Marshal(activity)
		if err == nil {
			_ = h.ap.SaveInboxActivity(followID, raw)
		}
	}
	if err := h.ap.AddFollower(f); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	if h.deliver != nil {
		accept, err := federation.BuildAccept(h.actorIRI, h.baseURL, activity)
		if err != nil {
			log.Printf("inbox: build accept: %v", err)
			http.Error(w, "accept error", http.StatusInternalServerError)
			return
		}
		if err := h.deliver(inbox, accept); err != nil {
			log.Printf("inbox: deliver accept to %s: %v", inbox, err)
			http.Error(w, "accept delivery failed", http.StatusBadGateway)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handleCreate(w http.ResponseWriter, remote vocab.Actor, activity map[string]any) {
	obj, _ := activity["object"].(map[string]any)
	if obj == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	inReply, ok := obj["inReplyTo"].(string)
	if !ok || inReply == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	id, _ := activity["id"].(string)
	if id == "" {
		id = inReply + "#reply-" + time.Now().UTC().Format(time.RFC3339Nano)
	}

	if h.moderation == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var postSlug string
	var nested bool
	var parentActor, parentAuthorName string
	if strings.HasPrefix(inReply, h.baseURL) {
		// Direct reply to one of our posts.
		postSlug = outbox.PostSlug(inReply)
	} else {
		nested = true
		// Not a reply to a post of ours — check whether it's a reply to a
		// reply we already know about (nested threading) by resolving the
		// parent's own ActivityPub object id and inheriting its PostSlug.
		// Anything that doesn't resolve (a reply to something we never saw,
		// or a genuinely unrelated remote conversation) is dropped, same as
		// a non-matching top-level inReplyTo always has been.
		parent, err := h.moderation.FindByObjectID(inReply)
		if err != nil {
			log.Printf("inbox: resolve reply parent: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		if parent == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		postSlug = parent.PostSlug
		parentActor = parent.Actor
		parentAuthorName = parent.AuthorName
	}

	// Trust/block enforcement and the persisted actor identity are keyed
	// on the HTTP-signature-verified remoteActor, never on
	// activity["actor"] or object["attributedTo"] read from the
	// unverified body (KTD3, R11) — a signed request cannot claim a
	// different (trusted) actor's identity.
	actorURI := string(remote.ID)
	// Moderation-off auto-approves everything except explicitly blocked
	// actors — blocking is a separate, deliberate admin decision from the
	// pending queue, so it still applies even with the queue disabled.
	status := moderation.StatusPending
	if !h.moderationOn() {
		status = moderation.StatusApproved
	}
	if trusted, blocked, err := h.moderation.ClassifyActor(actorURI); err == nil {
		switch {
		case blocked:
			status = moderation.StatusRejected
		case trusted:
			status = moderation.StatusApproved
		}
	}

	var authorName string
	if len(remote.Name) > 0 {
		authorName = remote.Name.String()
	}

	objectID, _ := obj["id"].(string)
	content, _ := obj["content"].(string)
	reply := moderation.Reply{
		ActivityID:       id,
		PostSlug:         postSlug,
		Actor:            actorURI,
		Content:          content,
		AuthorName:       authorName,
		URL:              replyObjectURL(obj),
		AvatarURL:        itemLink(remote.Icon),
		ObjectID:         objectID,
		InReplyTo:        inReply,
		Nested:           nested,
		ParentActor:      parentActor,
		ParentAuthorName: parentAuthorName,
		Status:           status,
		Verified:         true,
	}
	if err := h.moderation.SaveReply(reply); err != nil {
		log.Printf("inbox: save reply: %v", err)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func remoteInbox(act vocab.Actor) string {
	if act.Inbox == nil {
		return ""
	}
	if iri, ok := act.Inbox.(vocab.IRI); ok {
		return string(iri)
	}
	return ""
}

func remoteInboxFromActorIRI(actorIRI string) string {
	if strings.HasSuffix(actorIRI, "/") {
		return actorIRI + "inbox"
	}
	return actorIRI + "/inbox"
}

// replyObjectURL picks the reply's browsable origin link: Mastodon (and most
// implementations) set "url" to the human-facing status page distinct from
// "id" (the machine-dereferenceable AP object). Fall back to "id" when "url"
// is absent, since some implementations don't distinguish the two.
func replyObjectURL(obj map[string]any) string {
	if u, ok := obj["url"].(string); ok && u != "" {
		return u
	}
	if id, ok := obj["id"].(string); ok {
		return id
	}
	return ""
}

// itemLink best-effort resolves an ActivityPub Item to a URL string —
// handles a bare IRI (Icon as a plain link) and an embedded Image object
// (Icon as {"type":"Image","url":"..."}, Mastodon's shape), preferring the
// object's "url" over its "id" since icon objects are usually anonymous.
func itemLink(item vocab.Item) string {
	switch v := item.(type) {
	case nil:
		return ""
	case vocab.IRI:
		return string(v)
	case *vocab.Object:
		if v == nil {
			return ""
		}
		if v.URL != nil {
			return itemLink(v.URL)
		}
		return string(v.ID)
	case vocab.Object:
		if v.URL != nil {
			return itemLink(v.URL)
		}
		return string(v.ID)
	default:
		return ""
	}
}

func remoteInboxFromItem(item vocab.Item) string {
	if item == nil {
		return ""
	}
	if iri, ok := item.(vocab.IRI); ok {
		return string(iri)
	}
	return ""
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return strings.Split(r.RemoteAddr, ":")[0]
}

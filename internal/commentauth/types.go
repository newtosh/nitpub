package commentauth

import "time"

// CommentSession identifies a remembered anonymous commenter by their
// Mastodon handle — never a live access token (KTD2: the struct has nowhere
// to put one).
type CommentSession struct {
	ID          string    `json:"id"`
	Instance    string    `json:"instance"`
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastSeen    time.Time `json:"last_seen"`
}

// PendingCommentAuth correlates an OAuth redirect round-trip back to the
// post and draft text it belongs to (KTD4), and its token doubles as the
// OAuth state parameter.
type PendingCommentAuth struct {
	Token     string    `json:"token"`
	PostSlug  string    `json:"post_slug"`
	DraftText string    `json:"draft_text"`
	Instance  string    `json:"instance"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

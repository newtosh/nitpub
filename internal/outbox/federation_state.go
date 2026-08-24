package outbox

// FederationDelivered reports a successful ActivityPub delivery for a post.
func FederationDelivered(post Post) bool {
	return post.Federation != nil && post.Federation.Shared && post.Federation.Error == ""
}

// NeedsFederationBackfill reports whether a post has never been successfully federated.
func NeedsFederationBackfill(post Post) bool {
	return !FederationDelivered(post)
}

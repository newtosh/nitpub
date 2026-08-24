package mastodon

import "time"

// AppRegistration is a cached OAuth app credential for one Mastodon-API-
// compatible domain (KTD3), reused across every visitor from that domain.
type AppRegistration struct {
	Domain       string    `json:"domain"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	Scope        string    `json:"scope"`
	RegisteredAt time.Time `json:"registered_at"`
}

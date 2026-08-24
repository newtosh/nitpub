package mastodon

import "context"

// DefaultScope is requested on first registration for a domain (KTD3).
// FallbackScope is used if a real instance rejects DefaultScope's
// read:search (unverified against a real instance — see the plan's
// Outstanding Questions).
const (
	// read:accounts covers VerifyCredentials (needed to show "Commenting
	// as @handle@instance", R6) on top of the search-resolve + posting
	// scopes the plan already committed to.
	DefaultScope  = "read:search read:accounts write:statuses"
	FallbackScope = "read write:statuses"
)

// AppRegistrar registers or reuses a cached OAuth app per instance domain.
type AppRegistrar struct {
	client *Client
	store  *AppStore
}

func NewAppRegistrar(client *Client, store *AppStore) *AppRegistrar {
	return &AppRegistrar{client: client, store: store}
}

// RegisterOrGetApp returns the cached app for domain, registering one with
// DefaultScope on a cache miss (KTD3).
func (r *AppRegistrar) RegisterOrGetApp(ctx context.Context, domain, redirectURI string) (*AppRegistration, error) {
	if cached, err := r.store.get(domain); err == nil {
		return cached, nil
	}
	reg, err := r.client.RegisterApp(ctx, domain, redirectURI, DefaultScope)
	if err != nil {
		return nil, err
	}
	if err := r.store.put(reg); err != nil {
		return nil, err
	}
	return reg, nil
}

// Reregister forces a fresh app registration with FallbackScope and
// overwrites the cache entry for domain — called when a ResolveStatus call
// returns ErrScopeRejected, indicating DefaultScope didn't work on this
// instance. Fixes the cache for the next visitor from this domain; it
// cannot retroactively widen the token already issued for the request that
// triggered it.
func (r *AppRegistrar) Reregister(ctx context.Context, domain, redirectURI string) (*AppRegistration, error) {
	reg, err := r.client.RegisterApp(ctx, domain, redirectURI, FallbackScope)
	if err != nil {
		return nil, err
	}
	if err := r.store.put(reg); err != nil {
		return nil, err
	}
	return reg, nil
}

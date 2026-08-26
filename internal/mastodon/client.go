// Package mastodon talks to any Mastodon-API-compatible instance (Mastodon,
// Pleroma, Akkoma, GoToSocial, ...) to let a blog visitor post a comment
// through their own account. See
// docs/plans/2026-08-21-002-feat-mastodon-powered-comments-plan.md.
package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const userAgent = "nitpub-comments (+https://github.com/newtosh/nitpub)"

// Client performs the REST calls this package needs against a visitor's
// home instance. It holds no per-visitor state — every call is scoped by
// the domain/token passed in.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	transport := &http.Transport{DialContext: secureDialContext}
	return &Client{httpClient: &http.Client{Timeout: 15 * time.Second, Transport: transport}}
}

// secureDialContext closes the DNS-rebinding gap in ValidateInstanceHost
// (KTD7): that check runs once, before the app is registered, but every
// actual HTTP call re-resolves DNS independently — an attacker who
// controls the instance domain's DNS can pass the one-time check with a
// public IP, then rebind the record to a private/loopback/metadata address
// before (or between) the calls that follow, turning nitpub into an SSRF
// proxy. Resolving and re-validating at the moment of every dial, then
// connecting to the vetted IP directly, closes that TOCTOU window
// regardless of how long the pending-auth record or app cache lives.
func secureDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if err := ValidateInstanceHost(host); err != nil {
		return nil, fmt.Errorf("dial %s: %w", host, err)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var lastErr error
	for _, ip := range ips {
		if isDisallowedIP(ip.IP) {
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no allowed address for %s", host)
	}
	return nil, lastErr
}

// NewClientWithHTTP builds a Client around a caller-supplied *http.Client —
// used in tests to point at an httptest.Server with a trusted transport.
func NewClientWithHTTP(hc *http.Client) *Client {
	return &Client{httpClient: hc}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", userAgent)
	return c.httpClient.Do(req)
}

func readErrorBody(resp *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return strings.TrimSpace(string(body))
}

// RegisterApp calls POST /api/v1/apps to dynamically register an OAuth app
// on domain (KTD3).
func (c *Client) RegisterApp(ctx context.Context, domain, redirectURI, scope string) (*AppRegistration, error) {
	form := url.Values{
		"client_name":   {"nitpub comments"},
		"redirect_uris": {redirectURI},
		"scopes":        {scope},
		"website":       {"https://" + domain},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+domain+"/api/v1/apps", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("register app on %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("register app on %s: status %d: %s", domain, resp.StatusCode, readErrorBody(resp))
	}

	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode app registration from %s: %w", domain, err)
	}
	if out.ClientID == "" || out.ClientSecret == "" {
		return nil, fmt.Errorf("app registration from %s missing client credentials", domain)
	}
	return &AppRegistration{
		Domain:       domain,
		ClientID:     out.ClientID,
		ClientSecret: out.ClientSecret,
		Scope:        scope,
		RegisteredAt: time.Now(),
	}, nil
}

// ExchangeToken calls POST /oauth/token with grant_type=authorization_code.
func (c *Client) ExchangeToken(ctx context.Context, domain, clientID, clientSecret, code, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+domain+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(req)
	if err != nil {
		return "", fmt.Errorf("exchange token on %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("exchange token on %s: status %d: %s", domain, resp.StatusCode, readErrorBody(resp))
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode token response from %s: %w", domain, err)
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf("token response from %s missing access_token", domain)
	}
	return out.AccessToken, nil
}

// ErrStatusNotFound means the search succeeded but found no matching
// status — distinct from a scope/auth rejection (see ErrScopeRejected).
var ErrStatusNotFound = fmt.Errorf("status not found on remote instance")

// ErrScopeRejected means the remote instance rejected the request in a way
// that indicates the granted scope was insufficient, not that the post is
// simply unresolvable yet (KTD3's scope fallback).
var ErrScopeRejected = fmt.Errorf("instance rejected the requested scope")

type resolvedStatus struct {
	ID   string
	Acct string
}

// searchResolveStatus calls GET /api/v2/search?resolve=true and returns the
// entry whose URL/URI exactly matches postURL. Shared by ResolveStatus
// (needs the local status ID, to reply against) and ResolvePermalink
// (needs the instance's own rendered URL for that status).
func (c *Client) searchResolveStatus(ctx context.Context, domain, token, postURL string) (resolvedStatus, error) {
	q := url.Values{
		"q":       {postURL},
		"type":    {"statuses"},
		"resolve": {"true"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+domain+"/api/v2/search?"+q.Encode(), nil)
	if err != nil {
		return resolvedStatus{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.do(req)
	if err != nil {
		return resolvedStatus{}, fmt.Errorf("resolve status on %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return resolvedStatus{}, ErrScopeRejected
	}
	if resp.StatusCode != http.StatusOK {
		return resolvedStatus{}, fmt.Errorf("resolve status on %s: status %d: %s", domain, resp.StatusCode, readErrorBody(resp))
	}

	var out struct {
		Statuses []struct {
			ID      string `json:"id"`
			URL     string `json:"url"`
			URI     string `json:"uri"`
			Account struct {
				Acct string `json:"acct"`
			} `json:"account"`
		} `json:"statuses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return resolvedStatus{}, fmt.Errorf("decode search response from %s: %w", domain, err)
	}
	for _, s := range out.Statuses {
		if s.URL == postURL || s.URI == postURL {
			return resolvedStatus{ID: s.ID, Acct: s.Account.Acct}, nil
		}
	}
	// No exact URL/URI match: fail rather than guess. Attaching to the
	// first search hit would thread the visitor's reply onto an unrelated
	// status on their own instance, publicly and irreversibly.
	return resolvedStatus{}, ErrStatusNotFound
}

// ResolveStatus calls GET /api/v2/search?resolve=true to find postURL's
// status ID on domain.
func (c *Client) ResolveStatus(ctx context.Context, domain, token, postURL string) (string, error) {
	s, err := c.searchResolveStatus(ctx, domain, token, postURL)
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

// ResolvePermalink is like ResolveStatus but returns the instance's own
// rendered page for the status (e.g. "https://mastodon.social/@user/123")
// instead of the bare local ID. Built manually from the account handle and
// local ID — Mastodon's own status.url field for a remote status just
// echoes back the origin's declared AS2 object URL (verified against a
// live instance: nitpub sets that URL to its own /p/ permalink, and the
// search response's "url" field came back identical to it), not a
// mastodon.social-hosted page.
func (c *Client) ResolvePermalink(ctx context.Context, domain, token, postURL string) (string, error) {
	s, err := c.searchResolveStatus(ctx, domain, token, postURL)
	if err != nil {
		return "", err
	}
	if s.Acct == "" {
		return "", fmt.Errorf("resolve permalink on %s: status missing account handle", domain)
	}
	return "https://" + domain + "/@" + s.Acct + "/" + s.ID, nil
}

// PostReply calls POST /api/v1/statuses with in_reply_to_id set.
func (c *Client) PostReply(ctx context.Context, domain, token, inReplyToID, text string) error {
	form := url.Values{
		"status":         {text},
		"in_reply_to_id": {inReplyToID},
		"visibility":     {"public"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+domain+"/api/v1/statuses", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("post reply on %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post reply on %s: status %d: %s", domain, resp.StatusCode, readErrorBody(resp))
	}
	return nil
}

// Account is the subset of GET /api/v1/accounts/verify_credentials this
// package needs to show "Commenting as @handle@instance" (R6).
type Account struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar"`
}

// VerifyCredentials calls GET /api/v1/accounts/verify_credentials to
// identify the account behind token.
func (c *Client) VerifyCredentials(ctx context.Context, domain, token string) (*Account, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+domain+"/api/v1/accounts/verify_credentials", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("verify credentials on %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verify credentials on %s: status %d: %s", domain, resp.StatusCode, readErrorBody(resp))
	}
	var acct Account
	if err := json.NewDecoder(resp.Body).Decode(&acct); err != nil {
		return nil, fmt.Errorf("decode account from %s: %w", domain, err)
	}
	if acct.Username == "" {
		return nil, fmt.Errorf("account response from %s missing username", domain)
	}
	return &acct, nil
}

// RevokeToken calls POST /oauth/revoke (KTD5). Best-effort: the caller
// logs and proceeds on failure rather than treating it as fatal — the
// token is already being discarded from this process either way.
func (c *Client) RevokeToken(ctx context.Context, domain, clientID, clientSecret, token string) error {
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"token":         {token},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+domain+"/oauth/revoke", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("revoke token on %s: %w", domain, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("revoke token on %s: status %d: %s", domain, resp.StatusCode, readErrorBody(resp))
	}
	return nil
}

// RevokeTokenBestEffort calls RevokeToken and logs, rather than returns,
// any failure (KTD5).
func (c *Client) RevokeTokenBestEffort(ctx context.Context, domain, clientID, clientSecret, token string) {
	if err := c.RevokeToken(ctx, domain, clientID, clientSecret, token); err != nil {
		log.Printf("comment-oauth: revoke token on %s: %v", domain, err)
	}
}

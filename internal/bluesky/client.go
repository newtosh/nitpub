// Package bluesky talks to a Bluesky/AT Protocol PDS (bsky.social by
// default) to crosspost nitpub entries. See
// docs/plans/2026-09-04-001-feat-bluesky-crosspost-plan.md.
package bluesky

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultBaseURL is Bluesky's hosted PDS. A self-hosted PDS can be passed to
// NewClient instead.
const DefaultBaseURL = "https://bsky.social"

const feedPostCollection = "app.bsky.feed.post"

// Client performs XRPC calls against one PDS. It holds no per-caller state —
// every authenticated call takes its access token as an argument.
type Client struct {
	BaseURL    string
	httpClient *http.Client
}

// NewClient builds a Client against baseURL, or DefaultBaseURL if empty.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{BaseURL: baseURL, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// NewClientWithHTTP builds a Client around a caller-supplied *http.Client —
// used in tests to point at an httptest.Server.
func NewClientWithHTTP(baseURL string, hc *http.Client) *Client {
	return &Client{BaseURL: baseURL, httpClient: hc}
}

// APIError is an AT Protocol XRPC error response: a non-2xx status carrying
// {"error": "...", "message": "..."}. Callers use IsUnauthorized to detect a
// 401 specifically (to trigger a refresh-and-retry) rather than string
// matching.
type APIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("bluesky: %s: %s (status %d)", e.ErrorCode, e.Message, e.StatusCode)
}

// IsUnauthorized reports whether err is an APIError with a 401 status.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized
	}
	return false
}

// xrpc performs one XRPC call, decoding a JSON response body into out (if
// non-nil) on success or an APIError on any non-2xx status.
func (c *Client) xrpc(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("bluesky: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		_ = json.Unmarshal(body, &apiErr)
		return &APIError{StatusCode: resp.StatusCode, ErrorCode: apiErr.Error, Message: apiErr.Message}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("bluesky: decode response: %w", err)
	}
	return nil
}

// CreateSession calls POST /xrpc/com.atproto.server.createSession with an
// app password. Unauthenticated.
func (c *Client) CreateSession(ctx context.Context, handle, appPassword string) (Session, error) {
	body, err := json.Marshal(struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}{Identifier: handle, Password: appPassword})
	if err != nil {
		return Session{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/xrpc/com.atproto.server.createSession", bytes.NewReader(body))
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var session Session
	if err := c.xrpc(req, &session); err != nil {
		return Session{}, fmt.Errorf("create session for %s: %w", handle, err)
	}
	return session, nil
}

// RefreshSession calls POST /xrpc/com.atproto.server.refreshSession,
// exchanging a refresh token for a new session. Called on a 401 from any
// authenticated call.
func (c *Client) RefreshSession(ctx context.Context, refreshJWT string) (Session, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/xrpc/com.atproto.server.refreshSession", nil)
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("Authorization", "Bearer "+refreshJWT)

	var session Session
	if err := c.xrpc(req, &session); err != nil {
		return Session{}, fmt.Errorf("refresh session: %w", err)
	}
	return session, nil
}

// UploadBlob calls POST /xrpc/com.atproto.repo.uploadBlob with the raw
// bytes, for use in a post embed by later units.
func (c *Client) UploadBlob(ctx context.Context, accessJWT string, data []byte, mimeType string) (BlobRef, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/xrpc/com.atproto.repo.uploadBlob", bytes.NewReader(data))
	if err != nil {
		return BlobRef{}, err
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Authorization", "Bearer "+accessJWT)

	var out struct {
		Blob BlobRef `json:"blob"`
	}
	if err := c.xrpc(req, &out); err != nil {
		return BlobRef{}, fmt.Errorf("upload blob: %w", err)
	}
	return out.Blob, nil
}

// CreateRecord calls POST /xrpc/com.atproto.repo.createRecord against the
// app.bsky.feed.post collection and returns the new record's URI and CID.
func (c *Client) CreateRecord(ctx context.Context, accessJWT, did string, record PostRecord) (uri, cid string, err error) {
	if record.Type == "" {
		record.Type = feedPostCollection
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(struct {
		Repo       string     `json:"repo"`
		Collection string     `json:"collection"`
		Record     PostRecord `json:"record"`
	}{Repo: did, Collection: feedPostCollection, Record: record})
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/xrpc/com.atproto.repo.createRecord", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessJWT)

	var out struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}
	if err := c.xrpc(req, &out); err != nil {
		return "", "", fmt.Errorf("create record: %w", err)
	}
	if out.URI == "" || out.CID == "" {
		return "", "", fmt.Errorf("create record: response missing uri/cid")
	}
	return out.URI, out.CID, nil
}

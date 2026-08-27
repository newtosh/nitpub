// Package telemetry implements opt-in version reporting for self-hosted
// nitpub instances: a one-time registration call that mints a bearer
// credential, and periodic OTLP metric exports carrying the running
// version and a few non-PII fields. See docs/plans/2026-08-27-001-feat-
// version-telemetry-plan.md.
//
// Every endpoint this package talks to is operator-supplied
// configuration (config.TelemetryRegisterURL / TelemetryIngestURL) — it
// never hardcodes a real receiver.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var registerClient = &http.Client{Timeout: 60 * time.Second}

type registerRequest struct {
	InstanceID string `json:"instance_id"`
}

type registerResponse struct {
	Token string `json:"token"`
}

// newInstanceID generates a random local instance identifier. It does not
// need RFC 4122 UUID structure, only uniqueness — same shape as
// auth.NewSessionID.
func newInstanceID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Register calls the configured registration endpoint and returns the
// instance ID it generated plus the bearer token the receiver issued.
// The exact response shape is a receiver-side contract (currently
// {"token": "..."}); callers should treat registration failure as
// "leave telemetry disabled" rather than fatal.
func Register(ctx context.Context, registerURL string) (instanceID, token string, err error) {
	instanceID, err = newInstanceID()
	if err != nil {
		return "", "", fmt.Errorf("telemetry: generate instance id: %w", err)
	}

	body, err := json.Marshal(registerRequest{InstanceID: instanceID})
	if err != nil {
		return "", "", fmt.Errorf("telemetry: encode register request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registerURL, bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("telemetry: build register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := registerClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("telemetry: register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("telemetry: register: unexpected status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("telemetry: read register response: %w", err)
	}

	var rr registerResponse
	if err := json.Unmarshal(raw, &rr); err != nil {
		return "", "", fmt.Errorf("telemetry: decode register response: %w", err)
	}
	if rr.Token == "" {
		return "", "", fmt.Errorf("telemetry: register response had no token")
	}

	return instanceID, rr.Token, nil
}

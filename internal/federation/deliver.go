package federation

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-ap/client"
)

// maxLoggedDeliveryBody caps how much of a remote inbox's response body gets
// logged per delivery attempt, so a large or malicious response can't bloat logs.
const maxLoggedDeliveryBody = 2048

// Deliver posts a signed activity to a remote inbox.
func Deliver(cl *client.C, signer *Signer, inboxURL string, activity any) error {
	body, err := MarshalActivity(activity)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, inboxURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if u, err := url.Parse(inboxURL); err == nil && u.Host != "" {
		req.Host = u.Host
		req.Header.Set("Host", u.Host)
	}
	req.Header.Set("Content-Type", client.ContentTypeJsonActivity)
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	if err := signer.SignDraft(req); err != nil {
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return fmt.Errorf("deliver to %s: %w", inboxURL, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxLoggedDeliveryBody))
	_, _ = io.Copy(io.Discard, resp.Body)
	// A 2xx here only means the remote server accepted the request for async
	// processing (e.g. Mastodon's Sidekiq queue) — it does not guarantee the
	// activity was actually rendered. Log the response body so a silent
	// downstream rejection is at least visible in our own logs.
	log.Printf("federation: deliver to %s: HTTP %d body=%q", inboxURL, resp.StatusCode, respBody)
	if resp.StatusCode >= 300 {
		return &DeliveryError{Inbox: inboxURL, Status: resp.StatusCode}
	}
	return nil
}

// DeliveryError reports a failed inbox delivery.
type DeliveryError struct {
	Inbox  string
	Status int
}

func (e *DeliveryError) Error() string {
	return fmt.Sprintf("delivery to %s failed with HTTP %d", e.Inbox, e.Status)
}

// DeliverToFollowers sends an activity to each follower inbox.
func DeliverToFollowers(cl *client.C, signer *Signer, inboxes []string, activity any) error {
	return DeliverToInboxes(cl, signer, inboxes, activity)
}

// DeliverToInboxes sends an activity to each inbox URL.
func DeliverToInboxes(cl *client.C, signer *Signer, inboxes []string, activity any) error {
	var errs []error
	for _, inbox := range inboxes {
		if err := Deliver(cl, signer, inbox, activity); err != nil {
			errs = append(errs, err)
		}
	}
	return JoinErrors(errs)
}

// JoinErrors combines multiple delivery errors into one.
func JoinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	var parts []string
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	return fmt.Errorf("delivery failed (%d): %s", len(parts), strings.Join(parts, "; "))
}

const MainKeyFragment = "#main-key"

package federation

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/auth"
	"github.com/go-ap/client"

	"github.com/newtosh/nitpub/internal/apstore"
)

// Verifier wraps inbound Cavage signature verification.
type Verifier struct {
	ap *apstore.AP
	cl *client.C
}

// NewVerifier constructs an inbound signature verifier.
func NewVerifier(ap *apstore.AP, cl *client.C) *Verifier {
	return &Verifier{ap: ap, cl: cl}
}

func (v *Verifier) verifier() interface {
	Verify(*http.Request) (vocab.Actor, error)
} {
	return auth.HTTPSignature(auth.WithStorage(v.ap), auth.WithClient(v.cl))
}

// VerifyRequest validates the HTTP signature on an inbox POST. Verify
// dispatches to RFC 9421 ("Signature-Input") or legacy Cavage draft
// ("Signature") verification depending on which header the sender used —
// calling VerifyDraftSignature directly would reject any sender (e.g.
// current Mastodon) using RFC 9421 signatures outright.
func (v *Verifier) VerifyRequest(r *http.Request) (vocab.Actor, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return vocab.Actor{}, err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	act, err := v.verifier().Verify(r)
	if err != nil {
		_ = v.ap.EnqueueKeyFetch(signatureKeyID(r))
		return vocab.Actor{}, err
	}
	return act, nil
}

func signatureKeyID(r *http.Request) string {
	sig := r.Header.Get("Signature")
	start := indexAfter(sig, `keyId="`)
	if start < 0 {
		return ""
	}
	rest := sig[start:]
	end := indexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func indexAfter(s, prefix string) int {
	for i := 0; i+len(prefix) <= len(s); i++ {
		if s[i:i+len(prefix)] == prefix {
			return i + len(prefix)
		}
	}
	return -1
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// CacheRemoteActor stores a fetched remote actor document.
func CacheRemoteActor(ap *apstore.AP, act vocab.Actor) error {
	return ap.SaveActor(act.ID, act)
}

// UnmarshalActivity decodes an ActivityStreams JSON payload.
func UnmarshalActivity(raw []byte) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

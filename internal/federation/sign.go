package federation

import (
	"crypto/rsa"
	"net/http"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client/s2s"
)

// Signer wraps Cavage-draft signing for outbound delivery.
type Signer struct {
	inner *s2s.Signer
}

// NewSigner constructs a Cavage-draft signer for the local actor.
func NewSigner(act vocab.Actor, key *rsa.PrivateKey) *Signer {
	actPtr := new(vocab.Actor)
	*actPtr = act
	return &Signer{inner: s2s.New(s2s.WithActor(actPtr, key))}
}

// SignDraft signs a request using the Cavage draft algorithm.
func (s *Signer) SignDraft(req *http.Request) error {
	return s.inner.SignDraft(req)
}

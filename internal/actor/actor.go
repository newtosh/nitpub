package actor

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html"
	"net/http"
	"strings"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/sitecontent"
)

const mainKeyFragment = "#main-key"

// Service manages the local ActivityPub actor.
type Service struct {
	cfg   config.Config
	store *apstore.AP
	site  *sitecontent.Service
	actor vocab.Actor
	key   *rsa.PrivateKey
}

// LoadOrCreate initializes the local actor and RSA keypair. site supplies the
// operator-editable icon/tagline shown on the actor document — its values
// are read live on each request (see ServeActor), not baked in at startup.
func LoadOrCreate(cfg config.Config, st *apstore.AP, site *sitecontent.Service) (*Service, error) {
	s := &Service{cfg: cfg, store: st, site: site}
	actorIRI := vocab.IRI(cfg.BaseURL + "/actor")

	if raw, err := st.GetMeta("private_key_pem"); err == nil && len(raw) > 0 {
		block, _ := pem.Decode(raw)
		if block == nil {
			return nil, fmt.Errorf("decode stored private key")
		}
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse stored private key: %w", err)
		}
		s.key = key
	} else {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		s.key = key
		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		if err := st.PutMeta("private_key_pem", pemBytes); err != nil {
			return nil, err
		}
	}

	if err := s.buildActor(actorIRI); err != nil {
		return nil, err
	}
	if err := st.SaveActor(actorIRI, s.actor); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Service) buildActor(actorIRI vocab.IRI) error {
	pubDER, err := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	if err != nil {
		return err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	person := vocab.PersonNew(actorIRI)
	person.Context = jsonLDContext()
	person.Inbox = vocab.IRI(s.cfg.BaseURL + "/inbox")
	person.Outbox = vocab.IRI(s.cfg.BaseURL + "/outbox")
	person.URL = vocab.IRI(s.cfg.BaseURL)
	_ = person.PreferredUsername.Append(vocab.NilLangRef, vocab.Content(s.cfg.Actor))
	_ = person.Name.Append(vocab.NilLangRef, vocab.Content(s.cfg.Actor))
	person.PublicKey = vocab.PublicKey{
		ID:           vocab.IRI(string(actorIRI) + mainKeyFragment),
		Owner:        actorIRI,
		PublicKeyPem: string(pubPEM),
	}

	s.actor = *person
	return nil
}

func (s *Service) Actor() vocab.Actor          { return s.actor }
func (s *Service) PrivateKey() *rsa.PrivateKey { return s.key }
func (s *Service) ActorIRI() vocab.IRI         { return s.actor.ID }
func (s *Service) MainKeyID() vocab.IRI        { return s.actor.PublicKey.ID }

func (s *Service) ServeActor(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, s.actor)
}

func (s *Service) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/activity+json")
	branding := sitecontent.BrandingConfig{Tagline: sitecontent.DefaultTagline}
	if s.site != nil {
		if m, err := s.site.Load(); err == nil {
			branding = m.Branding
		}
	}
	body, err := marshalActorDocument(v, s.cfg.BaseURL, branding)
	if err != nil {
		http.Error(w, "encode actor", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

// marshalActorDocument emits ActivityPub JSON-LD with Mastodon-compatible
// follow/discovery flags, plus the operator-editable icon (brand logo,
// falling back to the favicon) and profile bio.
func marshalActorDocument(v any, siteURL string, branding sitecontent.BrandingConfig) ([]byte, error) {
	body, err := marshalActivityPub(v)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	doc["@context"] = actorJSONLDContext()
	// Open account: instant follows, no approval queue on Mastodon.
	doc["manuallyApprovesFollowers"] = false
	doc["discoverable"] = true
	doc["indexable"] = true
	siteURL = strings.TrimRight(siteURL, "/")
	if siteURL != "" {
		doc["attachment"] = []map[string]any{
			{
				"type":  "PropertyValue",
				"name":  "Website",
				"value": websiteAttachmentValue(siteURL),
			},
		}
	}
	if iconURL := absoluteMediaURL(siteURL, branding.LogoURL); iconURL != "" {
		doc["icon"] = map[string]any{"type": "Image", "url": iconURL}
	} else if iconURL := absoluteMediaURL(siteURL, branding.FaviconURL); iconURL != "" {
		doc["icon"] = map[string]any{"type": "Image", "url": iconURL}
	}
	if branding.Tagline != "" {
		doc["summary"] = html.EscapeString(branding.Tagline)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}

func actorJSONLDContext() []any {
	return []any{
		"https://www.w3.org/ns/activitystreams",
		"https://w3id.org/security/v1",
		map[string]string{
			"schema":        "http://schema.org#",
			"PropertyValue": "schema:PropertyValue",
			"value":         "schema:value",
		},
	}
}

// absoluteMediaURL resolves an uploaded media path (e.g. "/media/xyz.png")
// against the site's base URL. Already-absolute URLs pass through unchanged.
func absoluteMediaURL(baseURL, mediaURL string) string {
	if mediaURL == "" {
		return ""
	}
	if strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return mediaURL
	}
	if baseURL == "" {
		return ""
	}
	return baseURL + "/" + strings.TrimPrefix(mediaURL, "/")
}

func websiteAttachmentValue(siteURL string) string {
	esc := html.EscapeString(siteURL)
	return fmt.Sprintf(`<a href="%s" rel="me nofollow noopener noreferrer" target="_blank">%s</a>`, esc, esc)
}

// marshalActivityPub emits ActivityPub JSON-LD with a proper @context key.
func marshalActivityPub(v any) ([]byte, error) {
	body, err := jsonld.Marshal(v)
	if err != nil {
		return nil, err
	}
	// go-ap/jsonld names the field "context"; fediverse software expects "@context".
	body = bytes.Replace(body, []byte(`"context":`), []byte(`"@context":`), 1)
	return body, nil
}

func jsonLDContext() vocab.ItemCollection {
	ctx := make(vocab.ItemCollection, len(vocab.JsonLDContext))
	for i, iri := range vocab.JsonLDContext {
		ctx[i] = iri
	}
	return ctx
}

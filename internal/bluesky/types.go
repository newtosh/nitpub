package bluesky

import "time"

// Session is the result of CreateSession or RefreshSession: the tokens and
// identity needed for every authenticated AT Protocol call that follows.
type Session struct {
	DID        string `json:"did"`
	Handle     string `json:"handle"`
	AccessJWT  string `json:"accessJwt"`
	RefreshJWT string `json:"refreshJwt"`
}

// BlobRef is the AT Protocol blob reference returned by UploadBlob, embedded
// verbatim into a post record (e.g. as an image embed) by later units.
type BlobRef struct {
	Type     string      `json:"$type"`
	Ref      BlobRefLink `json:"ref"`
	MimeType string      `json:"mimeType"`
	Size     int64       `json:"size"`
}

// BlobRefLink holds the CID link inside a BlobRef.
type BlobRefLink struct {
	Link string `json:"$link"`
}

// PostRecord is the app.bsky.feed.post record body sent to CreateRecord.
// Embed is left untyped ($type-discriminated per the AT Protocol lexicon)
// so later units (image/quote embeds) can attach one without this package
// needing to know every embed shape in advance.
type PostRecord struct {
	Type      string      `json:"$type"`
	Text      string      `json:"text"`
	CreatedAt time.Time   `json:"createdAt"`
	Embed     interface{} `json:"embed,omitempty"`
	// Facets marks byte ranges of Text as rich-text features (e.g. a
	// tappable link) — see Facet's doc comment in content.go for why
	// plain text needs this. Callers pass BuildPostText's Facets slice
	// through directly.
	Facets []Facet `json:"facets,omitempty"`
}

package federation

import (
	"bytes"

	"github.com/go-ap/jsonld"
)

// MarshalActivity serializes an ActivityPub object with a proper @context key.
func MarshalActivity(v any) ([]byte, error) {
	body, err := jsonld.Marshal(v)
	if err != nil {
		return nil, err
	}
	// go-ap/jsonld names the field "context"; fediverse software expects "@context".
	body = bytes.Replace(body, []byte(`"context":`), []byte(`"@context":`), 1)
	return body, nil
}

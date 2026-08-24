package federation

import (
	"fmt"

	vocab "github.com/go-ap/activitypub"
	"github.com/google/uuid"
)

// BuildAccept creates an Accept activity for a verified Follow.
func BuildAccept(actorIRI, baseURL string, follow map[string]any) (*vocab.Accept, error) {
	_ = baseURL
	followID, _ := follow["id"].(string)
	if followID == "" {
		return nil, fmt.Errorf("follow activity missing id")
	}
	followerActor := activityPubIRI(follow["actor"])
	followObject := activityPubIRI(follow["object"])
	if followerActor == "" || followObject == "" {
		return nil, fmt.Errorf("follow activity missing actor or object")
	}

	embedded := vocab.FollowNew(vocab.IRI(followID), vocab.IRI(followObject))
	embedded.Actor = vocab.IRI(followerActor)

	acceptID := vocab.IRI(fmt.Sprintf("%s#accepts/follows/%s", actorIRI, uuid.NewString()))
	accept := vocab.AcceptNew(acceptID, embedded)
	accept.Actor = vocab.IRI(actorIRI)
	accept.To = vocab.ItemCollection{vocab.IRI(followerActor)}
	accept.Context = vocab.ActivityBaseURI
	return accept, nil
}

func activityPubIRI(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		if id, ok := x["id"].(string); ok {
			return id
		}
	}
	return ""
}

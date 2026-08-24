package outbox

import vocab "github.com/go-ap/activitypub"

// FederatedActivity builds the outbound representation per KTD2.
func FederatedActivity(post *Post, create *vocab.Create) (*vocab.Create, error) {
	if post.Kind == KindNote {
		return create, nil
	}

	contentHTML := ArticleFederationContentHTML(post.ID, post.Content)
	note := vocab.ObjectNew(vocab.NoteType)
	note.Content = vocab.NaturalLanguageValuesNew()
	note.ID = vocab.IRI(post.ID + "/federated")
	_ = note.Content.Append(vocab.NilLangRef, vocab.Content(contentHTML))
	note.URL = vocab.IRI(post.ID)
	note.Published = post.CreatedAt
	note.AttributedTo = actorItem(create.Actor)

	fed := vocab.CreateNew(vocab.IRI(post.ID+"/federated/activity"), note)
	fed.Actor = actorItem(create.Actor)
	fed.To = vocab.ItemCollection{vocab.PublicNS}
	fed.Published = post.CreatedAt
	fed.Context = vocab.ActivityBaseURI
	return fed, nil
}

func actorItem(item vocab.Item) vocab.Item {
	if iri, ok := item.(vocab.IRI); ok {
		return iri
	}
	return item
}

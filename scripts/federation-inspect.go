//go:build ignore

// Read-only federation state from nitpub.db (followers, inbox activities).
//
// Stop nitpub first or use on a copy — bbolt is single-writer.
//
// Usage:
//
//	go run scripts/federation-inspect.go
//	NITPUB_CONFIG=/etc/nitpub/config.toml go run scripts/federation-inspect.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/go-ap/activitypub"
	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/apstore"
	"github.com/newtosh/nitpub/internal/config"
	"github.com/newtosh/nitpub/internal/store"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit JSON")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	actorIRI := activitypub.IRI(cfg.BaseURL + "/actor")
	ap := apstore.New(st, actorIRI)

	followers, err := ap.ListFollowers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "followers: %v\n", err)
		os.Exit(1)
	}

	inboxCount, inboxIDs, err := listInbox(st.DB())
	if err != nil {
		fmt.Fprintf(os.Stderr, "inbox: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"domain":         cfg.Domain,
			"actor":          actorIRI,
			"follower_count": len(followers),
			"followers":      followers,
			"inbox_count":    inboxCount,
			"inbox_activity": inboxIDs,
		})
		return
	}

	fmt.Printf("domain: %s\n", cfg.Domain)
	fmt.Printf("actor: %s\n", actorIRI)
	fmt.Printf("followers: %d\n", len(followers))
	for _, f := range followers {
		fmt.Printf("  - %s -> %s\n", f.ActorIRI, f.InboxIRI)
	}
	fmt.Printf("inbox activities: %d\n", inboxCount)
	for _, id := range inboxIDs {
		fmt.Printf("  - %s\n", id)
	}
}

func listInbox(db *bolt.DB) (int, []string, error) {
	var ids []string
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(store.BucketInbox))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, _ []byte) error {
			ids = append(ids, string(k))
			return nil
		})
	})
	return len(ids), ids, err
}

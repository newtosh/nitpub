// Sync posts and outbox activities from one nitpub bbolt file into another.
//
// Usage:
//
//	go run scripts/sync-posts-db.go --from ./data/nitpub.db --to /var/lib/nitpub/nitpub.db
package main

import (
	"flag"
	"fmt"
	"os"

	bolt "go.etcd.io/bbolt"

	"github.com/newtosh/nitpub/internal/store"
)

func main() {
	from := flag.String("from", "", "source nitpub.db path")
	to := flag.String("to", "", "destination nitpub.db path")
	flag.Parse()
	if *from == "" || *to == "" {
		fmt.Fprintln(os.Stderr, "usage: sync-posts-db --from SRC --to DST")
		os.Exit(2)
	}

	src, err := bolt.Open(*from, 0o600, &bolt.Options{ReadOnly: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open source: %v\n", err)
		os.Exit(1)
	}
	defer src.Close()

	dst, err := bolt.Open(*to, 0o600, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open destination: %v\n", err)
		os.Exit(1)
	}
	defer dst.Close()

	buckets := []string{store.BucketPosts, store.BucketOutbox}
	for _, name := range buckets {
		if err := copyBucket(src, dst, name); err != nil {
			fmt.Fprintf(os.Stderr, "copy %s: %v\n", name, err)
			os.Exit(1)
		}
	}
	fmt.Println("synced posts and outbox buckets")
}

func copyBucket(src, dst *bolt.DB, name string) error {
	var entries [][2][]byte
	if err := src.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b == nil {
			return fmt.Errorf("source bucket %q missing", name)
		}
		return b.ForEach(func(k, v []byte) error {
			entries = append(entries, [2][]byte{append([]byte(nil), k...), append([]byte(nil), v...)})
			return nil
		})
	}); err != nil {
		return err
	}

	return dst.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return err
		}
		if err := b.ForEach(func(k, _ []byte) error {
			return b.Delete(k)
		}); err != nil {
			return err
		}
		for _, pair := range entries {
			if err := b.Put(pair[0], pair[1]); err != nil {
				return err
			}
		}
		fmt.Printf("  %s: %d entries\n", name, len(entries))
		return nil
	})
}

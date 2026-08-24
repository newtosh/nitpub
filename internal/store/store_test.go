package store

import (
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func TestOpenCreatesDataDirAndBuckets(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested", "data")

	s, err := Open(nested)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("data dir not created: %v", err)
	}

	err = s.DB().View(func(tx *bolt.Tx) error {
		for _, name := range requiredBuckets {
			if tx.Bucket([]byte(name)) == nil {
				t.Fatalf("missing bucket %q", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("view buckets: %v", err)
	}
}

func TestReopenExistingDatabase(t *testing.T) {
	dir := t.TempDir()

	s1, err := Open(dir)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := s1.DB().Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucketMeta)).Put([]byte("probe"), []byte("ok"))
	}); err != nil {
		t.Fatalf("write probe: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	err = s2.DB().View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bucketMeta)).Get([]byte("probe"))
		if string(v) != "ok" {
			t.Fatalf("probe = %q, want ok", v)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("read probe: %v", err)
	}
}

func TestOpenUnwritablePath(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; cannot test unwritable path reliably")
	}
	_, err := Open("/proc/nitpub-should-not-write")
	if err == nil {
		t.Fatal("expected error opening unwritable path")
	}
}

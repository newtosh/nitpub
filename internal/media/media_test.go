package media

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndOpenPNG(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	name, err := svc.Save(bytes.NewReader(png), "image/png", int64(len(png)))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(name) != ".png" {
		t.Fatalf("ext = %q", name)
	}
	f, ct, err := svc.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if ct != "image/png" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestRejectInvalidName(t *testing.T) {
	dir := t.TempDir()
	svc, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Open("../secret.png"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMediaDirCreated(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "media")); err != nil {
		t.Fatal(err)
	}
}

package updatecheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryAssetName(t *testing.T) {
	got := BinaryAssetName("v1.2.3", "linux-amd64")
	if got != "nitpub-v1.2.3-linux-amd64" {
		t.Fatalf("got %q", got)
	}
}

func TestParseSHA256SUMS(t *testing.T) {
	body := `
# comment
deadbeefcafebabe000000000000000000000000000000000000000000000000  nitpub-v1.0.0-linux-amd64
abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd  ./nitpub-v1.0.0-linux-arm64
`
	m := ParseSHA256SUMS(body)
	if m["nitpub-v1.0.0-linux-amd64"] != "deadbeefcafebabe000000000000000000000000000000000000000000000000" {
		t.Fatalf("amd64: %#v", m)
	}
	if m["nitpub-v1.0.0-linux-arm64"] != "abcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcdabcd" {
		t.Fatalf("arm64: %#v", m)
	}
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, sum); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected mismatch error")
	}
	if err := VerifySHA256(path, ""); err == nil {
		t.Fatal("expected missing checksum error")
	}
}

func TestFindAsset(t *testing.T) {
	rel := Release{Tag: "v1", Assets: []Asset{{Name: "a", BrowserDownloadURL: "http://x"}}}
	a, err := FindAsset(rel, "a")
	if err != nil || a.BrowserDownloadURL != "http://x" {
		t.Fatalf("%v %#v", err, a)
	}
	if _, err := FindAsset(rel, "missing"); err == nil {
		t.Fatal("expected error")
	}
}

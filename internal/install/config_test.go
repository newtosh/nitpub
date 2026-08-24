package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	wrote, err := WriteConfigIfMissing(path, "blog.example.com", "T", "alice", "secret-value-here", dir+"/data", 8080)
	if err != nil || !wrote {
		t.Fatalf("wrote=%v err=%v", wrote, err)
	}
	wrote, err = WriteConfigIfMissing(path, "other.com", "T", "bob", "secret", dir+"/data", 8080)
	if err != nil || wrote {
		t.Fatalf("idempotent wrote=%v err=%v", wrote, err)
	}
}

func TestEnsureFederationSiteTOML(t *testing.T) {
	dir := t.TempDir()
	skipped, err := EnsureFederationSiteTOML(dir, false)
	if err != nil || skipped {
		t.Fatalf("first: skipped=%v err=%v", skipped, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "site", "site.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "cross_post_default") {
		t.Fatalf("missing key: %s", data)
	}
	skipped, err = EnsureFederationSiteTOML(dir, true)
	if err != nil || !skipped {
		t.Fatalf("second: skipped=%v err=%v", skipped, err)
	}
}

func TestScaffoldAnalytics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("domain = \"x\"\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	skipped, err := ScaffoldAnalytics(path)
	if err != nil || skipped {
		t.Fatalf("first: skipped=%v err=%v", skipped, err)
	}
	skipped, err = ScaffoldAnalytics(path)
	if err != nil || !skipped {
		t.Fatalf("second: skipped=%v err=%v", skipped, err)
	}
}

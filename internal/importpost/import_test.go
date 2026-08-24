package importpost

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/newtosh/nitpub/internal/outbox"
)

func TestParseFileFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	content := "---\nkind: note\n---\n\nHello note"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ParseFile(path, outbox.KindArticle)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != outbox.KindNote || p.Content != "Hello note" {
		t.Fatalf("%+v", p)
	}
}

func TestInferArticle(t *testing.T) {
	p, err := ParseFile(writeTemp(t, "Title line\n\nBody"), "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != outbox.KindArticle {
		t.Fatalf("kind = %s", p.Kind)
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

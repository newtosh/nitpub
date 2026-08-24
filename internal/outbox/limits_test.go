package outbox

import (
	"testing"

	"github.com/newtosh/nitpub/internal/store"
)

func TestValidateNoteContent(t *testing.T) {
	short := make([]rune, NoteMaxRunes)
	for i := range short {
		short[i] = 'a'
	}
	if err := ValidateNoteContent(string(short)); err != nil {
		t.Fatalf("expected ok at limit: %v", err)
	}

	long := string(short) + "x"
	if err := ValidateNoteContent(long); err == nil {
		t.Fatal("expected error over limit")
	}
}

func TestCreateNoteRejectsLongContent(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	svc := New(st, "http://example.test", "http://example.test/actor")
	body := make([]byte, NoteMaxRunes+1)
	for i := range body {
		body[i] = 'x'
	}
	_, _, err = svc.CreatePost(KindNote, string(body))
	if err == nil {
		t.Fatal("expected error")
	}
}

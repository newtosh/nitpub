package outbox

import (
	"fmt"
	"unicode/utf8"
)

// NoteMaxRunes is the recommended note length. ActivityPub does not define a cap,
// but Mastodon's default status limit is 500 characters — the practical federation bound.
const NoteMaxRunes = 500

// ValidateNoteContent returns an error when note content exceeds NoteMaxRunes.
func ValidateNoteContent(content string) error {
	if utf8.RuneCountInString(content) > NoteMaxRunes {
		return fmt.Errorf("notes are limited to %d characters (use article for longer posts)", NoteMaxRunes)
	}
	return nil
}

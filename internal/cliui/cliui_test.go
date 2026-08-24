package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormat_NO_COLOR_noANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	line := Format(&buf, LevelOK, "ready")
	if strings.Contains(line, "\x1b[") {
		t.Fatalf("expected no ANSI with NO_COLOR, got %q", line)
	}
	if !strings.Contains(line, "[OK]") || !strings.Contains(line, "ready") {
		t.Fatalf("unexpected line %q", line)
	}
}

func TestFormat_levels(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	cases := []struct {
		level Level
		want  string
	}{
		{LevelOK, "[OK]"},
		{LevelWarn, "[WARN]"},
		{LevelFail, "[FAIL]"},
		{LevelProgress, "[…]"},
	}
	for _, tc := range cases {
		line := Format(&buf, tc.level, "x")
		if !strings.HasPrefix(line, tc.want) {
			t.Fatalf("level %v: got %q want prefix %q", tc.level, line, tc.want)
		}
	}
}

func TestPrintln_writesNewline(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	Println(&buf, LevelWarn, "check")
	got := buf.String()
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

// Package cliui provides status lines and interactive prompts for nitpub CLI tools.
package cliui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// Level is a status severity for operator-facing CLI output.
type Level int

const (
	LevelOK Level = iota
	LevelWarn
	LevelFail
	LevelProgress
)

func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func marker(level Level) string {
	switch level {
	case LevelOK:
		return "OK"
	case LevelWarn:
		return "WARN"
	case LevelFail:
		return "FAIL"
	case LevelProgress:
		return "…"
	default:
		return "?"
	}
}

func styleMarker(level Level, enabled bool) string {
	m := marker(level)
	if !enabled {
		return m
	}
	var s lipgloss.Style
	switch level {
	case LevelOK:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	case LevelWarn:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	case LevelFail:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	case LevelProgress:
		s = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	default:
		return m
	}
	return s.Render(m)
}

// Format formats a status line. Color is applied only when w is a TTY and NO_COLOR is unset.
func Format(w io.Writer, level Level, msg string) string {
	return fmt.Sprintf("[%s] %s", styleMarker(level, colorEnabled(w)), strings.TrimSpace(msg))
}

// Println writes a status line to w followed by a newline.
func Println(w io.Writer, level Level, msg string) {
	fmt.Fprintln(w, Format(w, level, msg))
}

// OK prints a success status line to stderr.
func OK(msg string) { Println(os.Stderr, LevelOK, msg) }

// Warn prints a warning status line to stderr.
func Warn(msg string) { Println(os.Stderr, LevelWarn, msg) }

// Fail prints a failure status line to stderr.
func Fail(msg string) { Println(os.Stderr, LevelFail, msg) }

// Progress prints a progress status line to stderr.
func Progress(msg string) { Println(os.Stderr, LevelProgress, msg) }

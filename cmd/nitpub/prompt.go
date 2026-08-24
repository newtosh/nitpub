package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(syscall.Stdin)
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		fmt.Fprintln(os.Stderr)
		return "", fmt.Errorf("no password on stdin; use --password, --password-stdin, or an interactive terminal")
	}
	fmt.Fprintln(os.Stderr)
	return sc.Text(), nil
}

func readPasswordTwice(prompt1, prompt2 string) (string, error) {
	p1, err := readPassword(prompt1)
	if err != nil {
		return "", err
	}
	p2, err := readPassword(prompt2)
	if err != nil {
		return "", err
	}
	if p1 != p2 {
		return "", fmt.Errorf("passwords do not match")
	}
	if strings.TrimSpace(p1) == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	return p1, nil
}

func passwordFromFlags(password string, fromStdin bool) (string, error) {
	switch {
	case password != "" && fromStdin:
		return "", fmt.Errorf("use only one of --password or --password-stdin")
	case password != "":
		if strings.TrimSpace(password) == "" {
			return "", fmt.Errorf("password cannot be empty")
		}
		return password, nil
	case fromStdin:
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		pw := strings.TrimRight(string(raw), "\r\n")
		if pw == "" {
			return "", fmt.Errorf("password cannot be empty")
		}
		return pw, nil
	default:
		return readPasswordTwice("New password: ", "Confirm password: ")
	}
}

package install

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

//go:embed nitpub.service
var embeddedUnit []byte

// EmbeddedUnit returns the default systemd unit file contents.
func EmbeddedUnit() []byte { return embeddedUnit }

// EnsureSystemUser creates the nitpub system user/home if missing.
func EnsureSystemUser(home string) error {
	if home == "" {
		home = "/var/lib/nitpub"
	}
	if _, err := user.Lookup("nitpub"); err == nil {
		return nil
	}
	shell := "/usr/sbin/nologin"
	if _, err := os.Stat(shell); err != nil {
		shell = "/bin/false"
	}
	cmd := exec.Command("useradd", "--system", "--home", home, "--create-home", "--shell", shell, "nitpub")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("useradd nitpub: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureDataDir creates dataDir and chowns to nitpub when possible.
func EnsureDataDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	u, err := user.Lookup("nitpub")
	if err != nil {
		return nil // user may not exist yet in unit tests
	}
	cmd := exec.Command("chown", "-R", u.Username+":"+u.Username, dataDir)
	_ = cmd.Run()
	return nil
}

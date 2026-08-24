package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// nitpubServiceName is the systemd unit this process's --offline admin
// commands stop/start. Override via NITPUB_SERVICE for a non-default
// instance (see deploy/update.sh, which uses the same env var).
func nitpubServiceName() string {
	if v := os.Getenv("NITPUB_SERVICE"); v != "" {
		return v
	}
	return "nitpub"
}

func stopNitpubService() (bool, error) {
	active, err := serviceActive()
	if err != nil {
		return false, err
	}
	if !active {
		return false, nil
	}
	name := nitpubServiceName()
	if err := runSystemctl("stop", name); err != nil {
		return false, fmt.Errorf("stop %s service: %w (run as root)", name, err)
	}
	return true, nil
}

func startNitpubService() error {
	name := nitpubServiceName()
	if err := runSystemctl("start", name); err != nil {
		return fmt.Errorf("start %s service: %w", name, err)
	}
	return nil
}

func serviceActive() (bool, error) {
	out, _ := exec.Command("systemctl", "is-active", nitpubServiceName()).Output()
	return strings.TrimSpace(string(out)) == "active", nil
}

func runSystemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/cliui"
	"github.com/newtosh/nitpub/internal/updatecheck"
	"github.com/newtosh/nitpub/internal/version"
)

func newUpdateCmd() *cobra.Command {
	var apply bool
	var fromSource bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for or apply nitpub updates",
		Long: `Compares the running build against the latest version published on
GitHub. By default this only checks and reports — nothing is changed.

Pass --apply to download the matching Release binary, verify SHA256SUMS,
replace /usr/local/bin/nitpub, and restart the systemd service.

Advanced: --from-source runs deploy/update.sh from a git checkout
(NITPUB_REPO_DIR, default /var/lib/nitpub/src) for maintainer rebuilds.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if apply {
				if fromSource {
					return runUpdateApplyFromSource()
				}
				return runUpdateApplyRelease()
			}
			return runUpdateCheck()
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "download Release binary, install, and restart")
	cmd.Flags().BoolVar(&fromSource, "from-source", false, "with --apply: rebuild via deploy/update.sh instead of Release binary")
	return cmd
}

func runUpdateCheck() error {
	rel, err := updatecheck.Latest()
	if err != nil {
		return fmt.Errorf("check for updates: %w", err)
	}
	current := version.Version
	if rel.Tag == "" || rel.Tag == current {
		fmt.Printf("Up to date (%s).\n", current)
		return nil
	}
	fmt.Printf("Update available: %s -> %s\n%s\n", current, rel.Tag, rel.URL)
	fmt.Println("Run `nitpub update --apply` to install the Release binary.")
	return nil
}

func runUpdateApplyFromSource() error {
	repoDir := os.Getenv("NITPUB_REPO_DIR")
	if repoDir == "" {
		repoDir = "/var/lib/nitpub/src"
	}
	script := filepath.Join(repoDir, "deploy", "update.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("deploy script not found at %s (set NITPUB_REPO_DIR to your repo checkout): %w", script, err)
	}

	c := exec.Command("bash", script)
	c.Env = append(os.Environ(), "NITPUB_REPO_DIR="+repoDir)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

func runUpdateApplyRelease() error {
	cliui.Progress("fetching latest release")
	rel, err := updatecheck.Latest()
	if err != nil {
		return err
	}
	if rel.Tag == "" || rel.Tag == version.Version {
		cliui.OK("already up to date (" + version.Version + ")")
		return nil
	}
	arch, err := updatecheck.ArchSuffix()
	if err != nil {
		return err
	}
	name := updatecheck.BinaryAssetName(rel.Tag, arch)
	asset, err := updatecheck.FindAsset(rel, name)
	if err != nil {
		return err
	}
	sumsAsset, err := updatecheck.FindAsset(rel, "SHA256SUMS")
	if err != nil {
		return fmt.Errorf("release missing SHA256SUMS: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "nitpub-update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	binPath := filepath.Join(tmpDir, name)
	sumsPath := filepath.Join(tmpDir, "SHA256SUMS")
	cliui.Progress("downloading " + name)
	if err := updatecheck.DownloadFile(asset.BrowserDownloadURL, binPath); err != nil {
		return err
	}
	if err := updatecheck.DownloadFile(sumsAsset.BrowserDownloadURL, sumsPath); err != nil {
		return err
	}
	sumsBody, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	sums := updatecheck.ParseSHA256SUMS(string(sumsBody))
	want, ok := sums[name]
	if !ok {
		return fmt.Errorf("SHA256SUMS has no entry for %s", name)
	}
	if err := updatecheck.VerifySHA256(binPath, want); err != nil {
		return err
	}
	cliui.OK("checksum verified")

	dst := "/usr/local/bin/nitpub"
	cliui.Progress("installing binary to " + dst)
	if err := sudoRun("install", "-m", "755", binPath, dst); err != nil {
		return err
	}

	svc := nitpubServiceName()
	cliui.Progress("restarting " + svc)
	if err := sudoRun("systemctl", "restart", svc); err != nil {
		cliui.Warn("restart failed: " + err.Error())
		return err
	}
	cliui.OK("updated to " + rel.Tag)
	return nil
}

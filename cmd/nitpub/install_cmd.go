package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/newtosh/nitpub/internal/cliui"
	"github.com/newtosh/nitpub/internal/install"
)

func newInstallCmd() *cobra.Command {
	var (
		domain, title, actor, secret, username, password string
		configPath, dataDir, binaryPath                  string
		port                                             int
		withCaddy, noCaddy                               bool
		withFederation, noFederation                     bool
		withAnalytics, noAnalytics                       bool
		withTelemetry, noTelemetry                       bool
		nonInteractive                                   bool
		skipDoctor                                       bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and configure nitpub on this VPS",
		Long: `Interactive (or fully flagged) installer for Debian/Ubuntu VPS hosts.

Core setup writes config (if missing), installs a systemd unit, optionally
configures Caddy / federation defaults / analytics scaffold, creates an admin
account, and runs doctor.

Re-runs are lossless: existing config, Caddy site blocks, and site.toml are
skipped rather than overwritten.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := installOpts{
				Domain: domain, Title: title, Actor: actor, Secret: secret,
				Username: username, Password: password,
				ConfigPath: configPath, DataDir: dataDir, BinaryPath: binaryPath,
				Port:              port,
				WithCaddy:         gateChoice(withCaddy, noCaddy),
				WithFederation:    gateChoice(withFederation, noFederation),
				WithAnalytics:     gateChoice(withAnalytics, noAnalytics),
				WithTelemetry:     gateChoice(withTelemetry, noTelemetry),
				NonInteractive:    nonInteractive || !isInteractiveTTY(),
				SkipDoctor:        skipDoctor,
				CaddyFlagConflict: withCaddy && noCaddy,
				FedFlagConflict:   withFederation && noFederation,
				AnalyticsConflict: withAnalytics && noAnalytics,
				TelemetryConflict: withTelemetry && noTelemetry,
			}
			return runInstall(opts)
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "public hostname (required)")
	cmd.Flags().StringVar(&title, "title", "", "site title")
	cmd.Flags().StringVar(&actor, "actor", "", "ActivityPub actor username")
	cmd.Flags().StringVar(&secret, "secret", "", "instance secret (generated if empty in interactive mode)")
	cmd.Flags().StringVar(&username, "username", "admin", "admin username for admin init")
	cmd.Flags().StringVar(&password, "password", "", "admin password (prompted if empty and interactive)")
	cmd.Flags().StringVar(&configPath, "config", "/etc/nitpub/config.toml", "config.toml path")
	cmd.Flags().StringVar(&dataDir, "data-dir", "/var/lib/nitpub", "data directory")
	cmd.Flags().StringVar(&binaryPath, "binary", "/usr/local/bin/nitpub", "installed binary path")
	cmd.Flags().IntVar(&port, "port", 8080, "listen port")
	cmd.Flags().BoolVar(&withCaddy, "with-caddy", false, "enable Caddy gate")
	cmd.Flags().BoolVar(&noCaddy, "no-caddy", false, "skip Caddy gate")
	cmd.Flags().BoolVar(&withFederation, "with-federation", false, "enable federated cross-post default")
	cmd.Flags().BoolVar(&noFederation, "no-federation", false, "set cross_post_default=false on first site.toml write")
	cmd.Flags().BoolVar(&withAnalytics, "with-analytics", false, "scaffold analytics config keys")
	cmd.Flags().BoolVar(&noAnalytics, "no-analytics", false, "skip analytics gate")
	cmd.Flags().BoolVar(&withTelemetry, "with-telemetry", false, "opt in to version telemetry")
	cmd.Flags().BoolVar(&noTelemetry, "no-telemetry", false, "skip telemetry gate")
	cmd.Flags().BoolVar(&nonInteractive, "yes", false, "noninteractive: require flags, fail closed if missing")
	cmd.Flags().BoolVar(&skipDoctor, "skip-doctor", false, "skip doctor at end (not recommended)")
	return cmd
}

type tribool int

const (
	triUnset tribool = iota
	triYes
	triNo
)

func gateChoice(yes, no bool) tribool {
	if yes {
		return triYes
	}
	if no {
		return triNo
	}
	return triUnset
}

type installOpts struct {
	Domain, Title, Actor, Secret, Username, Password                         string
	ConfigPath, DataDir, BinaryPath                                          string
	Port                                                                     int
	WithCaddy, WithFederation, WithAnalytics, WithTelemetry                  tribool
	NonInteractive                                                           bool
	SkipDoctor                                                               bool
	CaddyFlagConflict, FedFlagConflict, AnalyticsConflict, TelemetryConflict bool
}

func isInteractiveTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func runInstall(o installOpts) error {
	// Optional gates: unset means skipped. Noninteractive does not invent answers.
	if err := o.fillInteractive(); err != nil {
		return err
	}
	if err := o.validate(); err != nil {
		return err
	}

	cliui.Progress("starting nitpub install")

	if err := ensureRootish(); err != nil {
		return err
	}

	cliui.Progress("ensuring system user and data directory")
	if err := ensureSystemUserAndData(o.DataDir); err != nil {
		return err
	}
	cliui.OK("system user and data directory ready")

	if err := writeConfigStep(o); err != nil {
		return err
	}

	if err := ensureSystemdUnit(o); err != nil {
		return fmt.Errorf("systemd: %w", err)
	}

	switch o.WithCaddy {
	case triYes:
		if err := runCaddyGate(o); err != nil {
			return err
		}
	default:
		cliui.OK("Caddy gate declined — skipped")
	}

	switch o.WithFederation {
	case triYes, triNo:
		cross := o.WithFederation == triYes
		skipped, err := ensureFederationStep(o.DataDir, cross)
		if err != nil {
			return err
		}
		if skipped {
			cliui.OK("federation: site.toml exists — skipped")
		} else if cross {
			cliui.OK("federation: wrote site.toml with cross_post_default=true")
		} else {
			cliui.OK("federation: wrote site.toml with cross_post_default=false")
		}
	default:
		cliui.OK("Federation gate declined — skipped")
	}

	if o.WithAnalytics == triYes {
		skipped, err := ensureAnalyticsStep(o.ConfigPath)
		if err != nil {
			return err
		}
		if skipped {
			cliui.OK("analytics: already enabled — skipped")
		} else {
			cliui.OK("analytics: scaffolded config keys (set API token; see deploy/README.md)")
			cliui.Warn("analytics: doctor will warn until GoatCounter token/base_url are complete")
		}
	} else {
		cliui.OK("Analytics gate declined — skipped")
	}

	if o.WithTelemetry == triYes {
		if err := maybeTelemetryEnable(o); err != nil {
			// Registration failure shouldn't fail the whole install —
			// warn and leave telemetry disabled, matching the "leave it
			// off" fallback used elsewhere for optional gates.
			cliui.Warn("telemetry: " + err.Error())
		}
	} else {
		cliui.OK("Telemetry gate declined — skipped")
	}

	if err := maybeAdminInit(o); err != nil {
		return err
	}
	// admin init runs directly as root (bypassing the nitpub service user),
	// so nitpub.db and anything else it touches under DataDir come out
	// root-owned — re-chown so the systemd unit (User=nitpub) can open them.
	if err := rechownDataDir(o.DataDir); err != nil {
		return fmt.Errorf("re-chown data dir after admin init: %w", err)
	}

	if !o.SkipDoctor {
		// Core install always configures systemd; require the unit to be active.
		if err := runDoctor(o.ConfigPath, o.BinaryPath, "nitpub", true); err != nil {
			return err
		}
	}

	cliui.OK("install finished")
	fmt.Fprintf(os.Stderr, "\nNext: open https://%s/login — then publish your first note at /author/compose\n", o.Domain)
	return nil
}

// rechownDataDir re-chowns dataDir to the nitpub system user, matching
// ensureSystemUserAndData's euid split: install.EnsureDataDir shells out to
// bare `chown` and silently ignores its error, which only works when this
// process already has permission (running as root). Under the sudo-based
// non-root path, it must go through sudoRun instead, same as
// ensureSystemUserAndData's own else-branch.
func rechownDataDir(dataDir string) error {
	if os.Geteuid() == 0 {
		return install.EnsureDataDir(dataDir)
	}
	return sudoRun("chown", "-R", "nitpub:nitpub", dataDir)
}

func ensureSystemUserAndData(dataDir string) error {
	if os.Geteuid() == 0 {
		if err := install.EnsureSystemUser(dataDir); err != nil {
			return err
		}
		return install.EnsureDataDir(dataDir)
	}
	if err := sudoRun("id", "nitpub"); err != nil {
		shell := "/usr/sbin/nologin"
		if err := sudoRun("useradd", "--system", "--home", dataDir, "--create-home", "--shell", shell, "nitpub"); err != nil {
			return fmt.Errorf("create system user nitpub: %w", err)
		}
	}
	if err := sudoRun("mkdir", "-p", dataDir); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	if err := sudoRun("chown", "-R", "nitpub:nitpub", dataDir); err != nil {
		return fmt.Errorf("chown data dir: %w", err)
	}
	return nil
}

func (o *installOpts) fillInteractive() error {
	if o.NonInteractive {
		if strings.TrimSpace(o.Secret) == "" {
			return fmt.Errorf("missing --secret")
		}
		if strings.TrimSpace(o.Password) == "" {
			return fmt.Errorf("missing --password (required in noninteractive mode)")
		}
		return nil
	}

	needForm := o.Domain == "" || o.Actor == "" ||
		o.WithCaddy == triUnset || o.WithFederation == triUnset || o.WithAnalytics == triUnset || o.WithTelemetry == triUnset
	if needForm {
		domain, actor, title, secret := o.Domain, o.Actor, o.Title, o.Secret
		caddy := o.WithCaddy == triYes || o.WithCaddy == triUnset
		fed := o.WithFederation != triNo
		analytics := o.WithAnalytics == triYes
		telemetryOptIn := o.WithTelemetry == triYes

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().Title("Domain").Value(&domain).Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
				huh.NewInput().Title("Actor (handle username)").Value(&actor).Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("required")
					}
					return nil
				}),
				huh.NewInput().Title("Site title").Value(&title),
				huh.NewInput().Title("Secret (leave empty to generate)").Value(&secret),
			),
			huh.NewGroup(
				huh.NewConfirm().Title("Install/configure Caddy for this domain?").Value(&caddy),
				huh.NewConfirm().Title("Enable federated cross-post by default?").Value(&fed),
				huh.NewConfirm().Title("Scaffold GoatCounter analytics config?").Value(&analytics),
				huh.NewConfirm().Title("Opt in to version telemetry? (sends version + instance ID to your configured receiver; off by default, no data leaves without this)").Value(&telemetryOptIn),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		o.Domain, o.Actor, o.Title, o.Secret = domain, actor, title, secret
		if o.WithCaddy == triUnset {
			o.WithCaddy = mapBool(caddy)
		}
		if o.WithFederation == triUnset {
			o.WithFederation = mapBool(fed)
		}
		if o.WithAnalytics == triUnset {
			o.WithAnalytics = mapBool(analytics)
		}
		if o.WithTelemetry == triUnset {
			o.WithTelemetry = mapBool(telemetryOptIn)
		}
	}
	if strings.TrimSpace(o.Secret) == "" {
		s, err := randomSecret()
		if err != nil {
			return err
		}
		o.Secret = s
		cliui.OK("generated instance secret")
	}
	if o.Password == "" {
		pw, err := readPasswordTwice("Admin password: ", "Confirm password: ")
		if err != nil {
			return err
		}
		o.Password = pw
	}
	return nil
}

func mapBool(v bool) tribool {
	if v {
		return triYes
	}
	return triNo
}

func (o installOpts) validate() error {
	if o.CaddyFlagConflict || o.FedFlagConflict || o.AnalyticsConflict || o.TelemetryConflict {
		return fmt.Errorf("conflicting with-/no- flags for an optional gate")
	}
	if strings.TrimSpace(o.Domain) == "" {
		return fmt.Errorf("missing --domain")
	}
	if strings.TrimSpace(o.Actor) == "" {
		return fmt.Errorf("missing --actor")
	}
	if strings.TrimSpace(o.Secret) == "" {
		return fmt.Errorf("missing --secret")
	}
	return nil
}

func ensureRootish() error {
	if os.Geteuid() == 0 {
		return nil
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("not root and sudo not found; re-run as root")
	}
	cliui.Progress("requesting sudo for privileged install steps")
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo authentication failed: %w", err)
	}
	cliui.OK("sudo credentials cached")
	return nil
}

func sudoRun(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("sudoRun: no args")
	}
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		all := append([]string{"sudo"}, args...)
		cmd = exec.Command(all[0], all[1:]...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func sudoAppendFile(path, content string) error {
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.Command("tee", "-a", path)
	} else {
		cmd = exec.Command("sudo", "tee", "-a", path)
	}
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readFilePrivileged(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if os.Geteuid() == 0 {
		return nil, err
	}
	cmd := exec.Command("sudo", "cat", path)
	out, err2 := cmd.CombinedOutput()
	if err2 != nil {
		return nil, fmt.Errorf("read %s: %w (%s)", path, err2, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func ensureFederationStep(dataDir string, cross bool) (bool, error) {
	sitePath := filepath.Join(dataDir, "site", "site.toml")
	if _, err := os.Stat(sitePath); err == nil {
		return true, nil
	}
	tmpDir, err := os.MkdirTemp("", "nitpub-site-*")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmpDir)
	skipped, err := install.EnsureFederationSiteTOML(tmpDir, cross)
	if err != nil || skipped {
		return skipped, err
	}
	src := filepath.Join(tmpDir, "site", "site.toml")
	dstDir := filepath.Join(dataDir, "site")
	if err := sudoRun("mkdir", "-p", dstDir); err != nil {
		return false, err
	}
	if err := sudoRun("install", "-m", "644", "-o", "nitpub", "-g", "nitpub", src, sitePath); err != nil {
		if err2 := sudoRun("install", "-m", "644", src, sitePath); err2 != nil {
			return false, err2
		}
	}
	return false, nil
}

func ensureAnalyticsStep(configPath string) (bool, error) {
	data, err := readFilePrivileged(configPath)
	if err != nil {
		return false, err
	}
	if strings.Contains(string(data), "analytics_enabled = true") {
		return true, nil
	}
	tmp, err := os.CreateTemp("", "nitpub-cfg-*.toml")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		return false, err
	}
	tmp.Close()
	if _, err := install.ScaffoldAnalytics(tmp.Name()); err != nil {
		return false, err
	}
	if err := sudoRun("install", "-m", "640", tmp.Name(), configPath); err != nil {
		return false, err
	}
	return false, nil
}

func writeConfigStep(o installOpts) error {
	_, err := os.Stat(o.ConfigPath)
	switch {
	case err == nil:
		cliui.OK("config exists — skipped " + o.ConfigPath)
		return nil
	case os.IsNotExist(err):
		// create below
	default:
		if _, err2 := readFilePrivileged(o.ConfigPath); err2 == nil {
			cliui.OK("config exists — skipped " + o.ConfigPath)
			return nil
		}
	}
	tmp, err := os.CreateTemp("", "nitpub-config-*.toml")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	_ = os.Remove(tmpPath) // WriteConfigIfMissing creates the file
	defer os.Remove(tmpPath)

	wrote, err := install.WriteConfigIfMissing(tmpPath, o.Domain, o.Title, o.Actor, o.Secret, o.DataDir, o.Port)
	if err != nil || !wrote {
		return fmt.Errorf("prepare config: wrote=%v err=%v", wrote, err)
	}
	if err := sudoRun("install", "-D", "-m", "640", "-o", "root", "-g", "nitpub", tmpPath, o.ConfigPath); err != nil {
		if err2 := sudoRun("install", "-D", "-m", "640", tmpPath, o.ConfigPath); err2 != nil {
			return err2
		}
	}
	cliui.OK("wrote " + o.ConfigPath)
	return nil
}

func runCaddyGate(o installOpts) error {
	if !install.IsDebianFamily() {
		return fmt.Errorf("caddy auto-provision supports Debian/Ubuntu only in v1")
	}
	if _, err := exec.LookPath("caddy"); err != nil {
		cliui.Progress("installing caddy via apt")
		if err := sudoRun("apt-get", "update"); err != nil {
			return err
		}
		if err := sudoRun("apt-get", "install", "-y", "caddy"); err != nil {
			return err
		}
		cliui.OK("caddy installed")
	} else {
		cliui.OK("caddy already installed")
	}
	caddyfile := "/etc/caddy/Caddyfile"
	present, err := caddySitePresent(caddyfile, o.Domain)
	if err != nil {
		return err
	}
	if present {
		cliui.OK("Caddy site for " + o.Domain + " already present — skipped")
		return nil
	}
	cliui.Progress("appending Caddy site block for " + o.Domain)
	block := fmt.Sprintf("\n%s {\n\tencode gzip\n\treverse_proxy localhost:%d\n}\n", o.Domain, o.Port)
	if err := sudoAppendFile(caddyfile, block); err != nil {
		return err
	}
	if err := sudoRun("caddy", "validate", "--config", caddyfile); err != nil {
		return fmt.Errorf("caddy validate failed: %w", err)
	}
	if err := sudoRun("systemctl", "reload", "caddy"); err != nil {
		return fmt.Errorf("caddy reload failed: %w", err)
	}
	cliui.OK("Caddy site configured for " + o.Domain)
	return nil
}

func caddySitePresent(caddyfile, domain string) (bool, error) {
	present, err := install.SiteBlockPresent(caddyfile, domain)
	if err == nil {
		return present, nil
	}
	data, err2 := readFilePrivileged(caddyfile)
	if err2 != nil {
		return false, err2
	}
	return install.SiteBlockInContent(string(data), domain)
}

func ensureSystemdUnit(o installOpts) error {
	dst := "/etc/systemd/system/nitpub.service"
	if _, err := os.Stat(dst); err == nil {
		cliui.OK("systemd unit exists — skipped")
		if err := sudoRun("systemctl", "enable", "--now", "nitpub"); err != nil {
			return fmt.Errorf("enable/start nitpub: %w", err)
		}
		return nil
	}
	tmp, err := os.CreateTemp("", "nitpub-*.service")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(install.EmbeddedUnit()); err != nil {
		return err
	}
	tmp.Close()
	cliui.Progress("installing systemd unit")
	if err := sudoRun("install", "-m", "644", tmp.Name(), dst); err != nil {
		return err
	}
	if err := sudoRun("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := sudoRun("systemctl", "enable", "--now", "nitpub"); err != nil {
		return err
	}
	cliui.OK("systemd unit enabled")
	return nil
}

func adminCmd(o installOpts, args ...string) *exec.Cmd {
	bin := o.BinaryPath
	if _, err := os.Stat(bin); err != nil {
		if exe, err2 := os.Executable(); err2 == nil {
			bin = exe
		}
	}
	full := append([]string{bin}, args...)
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.Command(full[0], full[1:]...)
	} else {
		all := append([]string{"sudo", "-E"}, full...)
		cmd = exec.Command(all[0], all[1:]...)
	}
	cmd.Env = append(os.Environ(), "NITPUB_CONFIG="+o.ConfigPath)
	return cmd
}

func maybeAdminInit(o installOpts) error {
	cliui.Progress("ensuring admin account")
	status := adminCmd(o, "admin", "status", "--offline")
	out, err := status.CombinedOutput()
	if err == nil && bytes.Contains(out, []byte("Admin:")) && !bytes.Contains(out, []byte("not configured")) {
		cliui.OK("admin already configured — skipped")
		return nil
	}

	cmd := adminCmd(o, "admin", "init", "--username", o.Username, "--offline", "--password", o.Password)
	out, err = cmd.CombinedOutput()
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	if err != nil {
		combined := string(out) + err.Error()
		if strings.Contains(combined, "admin already exists") {
			cliui.OK("admin already configured — skipped")
			return nil
		}
		return fmt.Errorf("admin init: %w", err)
	}
	cliui.OK("admin account ready")
	return nil
}

func maybeTelemetryEnable(o installOpts) error {
	cliui.Progress("registering telemetry")
	cmd := adminCmd(o, "telemetry", "enable", "--offline")
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		_, _ = os.Stdout.Write(out)
	}
	if err != nil {
		return fmt.Errorf("enable: %w", err)
	}
	cliui.OK("telemetry enabled")
	return nil
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

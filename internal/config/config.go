package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultPort       = 8080
	defaultDataDir    = "./data"
	defaultActor      = "user"
	defaultDevSecret  = "dev-secret-change-me"
	defaultConfigName = "config.toml"
	legacyConfigName  = "nitpub.toml"

	// defaultTelemetryRegisterURL and defaultTelemetryIngestURL point at
	// nitpub's project-run telemetry collector — a neutral hostname
	// under the maintainer's public project domain (newto.sh, already
	// the module's public identity via github.com/newtosh/nitpub), not
	// their home-infra domain. Being public doesn't weaken privacy: the
	// registration + bearer-token gate is what protects the backend,
	// not secrecy of the URL. Telemetry still defaults OFF (R2) — a
	// populated default here only makes opt-in available without an
	// operator having to source a URL out-of-band; it sends nothing
	// until explicitly enabled. Operators may still override both to
	// point at their own receiver.
	defaultTelemetryRegisterURL = "https://telemetry.newto.sh/register"
	defaultTelemetryIngestURL   = "https://telemetry.newto.sh/v1/metrics"
)

// Config holds runtime settings for a single-actor nitpub instance.
type Config struct {
	Domain     string
	Title      string
	Port       int
	DataDir    string
	Actor      string
	Secret     string
	BaseURL    string
	HTTPDev    bool
	SystemUser string
	ConfigPath string

	// AnalyticsEnabled and friends are deploy-time-only (read once at
	// startup, no runtime toggle) — see docs/plans/2026-08-23-001-feat-
	// goatcounter-analytics-plan.md. AnalyticsBaseURL points at the
	// GoatCounter instance nitpub proxies to (e.g. http://127.0.0.1:8081).
	AnalyticsEnabled  bool
	AnalyticsAPIToken string
	AnalyticsBaseURL  string

	// AnalyticsVhost, when set, overrides the Host header nitpub sends
	// on its GoatCounter API calls to match that site's own -vhost
	// value. GoatCounter resolves which site a request is for by Host
	// header, falling back to "the only site" only when its whole
	// instance has exactly one site — so leaving this empty relies on
	// that fallback, which breaks the moment a second site (for another
	// app on the same VPS) is added to the same GoatCounter instance.
	// Required only in that multi-site case.
	AnalyticsVhost string

	// AnalyticsPublicURL, when set (and analytics_enabled is true), is the
	// public-facing GoatCounter base URL (e.g. https://stats.example.com).
	// It does two things: (1) shows an "open full dashboard" link in Admin →
	// Analytics, and (2) auto-injects GoatCounter's count.js beacon into
	// every public SPA shell response so pageviews are recorded without a
	// manually pasted script. Distinct from AnalyticsBaseURL, which is the
	// internal API address nitpub itself calls. Left empty, no link and no
	// beacon. Never carries the GoatCounter secret token: that stays
	// server-side in Caddy's forward_auth config — see deploy/README.md.
	AnalyticsPublicURL string

	// SessionCookieDomain, when set, widens the admin session cookie
	// beyond its default host-only scope (e.g. ".example.com") so a
	// trusted subdomain can share it — specifically, a Caddy
	// forward_auth check protecting a proxied internal tool like the
	// GoatCounter dashboard above. Empty by default (unchanged
	// behavior); see internal/auth.Service's cookieDomain field comment
	// for the security tradeoff of setting this.
	SessionCookieDomain string

	// TelemetryRegisterURL and TelemetryIngestURL are the opt-in version
	// telemetry endpoints (see internal/telemetry). Deploy-time only,
	// like the Analytics fields above. Default to nitpub's project-run
	// collector (defaultTelemetryRegisterURL/IngestURL); an operator can
	// override both to point at their own receiver, or leave them as-is
	// — either way telemetry stays off (R2) until explicitly enabled.
	TelemetryRegisterURL string
	TelemetryIngestURL   string
}

type fileConfig struct {
	Domain               string `toml:"domain"`
	Title                string `toml:"title"`
	Port                 int    `toml:"port"`
	DataDir              string `toml:"data_dir"`
	Actor                string `toml:"actor"`
	Secret               string `toml:"secret"`
	HTTP                 bool   `toml:"http"`
	SystemUser           string `toml:"system_user"`
	AnalyticsEnabled     bool   `toml:"analytics_enabled"`
	AnalyticsAPIToken    string `toml:"analytics_api_token"`
	AnalyticsBaseURL     string `toml:"analytics_base_url"`
	AnalyticsVhost       string `toml:"analytics_vhost"`
	AnalyticsPublicURL   string `toml:"analytics_public_url"`
	SessionCookieDomain  string `toml:"session_cookie_domain"`
	TelemetryRegisterURL string `toml:"telemetry_register_url"`
	TelemetryIngestURL   string `toml:"telemetry_ingest_url"`
}

// Load reads configuration from the first discovered config file, then applies
// environment variable overrides.
func Load() (Config, error) {
	fc := fileConfig{
		Port:    defaultPort,
		DataDir: defaultDataDir,
		Actor:   defaultActor,
	}

	configPath, err := findConfigFile()
	if err != nil {
		return Config{}, err
	}
	if configPath != "" {
		if err := loadFile(configPath, &fc); err != nil {
			return Config{}, fmt.Errorf("load config %q: %w", configPath, err)
		}
	}

	cfg, err := mergeEnv(fc)
	if err != nil {
		return Config{}, err
	}
	cfg.ConfigPath = configPath
	return cfg, nil
}

func findConfigFile() (string, error) {
	if explicit := os.Getenv("NITPUB_CONFIG"); explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("NITPUB_CONFIG %q: %w", explicit, err)
		}
		return explicit, nil
	}

	home, _ := os.UserHomeDir()
	candidates := []string{
		legacyConfigName,
		filepath.Join(home, ".config", "nitpub", defaultConfigName),
		filepath.Join("/etc", "nitpub", defaultConfigName),
		// common VPS layout when admin runs as root but config lives under the service user
		filepath.Join("/var/lib/nitpub", ".config", "nitpub", defaultConfigName),
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil
}

func loadFile(path string, fc *fileConfig) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(raw, fc)
}

func mergeEnv(fc fileConfig) (Config, error) {
	domain := fc.Domain
	if v := os.Getenv("NITPUB_DOMAIN"); v != "" {
		domain = v
	}
	if domain == "" {
		return Config{}, fmt.Errorf("domain is required (set domain in config or NITPUB_DOMAIN)")
	}

	port := fc.Port
	if fc.Port == 0 {
		port = defaultPort
	}
	if raw := os.Getenv("NITPUB_PORT"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("NITPUB_PORT: %w", err)
		}
		port = p
	}

	dataDir := fc.DataDir
	if dataDir == "" {
		dataDir = defaultDataDir
	}
	if raw := os.Getenv("NITPUB_DATA_DIR"); raw != "" {
		dataDir = raw
	}

	actor := fc.Actor
	if actor == "" {
		actor = defaultActor
	}
	if raw := os.Getenv("NITPUB_ACTOR"); raw != "" {
		actor = raw
	}

	title := fc.Title
	if v := os.Getenv("NITPUB_TITLE"); v != "" {
		title = v
	}
	if title == "" {
		title = "nitpub"
	}

	httpDev := fc.HTTP || os.Getenv("NITPUB_HTTP") == "1"

	secret := fc.Secret
	if v := os.Getenv("NITPUB_SECRET"); v != "" {
		secret = v
	}
	if secret == "" {
		if httpDev {
			secret = defaultDevSecret
		} else {
			return Config{}, fmt.Errorf("secret is required in production (set secret in config or NITPUB_SECRET)")
		}
	}
	if !httpDev && secret == defaultDevSecret {
		return Config{}, fmt.Errorf("secret must not be the default dev secret in production")
	}

	systemUser := fc.SystemUser
	if v := os.Getenv("NITPUB_SYSTEM_USER"); v != "" {
		systemUser = v
	}

	telemetryRegisterURL := fc.TelemetryRegisterURL
	if telemetryRegisterURL == "" {
		telemetryRegisterURL = defaultTelemetryRegisterURL
	}
	if v := os.Getenv("NITPUB_TELEMETRY_REGISTER_URL"); v != "" {
		telemetryRegisterURL = v
	}

	telemetryIngestURL := fc.TelemetryIngestURL
	if telemetryIngestURL == "" {
		telemetryIngestURL = defaultTelemetryIngestURL
	}
	if v := os.Getenv("NITPUB_TELEMETRY_INGEST_URL"); v != "" {
		telemetryIngestURL = v
	}

	scheme := "https"
	if httpDev {
		scheme = "http"
	}

	baseURL := fmt.Sprintf("%s://%s", scheme, domain)
	if httpDev && port != 80 {
		// Production runs behind a reverse proxy terminating on 80/443, so the
		// browser's Origin header omits the port and BaseURL must match that.
		// httpDev serves the port directly to the browser, so WebAuthn origin
		// verification (SetRPOrigin) needs it included here.
		baseURL = fmt.Sprintf("%s:%d", baseURL, port)
	}

	return Config{
		Domain:               domain,
		Title:                title,
		Port:                 port,
		DataDir:              dataDir,
		Actor:                actor,
		Secret:               secret,
		BaseURL:              baseURL,
		HTTPDev:              httpDev,
		SystemUser:           systemUser,
		AnalyticsEnabled:     fc.AnalyticsEnabled,
		AnalyticsAPIToken:    fc.AnalyticsAPIToken,
		AnalyticsBaseURL:     fc.AnalyticsBaseURL,
		AnalyticsVhost:       fc.AnalyticsVhost,
		AnalyticsPublicURL:   fc.AnalyticsPublicURL,
		SessionCookieDomain:  fc.SessionCookieDomain,
		TelemetryRegisterURL: telemetryRegisterURL,
		TelemetryIngestURL:   telemetryIngestURL,
	}, nil
}

// EnsureDataDirWritable returns an error when the data directory cannot be used.
func EnsureDataDirWritable(cfg Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir %q: %w", cfg.DataDir, err)
	}
	test := filepath.Join(cfg.DataDir, ".write-test")
	if err := os.WriteFile(test, []byte("ok"), 0o600); err != nil {
		hint := ""
		if cfg.SystemUser != "" {
			hint = fmt.Sprintf("; run admin commands as user %q", cfg.SystemUser)
		}
		return fmt.Errorf("cannot write data dir %q%s: %w", cfg.DataDir, hint, err)
	}
	_ = os.Remove(test)
	return nil
}

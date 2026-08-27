package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nitpub.toml")
	content := `
domain = "file.test"
port = 9090
data_dir = "` + filepath.Join(dir, "data") + `"
actor = "writer"
secret = "file-secret"
http = true
system_user = "nitpub"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NITPUB_CONFIG", path)
	t.Setenv("NITPUB_DOMAIN", "")
	t.Setenv("NITPUB_PORT", "")
	t.Setenv("NITPUB_DATA_DIR", "")
	t.Setenv("NITPUB_ACTOR", "")
	t.Setenv("NITPUB_SECRET", "")
	t.Setenv("NITPUB_HTTP", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domain != "file.test" || cfg.Port != 9090 || cfg.Actor != "writer" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.SystemUser != "nitpub" {
		t.Fatalf("system user = %q", cfg.SystemUser)
	}
	if cfg.BaseURL != "http://file.test:9090" {
		t.Fatalf("base URL = %q", cfg.BaseURL)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nitpub.toml")
	if err := os.WriteFile(path, []byte(`domain = "file.test"
secret = "file-secret"
http = true`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NITPUB_CONFIG", path)
	t.Setenv("NITPUB_DOMAIN", "override.test")
	t.Setenv("NITPUB_HTTP", "1")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domain != "override.test" {
		t.Fatalf("domain = %q", cfg.Domain)
	}
}

func TestEnsureDataDirWritable(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DataDir: dir, SystemUser: "nitpub"}
	if err := EnsureDataDirWritable(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestLoadRequiresDomain(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when domain is missing")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_PORT", "")
	t.Setenv("NITPUB_DATA_DIR", "")
	t.Setenv("NITPUB_ACTOR", "")
	t.Setenv("NITPUB_HTTP", "1")
	t.Setenv("NITPUB_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != defaultPort {
		t.Fatalf("port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.DataDir != defaultDataDir {
		t.Fatalf("data dir = %q, want %q", cfg.DataDir, defaultDataDir)
	}
	if cfg.Actor != defaultActor {
		t.Fatalf("actor = %q, want %q", cfg.Actor, defaultActor)
	}
	if cfg.BaseURL != "http://example.test:8080" {
		t.Fatalf("base URL = %q", cfg.BaseURL)
	}
}

func TestLoadInvalidPort(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_PORT", "not-a-port")
	t.Setenv("NITPUB_HTTP", "1")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestLoadAnalyticsDefaultsOff(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_HTTP", "1")
	t.Setenv("NITPUB_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AnalyticsEnabled {
		t.Fatal("analytics should default to disabled")
	}
}

func TestLoadAnalyticsEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nitpub.toml")
	content := `
domain = "file.test"
secret = "file-secret"
http = true
analytics_enabled = true
analytics_api_token = "tok_abc123"
analytics_base_url = "http://127.0.0.1:8081"
analytics_vhost = "nitpub-internal"
analytics_public_url = "https://stats.file.test"
session_cookie_domain = ".file.test"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NITPUB_CONFIG", path)
	t.Setenv("NITPUB_DOMAIN", "")
	t.Setenv("NITPUB_SECRET", "")
	t.Setenv("NITPUB_HTTP", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AnalyticsEnabled {
		t.Fatal("expected analytics enabled")
	}
	if cfg.AnalyticsAPIToken != "tok_abc123" {
		t.Fatalf("api token = %q", cfg.AnalyticsAPIToken)
	}
	if cfg.AnalyticsBaseURL != "http://127.0.0.1:8081" {
		t.Fatalf("base url = %q", cfg.AnalyticsBaseURL)
	}
	if cfg.AnalyticsVhost != "nitpub-internal" {
		t.Fatalf("vhost = %q", cfg.AnalyticsVhost)
	}
	if cfg.AnalyticsPublicURL != "https://stats.file.test" {
		t.Fatalf("public url = %q", cfg.AnalyticsPublicURL)
	}
	if cfg.SessionCookieDomain != ".file.test" {
		t.Fatalf("session cookie domain = %q", cfg.SessionCookieDomain)
	}
}

func TestLoadSessionCookieDomainDefaultsEmpty(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_HTTP", "1")
	t.Setenv("NITPUB_SECRET", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SessionCookieDomain != "" {
		t.Fatalf("session cookie domain = %q, want empty by default", cfg.SessionCookieDomain)
	}
	if cfg.AnalyticsPublicURL != "" {
		t.Fatalf("analytics public url = %q, want empty by default", cfg.AnalyticsPublicURL)
	}
}

func TestLoadTelemetryDefaultsToProjectCollector(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_HTTP", "1")
	t.Setenv("NITPUB_SECRET", "")
	t.Setenv("NITPUB_TELEMETRY_REGISTER_URL", "")
	t.Setenv("NITPUB_TELEMETRY_INGEST_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TelemetryRegisterURL != defaultTelemetryRegisterURL {
		t.Fatalf("telemetry register url = %q, want %q", cfg.TelemetryRegisterURL, defaultTelemetryRegisterURL)
	}
	if cfg.TelemetryIngestURL != defaultTelemetryIngestURL {
		t.Fatalf("telemetry ingest url = %q, want %q", cfg.TelemetryIngestURL, defaultTelemetryIngestURL)
	}
}

func TestLoadTelemetryEnvOverridesDefault(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_HTTP", "1")
	t.Setenv("NITPUB_SECRET", "")
	t.Setenv("NITPUB_TELEMETRY_REGISTER_URL", "https://own-receiver.test/register")
	t.Setenv("NITPUB_TELEMETRY_INGEST_URL", "https://own-receiver.test/v1/metrics")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TelemetryRegisterURL != "https://own-receiver.test/register" {
		t.Fatalf("telemetry register url = %q, want operator override", cfg.TelemetryRegisterURL)
	}
	if cfg.TelemetryIngestURL != "https://own-receiver.test/v1/metrics" {
		t.Fatalf("telemetry ingest url = %q, want operator override", cfg.TelemetryIngestURL)
	}
}

func TestLoadRequiresSecretInProduction(t *testing.T) {
	t.Setenv("NITPUB_CONFIG", "")
	t.Setenv("NITPUB_DOMAIN", "example.test")
	t.Setenv("NITPUB_HTTP", "")
	t.Setenv("NITPUB_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error without secret in production mode")
	}
}

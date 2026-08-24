package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// IsDebianFamily reports whether /etc/os-release looks like Debian/Ubuntu.
func IsDebianFamily() bool {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	s := string(data)
	return strings.Contains(s, "ID=debian") ||
		strings.Contains(s, "ID=ubuntu") ||
		strings.Contains(s, "ID_LIKE=debian") ||
		strings.Contains(s, "ID_LIKE=\"debian")
}

// WriteConfigIfMissing writes a minimal config.toml when path does not exist.
func WriteConfigIfMissing(path string, domain, title, actor, secret, dataDir string, port int) (wrote bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if port <= 0 {
		port = 8080
	}
	if dataDir == "" {
		dataDir = "/var/lib/nitpub"
	}
	if title == "" {
		title = "My Blog"
	}
	body := fmt.Sprintf(`domain = %q
title = %q
port = %d
data_dir = %q
actor = %q
secret = %q
http = false
system_user = "nitpub"
`, domain, title, port, dataDir, actor, secret)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		return false, err
	}
	return true, nil
}

// EnsureFederationSiteTOML creates site.toml with cross_post_default when missing.
// If site.toml already exists, returns skipped=true without modification.
func EnsureFederationSiteTOML(dataDir string, crossPost bool) (skipped bool, err error) {
	siteDir := filepath.Join(dataDir, "site")
	path := filepath.Join(siteDir, "site.toml")
	if _, err := os.Stat(path); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		return false, err
	}
	type fed struct {
		CrossPostDefault bool `toml:"cross_post_default"`
	}
	type manifest struct {
		Federation fed `toml:"federation"`
	}
	b, err := toml.Marshal(manifest{Federation: fed{CrossPostDefault: crossPost}})
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return false, err
	}
	return false, nil
}

// ScaffoldAnalytics enables analytics keys in an existing config file without inventing tokens.
// If analytics_enabled is already true, skips. Does not overwrite an existing API token.
func ScaffoldAnalytics(configPath string) (skipped bool, err error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false, err
	}
	content := string(data)
	if strings.Contains(content, "analytics_enabled = true") {
		return true, nil
	}
	extra := `
# Scaffolded by nitpub install — set analytics_api_token after creating a GoatCounter site.
# See deploy/README.md § GoatCounter for full setup.
analytics_enabled = true
analytics_base_url = "http://127.0.0.1:8181"
# analytics_api_token = "CHANGE-ME"
# analytics_public_url = "https://stats.example.com"
`
	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return false, err
	}
	defer f.Close()
	if _, err := f.WriteString(extra); err != nil {
		return false, err
	}
	return false, nil
}

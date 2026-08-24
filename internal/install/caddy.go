package install

import (
	"fmt"
	"os"
	"strings"
)

// SiteBlockPresent reports whether domain already has a site block in the Caddyfile.
// Matches literal hostname or {$DOMAIN} style templates.
func SiteBlockPresent(caddyfile, domain string) (bool, error) {
	data, err := os.ReadFile(caddyfile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return SiteBlockInContent(string(data), domain)
}

// SiteBlockInContent reports whether domain already has a site block in content.
func SiteBlockInContent(content, domain string) (bool, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return false, fmt.Errorf("domain is empty")
	}
	for _, needle := range []string{domain + " {", domain + "{", "{$DOMAIN} {", "{$DOMAIN}{"} {
		if strings.Contains(content, needle) {
			return true, nil
		}
	}
	return false, nil
}

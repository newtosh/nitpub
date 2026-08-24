package mastodon

import (
	"fmt"
	"net"
	"strings"
)

// ValidateInstanceHost rejects a visitor-supplied instance domain that
// would let StartCommentAuth be used as an SSRF proxy (KTD7): IP-literal
// hosts, and any hostname that resolves to a private, loopback,
// link-local, or multicast address. Domain is a bare host (e.g.
// "mastodon.social"), not a URL — nitpub always dials it over https.
func ValidateInstanceHost(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("instance domain is empty")
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
		return fmt.Errorf("instance domain must be a bare host, not a URL")
	}
	host := domain
	if h, _, err := net.SplitHostPort(domain); err == nil {
		host = h
	}
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Errorf("instance domain must not be an IP literal")
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve instance domain: %w", err)
	}
	for _, addr := range addrs {
		if isDisallowedIP(addr) {
			return fmt.Errorf("instance domain resolves to a disallowed address")
		}
	}
	return nil
}

func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

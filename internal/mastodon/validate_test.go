package mastodon

import (
	"context"
	"testing"
)

func TestValidateInstanceHostRejectsURL(t *testing.T) {
	if err := ValidateInstanceHost("https://mastodon.social"); err == nil {
		t.Fatal("expected error for a full URL, not a bare host")
	}
}

func TestValidateInstanceHostRejectsIPLiteral(t *testing.T) {
	if err := ValidateInstanceHost("1.2.3.4"); err == nil {
		t.Fatal("expected error for an IP-literal host")
	}
}

func TestValidateInstanceHostRejectsLoopback(t *testing.T) {
	if err := ValidateInstanceHost("localhost"); err == nil {
		t.Fatal("expected error for localhost")
	}
}

func TestValidateInstanceHostRejectsMetadataAddress(t *testing.T) {
	// 169.254.169.254 is the cloud-metadata IP; as a bare IP literal it's
	// already rejected before DNS resolution, which is the point.
	if err := ValidateInstanceHost("169.254.169.254"); err == nil {
		t.Fatal("expected error for link-local metadata address")
	}
}

func TestValidateInstanceHostRejectsUnresolvable(t *testing.T) {
	if err := ValidateInstanceHost("this-domain-should-not-resolve.invalid"); err == nil {
		t.Fatal("expected error for an unresolvable domain")
	}
}

func TestValidateInstanceHostRejectsEmpty(t *testing.T) {
	if err := ValidateInstanceHost(""); err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestSecureDialContextRejectsLoopbackAtDialTime(t *testing.T) {
	// NewClient()'s dialer must re-validate at the moment of every dial,
	// not only once via ValidateInstanceHost before registration — that
	// one-time check is exactly what a DNS-rebinding attack bypasses.
	// "localhost" always resolves to a loopback address, so this proves
	// the dialer itself blocks the connection independent of any earlier
	// hostname-level check.
	c := NewClient()
	_, err := c.RegisterApp(context.Background(), "localhost:1", "https://nitpub.example/comment/callback", DefaultScope)
	if err == nil {
		t.Fatal("expected secureDialContext to reject a loopback-resolving dial")
	}
}

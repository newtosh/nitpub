package actor

import "testing"

func TestAcctMatches(t *testing.T) {
	tests := []struct {
		resource string
		want     bool
	}{
		{"acct:nit@nitpub.com", true},
		{"acct:Nit@nitpub.com", true},
		{"acct:nit@NITPUB.COM", true},
		{"@nit@nitpub.com", true},
		{"nit@nitpub.com", true},
		{"acct:other@nitpub.com", false},
		{"acct:nit@other.com", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := acctMatches(tc.resource, "nit", "nitpub.com"); got != tc.want {
			t.Errorf("acctMatches(%q) = %v, want %v", tc.resource, got, tc.want)
		}
	}
}

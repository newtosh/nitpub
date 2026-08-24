package auth

import "testing"

func TestNormalizeThemeID(t *testing.T) {
	if got := NormalizeThemeID("nord"); got != "nord" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeThemeID("midnight"); got != "tokyo-night" {
		t.Fatalf("legacy alias got %q", got)
	}
	if got := NormalizeThemeID("bogus"); got != DefaultThemeID {
		t.Fatalf("got %q", got)
	}
}

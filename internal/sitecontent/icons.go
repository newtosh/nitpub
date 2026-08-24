package sitecontent

// ValidIcon reports whether name is an allowed Lucide icon slug for nav items.
func ValidIcon(name string) bool {
	_, ok := allowedIcons[name]
	return ok
}

// allowedIcons mirrors icons exposed in the PWA nav (subset of Lucide).
var allowedIcons = map[string]struct{}{
	"user":          {},
	"newspaper":     {},
	"link":          {},
	"links":         {},
	"folder":        {},
	"book":          {},
	"book-open":     {},
	"home":          {},
	"rss":           {},
	"github":        {},
	"globe":         {},
	"mail":          {},
	"info":          {},
	"file-text":     {},
	"briefcase":     {},
	"code":          {},
	"search":        {},
	"external-link": {},
}

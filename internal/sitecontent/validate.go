package sitecontent

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

var reservedFirstSegments = map[string]struct{}{
	"api":         {},
	"actor":       {},
	"inbox":       {},
	"outbox":      {},
	".well-known": {},
	"feed.xml":    {},
	"healthz":     {},
	"author":      {},
	"admin":       {},
	"login":       {},
	"p":           {},
	"posts":       {},
	"search":      {},
	"assets":      {},
}

func normalizeNavPath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	clean := path.Clean(p)
	if clean == "/" {
		return "/", nil
	}
	if strings.Contains(p, "..") {
		return "", fmt.Errorf("invalid path %q", p)
	}
	seg := strings.TrimPrefix(clean, "/")
	first := strings.Split(seg, "/")[0]
	if _, ok := reservedNavSegments[first]; ok {
		return "", fmt.Errorf("reserved path %q", p)
	}
	return clean, nil
}

// reservedNavSegments blocks nav links to AP/system routes, not first-party app pages.
var reservedNavSegments = map[string]struct{}{
	"api":         {},
	"actor":       {},
	"inbox":       {},
	"outbox":      {},
	".well-known": {},
	"feed.xml":    {},
	"healthz":     {},
	"assets":      {},
	"media":       {},
}

func normalizePagePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	clean := path.Clean(p)
	if clean == "/" {
		return "", fmt.Errorf("path cannot be /")
	}
	if clean != p && clean+"/" != p {
		// path.Clean may collapse; ensure no trailing junk
		if strings.Contains(p, "..") {
			return "", fmt.Errorf("invalid path %q", p)
		}
	}
	seg := strings.TrimPrefix(clean, "/")
	if seg == "" {
		return "", fmt.Errorf("invalid path %q", p)
	}
	first := strings.Split(seg, "/")[0]
	if _, ok := reservedFirstSegments[first]; ok {
		return "", fmt.Errorf("reserved path %q", p)
	}
	return clean, nil
}

func safeRelFile(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("empty file path")
	}
	if path.IsAbs(rel) {
		return "", fmt.Errorf("absolute file path not allowed")
	}
	clean := path.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes site root")
	}
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("invalid file path %q", rel)
	}
	return clean, nil
}

func validateIcon(name string) error {
	if name == "" {
		return nil
	}
	if !ValidIcon(name) {
		return fmt.Errorf("unknown icon %q", name)
	}
	return nil
}

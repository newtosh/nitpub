package updatecheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Repo = "newtosh/nitpub"

var client = &http.Client{Timeout: 60 * time.Second}

// Release is a published GitHub release (or tag fallback).
type Release struct {
	Tag    string  `json:"tag_name"`
	URL    string  `json:"html_url"`
	Assets []Asset `json:"assets"`
}

// Asset is a release attachment.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Latest fetches the latest GitHub release for Repo. If the repo has no
// releases yet, it falls back to the most recent tag.
func Latest() (Release, error) {
	rel, err := latestRelease()
	if err == nil {
		return rel, nil
	}
	return latestTag()
}

func latestRelease() (Release, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nitpub-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github releases: unexpected status %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, err
	}
	if rel.Tag == "" {
		return Release{}, fmt.Errorf("github releases: no tag_name in response")
	}
	return rel, nil
}

func latestTag() (Release, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+Repo+"/tags", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "nitpub-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github tags: unexpected status %d", resp.StatusCode)
	}
	var tags []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return Release{}, err
	}
	if len(tags) == 0 {
		return Release{}, fmt.Errorf("github tags: repo has no tags")
	}
	return Release{
		Tag: tags[0].Name,
		URL: "https://github.com/" + Repo + "/releases/tag/" + tags[0].Name,
	}, nil
}

// ArchSuffix returns linux-amd64 / linux-arm64 for the running binary.
func ArchSuffix() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "linux-amd64", nil
	case "arm64":
		return "linux-arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture %s", runtime.GOARCH)
	}
}

// BinaryAssetName is the stable release asset name for a tag + arch suffix.
func BinaryAssetName(tag, archSuffix string) string {
	return fmt.Sprintf("nitpub-%s-%s", strings.TrimSpace(tag), archSuffix)
}

// FindAsset returns the named asset from a release.
func FindAsset(rel Release, name string) (Asset, error) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s has no asset %q", rel.Tag, name)
}

// ParseSHA256SUMS maps filenames to hex digests from a SHA256SUMS body.
func ParseSHA256SUMS(body string) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum, name := fields[0], fields[len(fields)-1]
		name = strings.TrimPrefix(name, "./")
		out[name] = strings.ToLower(sum)
	}
	return out
}

// DownloadFile fetches url into destPath.
func DownloadFile(url, destPath string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "nitpub-update-check")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

// FileSHA256 returns the hex sha256 of path.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifySHA256 fails closed when expected is non-empty and does not match.
func VerifySHA256(path, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return fmt.Errorf("missing checksum")
	}
	got, err := FileSHA256(path)
	if err != nil {
		return err
	}
	if got != expected {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", path, got, expected)
	}
	return nil
}

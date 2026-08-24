package media

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const maxUploadBytes = 10 << 20 // 10 MiB

var safeName = regexp.MustCompile(`^[a-f0-9-]+\.(jpe?g|png|gif|webp|ico)$`)

// Service stores uploaded media on disk.
type Service struct {
	dir string
}

// New creates the media directory under the instance data dir.
func New(dataDir string) (*Service, error) {
	dir := filepath.Join(dataDir, "media")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create media dir: %w", err)
	}
	return &Service{dir: dir}, nil
}

// Save writes an image upload and returns its public filename.
func (s *Service) Save(r io.Reader, declaredType string, size int64) (string, error) {
	if size <= 0 {
		return "", fmt.Errorf("empty upload")
	}
	if size > maxUploadBytes {
		return "", fmt.Errorf("file exceeds %d byte limit", maxUploadBytes)
	}

	limited := io.LimitReader(r, maxUploadBytes+1)
	sniff := make([]byte, 512)
	n, err := limited.Read(sniff)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read upload: %w", err)
	}
	if n == 0 {
		return "", fmt.Errorf("empty upload")
	}
	sniff = sniff[:n]

	ext, contentType, ok := imageExt(sniff, declaredType)
	if !ok {
		return "", fmt.Errorf("unsupported image type")
	}

	name := uuid.NewString() + ext
	path := filepath.Join(s.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(sniff); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if _, err := io.Copy(f, limited); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	_ = contentType
	return name, nil
}

// Open returns a readable media file after validating the filename.
func (s *Service) Open(name string) (*os.File, string, error) {
	if !safeName.MatchString(name) {
		return nil, "", fmt.Errorf("invalid media name")
	}
	path := filepath.Join(s.dir, name)
	if filepath.Dir(path) != s.dir {
		return nil, "", fmt.Errorf("invalid media path")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	ext := filepath.Ext(name)
	var contentType string
	if ext == ".ico" {
		// mime.TypeByExtension relies on the OS's mime.types file, which
		// often has no entry for .ico at all — fall back to
		// application/octet-stream in that case, which browsers won't
		// reliably treat as a favicon.
		contentType = "image/x-icon"
	} else {
		contentType = mime.TypeByExtension(ext)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return f, contentType, nil
}

func imageExt(sniff []byte, declared string) (ext, contentType string, ok bool) {
	switch {
	case len(sniff) >= 3 && sniff[0] == 0xFF && sniff[1] == 0xD8 && sniff[2] == 0xFF:
		return ".jpg", "image/jpeg", true
	case len(sniff) >= 8 && string(sniff[:8]) == "\x89PNG\r\n\x1a\n":
		return ".png", "image/png", true
	case len(sniff) >= 6 && (string(sniff[:6]) == "GIF87a" || string(sniff[:6]) == "GIF89a"):
		return ".gif", "image/gif", true
	case len(sniff) >= 12 && string(sniff[0:4]) == "RIFF" && string(sniff[8:12]) == "WEBP":
		return ".webp", "image/webp", true
	// ICONDIR header: reserved=0x0000, type=0x0001 (icon, as opposed to
	// 0x0002 for cursor). Multi-resolution frames all live inside this
	// one file — nothing extra needed to support them, since uploads are
	// stored and served as-is with no re-encoding.
	case len(sniff) >= 4 && sniff[0] == 0x00 && sniff[1] == 0x00 && sniff[2] == 0x01 && sniff[3] == 0x00:
		return ".ico", "image/x-icon", true
	}
	declared = strings.ToLower(strings.TrimSpace(declared))
	switch declared {
	case "image/jpeg", "image/jpg":
		return ".jpg", "image/jpeg", true
	case "image/png":
		return ".png", "image/png", true
	case "image/gif":
		return ".gif", "image/gif", true
	case "image/webp":
		return ".webp", "image/webp", true
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico", "image/x-icon", true
	default:
		return "", "", false
	}
}

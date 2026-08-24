package importpost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/newtosh/nitpub/internal/outbox"
)

// ParsedFile is a markdown file ready for import.
type ParsedFile struct {
	Path    string
	Kind    outbox.Kind
	Content string
}

// ParseFile reads a markdown file and determines kind + content.
func ParseFile(path string, defaultKind outbox.Kind) (ParsedFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ParsedFile{}, err
	}
	return ParseBytes(path, data, defaultKind)
}

// ParseBytes parses markdown content from memory (e.g. multipart upload).
func ParseBytes(name string, data []byte, defaultKind outbox.Kind) (ParsedFile, error) {
	content := string(data)
	kind := defaultKind
	if strings.HasPrefix(content, "---\n") {
		parts := strings.SplitN(content, "\n---\n", 2)
		if len(parts) == 2 {
			meta := parts[0]
			content = parts[1]
			for _, line := range strings.Split(meta, "\n") {
				line = strings.TrimSpace(strings.TrimPrefix(line, "---"))
				if strings.HasPrefix(line, "kind:") {
					k := strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
					switch strings.ToLower(k) {
					case "note":
						kind = outbox.KindNote
					case "article":
						kind = outbox.KindArticle
					}
				}
			}
		}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ParsedFile{}, fmt.Errorf("empty content")
	}
	if kind == "" {
		kind = inferKind(content)
	}
	return ParsedFile{Path: name, Kind: kind, Content: content}, nil
}

func inferKind(content string) outbox.Kind {
	first := strings.TrimSpace(strings.Split(content, "\n")[0])
	if strings.HasPrefix(first, "# ") {
		return outbox.KindNote
	}
	// Article: first line title, rest body
	if strings.Contains(content, "\n") {
		return outbox.KindArticle
	}
	return outbox.KindNote
}

// ImportDir parses all .md files in a directory (non-recursive).
func ImportDir(dir string, defaultKind outbox.Kind) ([]ParsedFile, []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}
	var files []ParsedFile
	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		p, err := ParseFile(filepath.Join(dir, e.Name()), defaultKind)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", e.Name(), err))
			continue
		}
		files = append(files, p)
	}
	return files, errs
}

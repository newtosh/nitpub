package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/newtosh/nitpub/internal/importpost"
	"github.com/newtosh/nitpub/internal/outbox"
)

type importPostsResponse struct {
	Imported int      `json:"imported"`
	Errors   []string `json:"errors"`
}

func (h *Handler) AdminImportPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.auth.Authenticated(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	defaultKind := outbox.Kind(strings.ToLower(r.FormValue("kind")))
	if defaultKind == "" {
		defaultKind = outbox.KindArticle
	}

	var resp importPostsResponse
	for _, headers := range r.MultipartForm.File {
		for _, fh := range headers {
			if !strings.HasSuffix(strings.ToLower(fh.Filename), ".md") {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: not a .md file", fh.Filename))
				continue
			}
			f, err := fh.Open()
			if err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", fh.Filename, err))
				continue
			}
			data, err := io.ReadAll(io.LimitReader(f, 2<<20))
			_ = f.Close()
			if err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", fh.Filename, err))
				continue
			}
			parsed, err := importpost.ParseBytes(fh.Filename, data, defaultKind)
			if err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", fh.Filename, err))
				continue
			}
			if err := h.importOnePost(parsed, nil); err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", fh.Filename, err))
				continue
			}
			resp.Imported++
		}
	}
	if resp.Imported > 0 {
		h.rebuildSearchIndex()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) importOnePost(parsed importpost.ParsedFile, federate *bool) error {
	post, create, err := h.outbox.CreatePost(parsed.Kind, parsed.Content)
	if err != nil {
		return err
	}
	_, err = h.completeFederation(post, create, h.resolveFederate(federate))
	return err
}

// ImportPostsFromDir imports markdown files from a directory (CLI helper).
func ImportPostsFromDir(ob *outbox.Service, deliver func(activity any) error, dir string, defaultKind outbox.Kind) (importPostsResponse, error) {
	files, parseErrs := importpost.ImportDir(dir, defaultKind)
	var resp importPostsResponse
	for _, e := range parseErrs {
		resp.Errors = append(resp.Errors, e.Error())
	}
	for _, parsed := range files {
		post, create, err := ob.CreatePost(parsed.Kind, parsed.Content)
		if err != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", filepath.Base(parsed.Path), err))
			continue
		}
		if deliver != nil {
			pub := FederationPublisher{Outbox: ob, Deliver: deliver}
			if _, err := pub.Complete(post, create, true); err != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("%s: %v", filepath.Base(parsed.Path), err))
				continue
			}
		}
		resp.Imported++
	}
	return resp, nil
}

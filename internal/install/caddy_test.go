package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSiteBlockPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Caddyfile")
	present, err := SiteBlockPresent(path, "blog.example.com")
	if err != nil || present {
		t.Fatalf("empty: present=%v err=%v", present, err)
	}
	block := "\nother.example.com {\n\treverse_proxy localhost:9\n}\nblog.example.com {\n\treverse_proxy localhost:8080\n}\n"
	if err := os.WriteFile(path, []byte(block), 0o644); err != nil {
		t.Fatal(err)
	}
	present, err = SiteBlockPresent(path, "blog.example.com")
	if err != nil || !present {
		t.Fatalf("after write: present=%v err=%v", present, err)
	}
	if strings.Count(block, "blog.example.com {") != 1 {
		t.Fatalf("fixture should have one site block")
	}
}

func TestSiteBlockInContent(t *testing.T) {
	ok, err := SiteBlockInContent("blog.example.com {\n}\n", "blog.example.com")
	if err != nil || !ok {
		t.Fatalf("literal: ok=%v err=%v", ok, err)
	}
	ok, err = SiteBlockInContent("{$DOMAIN} {\n}\n", "blog.example.com")
	if err != nil || !ok {
		t.Fatalf("template: ok=%v err=%v", ok, err)
	}
	ok, err = SiteBlockInContent("other.com {\n}\n", "blog.example.com")
	if err != nil || ok {
		t.Fatalf("miss: ok=%v err=%v", ok, err)
	}
}

func TestSiteBlockPresent_templateVar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Caddyfile")
	if err := os.WriteFile(path, []byte("{$DOMAIN} {\n\treverse_proxy localhost:8080\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	present, err := SiteBlockPresent(path, "blog.example.com")
	if err != nil || !present {
		t.Fatalf("template should count as present: present=%v err=%v", present, err)
	}
}

package seo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHugoIntegrationRoutes(t *testing.T) {
	if _, err := exec.LookPath("hugo"); err != nil {
		if os.Getenv("HUGO_INTEGRATION") == "1" {
			t.Fatal("Hugo is required by HUGO_INTEGRATION=1")
		}
		t.Skip("Hugo not installed")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "hugo.toml"), []byte("baseURL = 'https://example.org/blog/'\n"), 0600)
	os.MkdirAll(filepath.Join(root, "layouts", "_default"), 0700)
	template := []byte("<html><head><title>{{ .Title }}</title></head><body>{{ .Content }}</body></html>")
	os.WriteFile(filepath.Join(root, "layouts", "_default", "single.html"), template, 0600)
	// Support both the legacy and current Hugo template lookup conventions.
	os.WriteFile(filepath.Join(root, "layouts", "single.html"), template, 0600)
	os.MkdirAll(filepath.Join(root, "content", "posts"), 0700)
	target := filepath.Join(root, "content", "posts", "article.md")
	for _, frontMatter := range []string{"draft: true", "slug: custom-slug", "url: /custom/page.html"} {
		md := "---\ntitle: Target article\n" + frontMatter + "\n---\n\nTarget content."
		os.WriteFile(target, []byte(md), 0600)
		html, err := renderHugoMarkdown(context.Background(), md, root, target)
		if err != nil || !strings.Contains(html, "<title>Target article</title>") {
			t.Fatalf("%s: %q %v", frontMatter, html, err)
		}
		data, _ := os.ReadFile(target)
		if string(data) != md {
			t.Fatal("source modified")
		}
	}
}

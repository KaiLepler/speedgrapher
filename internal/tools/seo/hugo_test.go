package seo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestHugoOutputPath(t *testing.T) {
	for _, tt := range []struct{ name, permalink, base, want string }{
		{"slug", "https://example.org/custom/", "", "custom/index.html"},
		{"url", "https://example.org/custom.html", "", "custom.html"},
		{"subdirectory", "https://example.org/blog/posts/hello/", "https://example.org/blog/", "posts/hello/index.html"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := hugoOutputPath([]byte("path,permalink\ncontent/post.md,"+tt.permalink+"\n"), "content/post.md", tt.base)
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	for _, list := range []string{
		"path,permalink\ncontent/other.md,https://example.org/\n",
		"path,permalink\ncontent/post.md,https://example.org/../../outside.html\n",
		"path,permalink\ncontent/post.md,https://example.org/a/\ncontent/post.md,https://example.org/b/\n",
		"path,permalink\ncontent/post.md,https://example.org/a/\ncontent/other.md,https://example.org/a/\n",
	} {
		if _, err := hugoOutputPath([]byte(list), "content/post.md", ""); err == nil {
			t.Fatalf("expected rejection of %q", list)
		}
	}
}

func TestSnapshotRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires privileges on Windows")
	}
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "external")); err != nil {
		t.Fatal(err)
	}
	if err := copyHugoSite(context.Background(), root, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("snapshot followed a link outside the site")
	}
}

func fakeHugo(t *testing.T, build string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires Unix")
	}
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1" in
config) printf '{"baseurl":"https://example.org/"}'; exit 0 ;;
list)
  file=$(find content -name 'speedgrapher-preview-*.md')
  printf 'path,permalink\n%s,https://example.org/preview/\n' "$file"
  exit 0 ;;
esac
` + build
	if err := os.WriteFile(filepath.Join(dir, "hugo"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestRenderHugoIsolatedConcurrent(t *testing.T) {
	fakeHugo(t, `
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--destination" ]; then dest=$2; shift; fi
  shift
done
mkdir -p "$dest/preview"
cat content/speedgrapher-preview-*.md > "$dest/preview/index.html"
`)
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "hugo.toml"), []byte(""), 0600)
	os.Mkdir(filepath.Join(root, "content"), 0700)
	existing := filepath.Join(root, "content", "speedgrapher_seo_temp.md")
	os.WriteFile(existing, []byte("original"), 0600)
	var wg sync.WaitGroup
	for _, text := range []string{"first request", "second request"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			got, err := renderHugoMarkdown(context.Background(), text, root, "")
			if err != nil || got != text {
				t.Errorf("render got %q, %v; want %q", got, err, text)
			}
		}(text)
	}
	wg.Wait()
	data, _ := os.ReadFile(existing)
	if string(data) != "original" {
		t.Fatal("original content changed")
	}
	entries, _ := os.ReadDir(filepath.Join(root, "content"))
	if len(entries) != 1 {
		t.Fatalf("left temporary files: %v", entries)
	}
}

func TestRenderHugoMissingOutput(t *testing.T) {
	fakeHugo(t, `while [ "$#" -gt 0 ]; do
if [ "$1" = "--destination" ]; then dest=$2; shift; fi
shift
done
mkdir -p "$dest"
printf 'unrelated home page' > "$dest/index.html"
`)
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "hugo.toml"), nil, 0600)
	_, err := renderHugoMarkdown(context.Background(), "test", root, "")
	if err == nil || !strings.Contains(err.Error(), "intended Hugo output") {
		t.Fatalf("expected missing intended output: %v", err)
	}
}

func TestRenderHugoCancellationCleanup(t *testing.T) {
	fakeHugo(t, "sleep 30\n")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "hugo.toml"), nil, 0600)
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	_, err := renderHugoMarkdown(ctx, "test", root, "")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancellation: %v", err)
	}
	entries, _ := os.ReadDir(temp)
	if len(entries) != 0 {
		t.Fatalf("render left temp directories: %v", entries)
	}
}

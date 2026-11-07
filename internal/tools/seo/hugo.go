package seo

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/KaiLepler/speedgrapher/internal/safeshell"
)

// Render from a private copy: neither content nor Hugo's generated resources,
// lock files, or caches should be written into the author's working tree.
func renderHugoMarkdown(ctx context.Context, markdown, workspaceDir, sourcePath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	root, err := findHugoRoot(workspaceDir)
	if err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp("", "speedgrapher-hugo-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	site := filepath.Join(temp, "site")
	if err := copyHugoSite(ctx, root, site); err != nil {
		return "", fmt.Errorf("copy Hugo site: %w", err)
	}
	content := filepath.Join(site, "content")
	if err := os.MkdirAll(content, 0755); err != nil {
		return "", err
	}
	var target string
	if sourcePath != "" {
		rel, err := filepath.Rel(root, sourcePath)
		if err != nil || !filepath.IsLocal(rel) || !strings.HasPrefix(filepath.ToSlash(rel), "content/") {
			return "", fmt.Errorf("Markdown path must be inside the Hugo site's content directory")
		}
		target = filepath.Join(site, rel)
	} else {
		f, err := os.CreateTemp(content, "speedgrapher-preview-*.md")
		if err != nil {
			return "", err
		}
		target = f.Name()
		if err := f.Close(); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(target, []byte(markdown), 0600); err != nil {
		return "", err
	}
	run := func(args ...string) ([]byte, error) {
		res, err := safeshell.ExecuteWithOptions(ctx, safeshell.Options{
			Dir: site, MaxOutputBytes: 10 << 20,
		}, "hugo", args...)
		if err != nil {
			return nil, fmt.Errorf("hugo %v: %w", args, err)
		}
		if len(res.Stdout) >= 10<<20 {
			return nil, fmt.Errorf("Hugo output exceeds limit")
		}
		return res.Stdout, nil
	}
	// Let Hugo resolve slugs, URLs, permalinks, and language paths itself.
	cfgData, err := run("config", "--format", "json")
	if err != nil {
		return "", err
	}
	var cfg struct {
		BaseURL string `json:"baseurl"`
	}
	if err := json.Unmarshal(cfgData, &cfg); err != nil {
		return "", fmt.Errorf("parse Hugo configuration: %w", err)
	}
	list, err := run("list", "all")
	if err != nil {
		return "", err
	}
	rel, _ := filepath.Rel(site, target)
	output, err := hugoOutputPath(list, filepath.ToSlash(rel), cfg.BaseURL)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(temp, "public")
	if _, err := run("--destination", dest, "--cacheDir", filepath.Join(temp, "cache"),
		"--buildDrafts", "--buildFuture", "--buildExpired", "--quiet"); err != nil {
		return "", err
	}
	file, err := os.Open(filepath.Join(dest, filepath.FromSlash(output)))
	if err != nil {
		return "", fmt.Errorf("intended Hugo output %q missing: %w", output, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil {
		return "", err
	}
	if len(data) > 10<<20 {
		return "", fmt.Errorf("rendered Hugo HTML exceeds 10 MiB")
	}
	return string(data), nil
}

func hugoOutputPath(data []byte, target, baseURL string) (string, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	header, err := reader.Read()
	if err != nil {
		return "", fmt.Errorf("parse Hugo page list: %w", err)
	}
	pathCol, urlCol := -1, -1
	for i, name := range header {
		switch name {
		case "path":
			pathCol = i
		case "permalink":
			urlCol = i
		}
	}
	if pathCol < 0 || urlCol < 0 {
		return "", fmt.Errorf("Hugo page list lacks path or permalink")
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	var output string
	outputs := make(map[string]int)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		u, err := url.Parse(row[urlCol])
		if err != nil {
			return "", err
		}
		p := u.Path
		basePath := strings.TrimSuffix(base.Path, "/")
		if basePath != "" && strings.HasPrefix(p, basePath+"/") {
			p = strings.TrimPrefix(p, basePath)
		}
		p = strings.TrimPrefix(p, "/")
		if p == "" || strings.HasSuffix(p, "/") {
			p += "index.html"
		}
		outputs[p]++
		if filepath.ToSlash(row[pathCol]) != target {
			continue
		}
		if output != "" {
			return "", fmt.Errorf("ambiguous Hugo outputs for %q", target)
		}
		if !filepath.IsLocal(filepath.FromSlash(p)) {
			return "", fmt.Errorf("unsafe Hugo output path %q", p)
		}
		if !strings.HasSuffix(p, ".html") {
			return "", fmt.Errorf("Hugo page does not have a supported HTML output: %q", p)
		}
		output = p
	}
	if output == "" {
		return "", fmt.Errorf("intended page %q missing from Hugo page list", target)
	}
	if outputs[output] != 1 {
		return "", fmt.Errorf("another Hugo page shares the intended output %q", output)
	}
	return output, nil
}

func copyHugoSite(ctx context.Context, source, dest string) error {
	var copied int64
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			if rel == "public" || filepath.ToSlash(rel) == "resources/_gen" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dest, rel), 0700)
		}
		// Do not follow links back into the original site or outside it.
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported non-regular site file %q", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		copied += info.Size()
		if copied > 256<<20 {
			return fmt.Errorf("Hugo site snapshot exceeds 256 MiB")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, rel), data, 0600)
	})
}

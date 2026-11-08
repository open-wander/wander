package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/open-wander/wander/site/templates"
)

var referencePattern = regexp.MustCompile(`(?:href|src)="([^"]+)"`)

func main() {
	if err := build("dist", "static"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(outputDir, staticDir string) error {
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("clean output: %w", err)
	}

	for _, page := range templates.Pages() {
		var rendered bytes.Buffer
		if err := page.Component.Render(context.Background(), &rendered); err != nil {
			return fmt.Errorf("render %s: %w", page.Output, err)
		}
		path := filepath.Join(outputDir, filepath.FromSlash(page.Output))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create page directory: %w", err)
		}
		if err := os.WriteFile(path, rendered.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", page.Output, err)
		}
	}

	if err := copyTree(staticDir, filepath.Join(outputDir, "assets")); err != nil {
		return fmt.Errorf("copy static assets: %w", err)
	}
	if err := validateInternalReferences(outputDir); err != nil {
		return err
	}
	return nil
}

func copyTree(source, destination string) error {
	return os.CopyFS(destination, os.DirFS(source))
}

func validateInternalReferences(outputDir string) error {
	return filepath.Walk(outputDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || filepath.Ext(path) != ".html" {
			return walkErr
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range referencePattern.FindAllSubmatch(content, -1) {
			reference := string(match[1])
			if !strings.HasPrefix(reference, "/") || strings.HasPrefix(reference, "//") {
				continue
			}
			reference = strings.SplitN(reference, "#", 2)[0]
			if reference == "" {
				continue
			}
			target := filepath.Join(outputDir, filepath.FromSlash(strings.TrimPrefix(reference, "/")))
			if strings.HasSuffix(reference, "/") {
				target = filepath.Join(target, "index.html")
			}
			if _, err := os.Stat(target); err != nil {
				return fmt.Errorf("%s references missing %s", path, reference)
			}
		}
		return nil
	})
}

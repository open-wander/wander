package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateInternalReferences(t *testing.T) {
	testDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(testDir, "wander", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPage(t, filepath.Join(testDir, "wander", "docs", "index.html"), `<a href="/">Home</a>`)
	writeTestPage(t, filepath.Join(testDir, "index.html"), `<a href="/wander/docs/">Docs</a>`)
	if err := validateInternalReferences(testDir); err != nil {
		t.Fatalf("expected valid references: %v", err)
	}
}

func TestValidateInternalReferencesRejectsInvalidRoute(t *testing.T) {
	for name, reference := range map[string]string{
		"missing":   "/missing/",
		"traversal": "/../../outside",
	} {
		t.Run(name, func(t *testing.T) {
			testDir := t.TempDir()
			writeTestPage(t, filepath.Join(testDir, "index.html"), `<a href="`+reference+`">Invalid</a>`)
			if err := validateInternalReferences(testDir); err == nil {
				t.Fatalf("expected %s reference error", name)
			}
		})
	}
}

func writeTestPage(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

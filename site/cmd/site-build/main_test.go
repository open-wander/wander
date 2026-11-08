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

func TestValidateInternalReferencesRejectsMissingRoute(t *testing.T) {
	testDir := t.TempDir()
	writeTestPage(t, filepath.Join(testDir, "index.html"), `<a href="/missing/">Missing</a>`)
	if err := validateInternalReferences(testDir); err == nil {
		t.Fatal("expected missing route error")
	}
}

func writeTestPage(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

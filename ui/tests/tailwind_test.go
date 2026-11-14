package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTailwindOutputExists verifies that the Tailwind CSS output file is generated
func TestTailwindOutputExists(t *testing.T) {
	outputPath := filepath.Join("..", "static", "css", "output.css")

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Tailwind output.css does not exist at %s. Run 'make build-css-dev' first.", outputPath)
	}
}

// TestTailwindOutputNotEmpty verifies that the output.css has content
func TestTailwindOutputNotEmpty(t *testing.T) {
	outputPath := filepath.Join("..", "static", "css", "output.css")

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat output.css: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("output.css is empty")
	}

	// Should be at least 10KB for Tailwind base styles
	if info.Size() < 10000 {
		t.Fatalf("output.css is suspiciously small (%d bytes)", info.Size())
	}
}

// TestTailwindBaseStyles verifies that Tailwind base styles are present
func TestTailwindBaseStyles(t *testing.T) {
	outputPath := filepath.Join("..", "static", "css", "output.css")

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output.css: %v", err)
	}

	cssContent := string(content)

	// Check for Tailwind's CSS reset variables (always present)
	requiredPatterns := []string{
		"--tw-", // Tailwind CSS variables
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(cssContent, pattern) {
			t.Errorf("output.css missing expected pattern: %s", pattern)
		}
	}
}

// TestTailwindCustomClasses verifies our custom classes from input.css
func TestTailwindCustomClasses(t *testing.T) {
	outputPath := filepath.Join("..", "static", "css", "output.css")

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output.css: %v", err)
	}

	cssContent := string(content)

	// HTMX indicator class is always included (defined in utilities layer)
	expectedClasses := []string{
		".htmx-indicator", // HTMX loading indicator
		"opacity",         // Should have opacity utilities
	}

	for _, class := range expectedClasses {
		if !strings.Contains(cssContent, class) {
			t.Errorf("output.css missing expected pattern: %s", class)
		}
	}
}

// TestTailwindConfigValid verifies the shared theme file and the source paths
// declared in input.css.
func TestTailwindConfigValid(t *testing.T) {
	configPath := filepath.Join("..", "static", "css", "tailwind-theme.css")

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read tailwind-theme.css: %v", err)
	}

	configContent := string(content)

	input, err := os.ReadFile(filepath.Join("..", "static", "css", "input.css"))
	if err != nil {
		t.Fatalf("Failed to read input.css: %v", err)
	}

	// Verify source paths are declared
	requiredPaths := []string{
		`@source "../../templates"`,
		`@source "../../handlers"`,
		`@source "../js"`,
	}

	for _, path := range requiredPaths {
		if !strings.Contains(string(input), path) {
			t.Errorf("input.css missing source path: %s", path)
		}
	}

	// Verify plugins are configured
	requiredPlugins := []string{
		"@tailwindcss/forms",
		"@tailwindcss/typography",
	}

	for _, plugin := range requiredPlugins {
		if !strings.Contains(configContent, plugin) {
			t.Errorf("tailwind-theme.css missing plugin: %s", plugin)
		}
	}

	// Verify custom colors are defined
	if !strings.Contains(configContent, "--color-primary-500") ||
		!strings.Contains(configContent, "--color-success-500") ||
		!strings.Contains(configContent, "--color-error-500") {
		t.Error("tailwind-theme.css missing custom color definitions")
	}
}

// TestInputCSSExists verifies the input.css source file exists
func TestInputCSSExists(t *testing.T) {
	inputPath := filepath.Join("..", "static", "css", "input.css")

	content, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("Failed to read input.css: %v", err)
	}

	inputContent := string(content)

	// Verify the shared Tailwind setup is imported
	requiredDirectives := []string{
		`@import "./tailwind-theme.css"`,
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(inputContent, directive) {
			t.Errorf("input.css missing Tailwind directive: %s", directive)
		}
	}
}

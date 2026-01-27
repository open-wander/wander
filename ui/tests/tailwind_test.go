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
		"--tw-",                    // Tailwind CSS variables
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
		".htmx-indicator",         // HTMX loading indicator
		"opacity",                 // Should have opacity utilities
	}

	for _, class := range expectedClasses {
		if !strings.Contains(cssContent, class) {
			t.Errorf("output.css missing expected pattern: %s", class)
		}
	}
}

// TestTailwindConfigValid verifies the tailwind.config.js exists and is readable
func TestTailwindConfigValid(t *testing.T) {
	configPath := filepath.Join("..", "tailwind.config.js")

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read tailwind.config.js: %v", err)
	}

	configContent := string(content)

	// Verify content paths are configured
	requiredPaths := []string{
		"./templates/**/*.templ",
		"./templates/**/*.go",
		"./handlers/**/*.go",
		"./static/js/**/*.js",
	}

	for _, path := range requiredPaths {
		if !strings.Contains(configContent, path) {
			t.Errorf("tailwind.config.js missing content path: %s", path)
		}
	}

	// Verify plugins are configured
	requiredPlugins := []string{
		"@tailwindcss/forms",
		"@tailwindcss/typography",
	}

	for _, plugin := range requiredPlugins {
		if !strings.Contains(configContent, plugin) {
			t.Errorf("tailwind.config.js missing plugin: %s", plugin)
		}
	}

	// Verify custom colors are defined
	if !strings.Contains(configContent, "primary") ||
	   !strings.Contains(configContent, "success") ||
	   !strings.Contains(configContent, "error") {
		t.Error("tailwind.config.js missing custom color definitions")
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

	// Verify Tailwind directives are present
	requiredDirectives := []string{
		"@tailwind base",
		"@tailwind components",
		"@tailwind utilities",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(inputContent, directive) {
			t.Errorf("input.css missing Tailwind directive: %s", directive)
		}
	}
}

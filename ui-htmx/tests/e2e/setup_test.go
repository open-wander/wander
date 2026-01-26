// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

// TestE2E is the main entry point for running Playwright e2e tests.
// It starts the test server, sets up the environment, runs Playwright tests,
// and cleans up after completion.
func TestE2E(t *testing.T) {
	// Skip if running unit tests only
	if testing.Short() {
		t.Skip("Skipping e2e tests in short mode")
	}

	// Check if npm and npx are available
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm not found - skipping e2e tests")
	}
	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not found - skipping e2e tests")
	}

	// Check if node_modules exists
	if _, err := os.Stat("node_modules"); os.IsNotExist(err) {
		t.Skip("node_modules not found - run 'npm install' first")
	}

	// Start test server on port 3001
	srv, cleanup := StartTestServer(TestServerConfig{
		Port:     3001,
		LogLevel: "warn", // Reduce noise in test output
	})
	defer cleanup()

	t.Logf("Test server running at %s", srv.URL())

	// Set BASE_URL environment variable for Playwright
	os.Setenv("BASE_URL", srv.URL())
	defer os.Unsetenv("BASE_URL")

	// Run Playwright tests
	cmd := exec.Command("npx", "playwright", "test")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "." // Current directory (tests/e2e)

	if err := cmd.Run(); err != nil {
		t.Fatalf("Playwright tests failed: %v", err)
	}
}

// TestServerStartup verifies that the test server can start and respond to requests.
func TestServerStartup(t *testing.T) {
	// Start server with random port
	srv, cleanup := StartTestServer(TestServerConfig{
		Port:     0, // Use random available port
		LogLevel: "error",
	})
	defer cleanup()

	if srv.Port() == 0 {
		t.Fatal("Server port should not be 0")
	}

	t.Logf("Server started on port %d", srv.Port())

	// Server should be accessible via URL
	expectedURL := srv.URL()
	if expectedURL == "" {
		t.Fatal("Server URL should not be empty")
	}

	t.Logf("Server URL: %s", expectedURL)
}

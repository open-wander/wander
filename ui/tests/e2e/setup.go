// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui/handlers"
)

// TestServerConfig holds configuration for the test server.
type TestServerConfig struct {
	// Port is the port to listen on. If 0, a random available port is used.
	Port int
	// Logger is the logger to use. If nil, a default logger is created.
	Logger hclog.Logger
	// LogLevel is the log level (trace, debug, info, warn, error).
	// Default is "error" to reduce noise in tests.
	LogLevel string
}

// TestServer represents a running test server instance.
type TestServer struct {
	server   *http.Server
	handler  *handlers.Handler
	router   *handlers.Router
	port     int
	logger   hclog.Logger
	shutdown context.CancelFunc
}

// StartTestServer starts a test server with the provided configuration.
// It returns a TestServer instance that can be used to access server details
// and a cleanup function that should be called to stop the server.
//
// Example usage:
//
//	srv, cleanup := e2e.StartTestServer(e2e.TestServerConfig{Port: 3001})
//	defer cleanup()
//
//	// Run tests against http://localhost:3001
func StartTestServer(config TestServerConfig) (*TestServer, func()) {
	// Set default log level
	if config.LogLevel == "" {
		config.LogLevel = "error"
	}

	// Create logger
	logger := config.Logger
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Name:   "ui-htmx-e2e",
			Level:  hclog.LevelFromString(config.LogLevel),
			Color:  hclog.AutoColor,
			Output: os.Stderr,
		})
	}

	// If port is 0, find an available port
	port := config.Port
	if port == 0 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic(fmt.Sprintf("failed to find available port: %v", err))
		}
		port = listener.Addr().(*net.TCPAddr).Port
		listener.Close()
	}

	// Create handler and router using the same pattern as cmd/dev/main.go
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	// Create server
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Create context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())

	testServer := &TestServer{
		server:   srv,
		handler:  h,
		router:   router,
		port:     port,
		logger:   logger,
		shutdown: cancel,
	}

	// Start server in background
	go func() {
		logger.Info("Test server starting", "address", addr, "url", testServer.URL())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Test server failed", "error", err)
		}
	}()

	// Wait for server to be ready
	if err := waitForServer(testServer.URL(), 5*time.Second); err != nil {
		cancel()
		panic(fmt.Sprintf("test server failed to start: %v", err))
	}

	logger.Info("Test server ready", "url", testServer.URL())

	// Return cleanup function
	cleanup := func() {
		logger.Info("Shutting down test server")

		// Create shutdown context with timeout
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		// Attempt graceful shutdown
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to gracefully shut down server", "error", err)
		}

		// Cancel context
		cancel()

		logger.Info("Test server stopped")
	}

	return testServer, cleanup
}

// URL returns the base URL of the test server.
func (ts *TestServer) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", ts.port)
}

// Port returns the port the test server is listening on.
func (ts *TestServer) Port() int {
	return ts.port
}

// Logger returns the test server's logger.
func (ts *TestServer) Logger() hclog.Logger {
	return ts.logger
}

// waitForServer waits for the server to be ready by polling the health endpoint.
func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/ui/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("server did not become ready within %v", timeout)
}

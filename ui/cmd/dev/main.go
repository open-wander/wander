// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ui/handlers"
)

func main() {
	// Parse command line flags
	port := flag.String("port", "3000", "Port to listen on")
	logLevel := flag.String("log-level", "info", "Log level (trace, debug, info, warn, error)")
	flag.Parse()

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "ui-htmx-dev",
		Level: hclog.LevelFromString(*logLevel),
		Color: hclog.AutoColor,
	})

	logger.Info("Starting UI development server", "port", *port)

	// Create handler and router
	h := handlers.NewHandler(logger)
	router := handlers.NewRouter(h)

	// Start server
	addr := fmt.Sprintf(":%s", *port)
	logger.Info("Server listening", "address", addr, "url", fmt.Sprintf("http://localhost:%s/ui/", *port))

	server := &http.Server{
		Addr:              addr,
		Handler:           router.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

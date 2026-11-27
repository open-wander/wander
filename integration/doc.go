// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

//go:build integration

// Package integration runs Wander's multi-node tests against a real cluster in
// Docker: three Consul servers, a dev Vault, three Wander servers and three
// Wander clients, started from compose.yml through Testcontainers.
//
// TestMain builds the node image from the checkout's binary, starts the stack
// once, and removes it when the run ends. Tests share that cluster and undo
// their own changes with t.Cleanup.
package integration

// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

// -------------------------------------------------------------------------------
// Integration Cluster Fixture
//
// Project: Wander
//
// TestMain owns the cluster every test runs against. It clears the operator's
// NOMAD_*, WANDER_*, CONSUL_* and VAULT_* variables, builds the node image from
// the checkout's linux/amd64 binary, starts compose.yml through Testcontainers,
// and removes the stack when the run ends. A failed run saves each container's
// log, and the Consul and dockerd logs from inside the nodes, under results/.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/compose"

	"github.com/open-wander/wander/api"
)

// -------------------------------------------------------------------------
// CONSTANTS
// -------------------------------------------------------------------------

const (
	consulImage  = "hashicorp/consul:2.1.0@sha256:259fd02f2544fe1aa03fa49efd3b105c8dd67106d710a72f5e898110266356f7"
	nodeRepo     = "wander-it-node"
	nodeTag      = "local"
	nodeImage    = nodeRepo + ":" + nodeTag
	stackName    = "wander-integration"
	wanderBinary = "../pkg/integration/wander"
	upTimeout    = 5 * time.Minute
)

// clusterEnvPrefixes are the variable prefixes that can point a client at a
// real cluster.
var clusterEnvPrefixes = []string{"NOMAD_", "WANDER_", "CONSUL_", "VAULT_"}

// serverNames lists the Wander server services in compose.yml.
var serverNames = []string{"wander-server-1", "wander-server-2", "wander-server-3"}

// servers holds an API client for each Wander server, keyed by service name.
// TestMain fills it before any test runs.
var servers map[string]*api.Client

// -------------------------------------------------------------------------
// FIXTURE
// -------------------------------------------------------------------------

// TestMain starts the cluster, runs the tests and exits with their result.
func TestMain(m *testing.M) {
	os.Exit(run(m))
}

// run is the body of TestMain, kept separate so deferred teardown runs before
// os.Exit.
func run(m *testing.M) int {
	scrubClusterEnv()
	ctx := context.Background()

	if err := buildNodeImage(ctx); err != nil {
		return fail(err)
	}

	stack, err := compose.NewDockerComposeWith(
		compose.WithStackFiles("compose.yml"),
		compose.StackIdentifier(stackName),
	)
	if err != nil {
		return fail(fmt.Errorf("load compose.yml: %w", err))
	}
	stack.WithEnv(map[string]string{"CONSUL_IMAGE": consulImage, "NODE_IMAGE": nodeImage})

	defer func() {
		err := stack.Down(context.Background(), compose.RemoveOrphans(true), compose.RemoveVolumes(true))
		if err != nil {
			fmt.Fprintf(os.Stderr, "integration: stack down: %v\n", err)
		}
	}()

	upCtx, cancel := context.WithTimeout(ctx, upTimeout)
	defer cancel()
	if err := stack.Up(upCtx, compose.Wait(true)); err != nil {
		saveLogs(ctx, stack)
		return fail(fmt.Errorf("stack up: %w", err))
	}

	if servers, err = serverClients(ctx, stack); err != nil {
		saveLogs(ctx, stack)
		return fail(err)
	}

	code := m.Run()
	if code != 0 {
		saveLogs(ctx, stack)
	}
	return code
}

// scrubClusterEnv unsets every variable with a cluster prefix before anything
// reads the environment, so no client can reach the operator's real cluster.
func scrubClusterEnv() {
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		for _, prefix := range clusterEnvPrefixes {
			if strings.HasPrefix(name, prefix) {
				os.Unsetenv(name)
			}
		}
	}
}

// fail reports a setup error and returns the exit code for it.
func fail(err error) int {
	fmt.Fprintf(os.Stderr, "integration: %v\n", err)
	return 1
}

// -------------------------------------------------------------------------
// NODE IMAGE
// -------------------------------------------------------------------------

// buildNodeImage builds node/Dockerfile around the checkout's wander binary
// and tags it nodeImage, which compose.yml runs with pull_policy never.
func buildNodeImage(ctx context.Context) error {
	if _, err := os.Stat(wanderBinary); err != nil {
		return fmt.Errorf("no wander binary at %s, build it with `make integration-cluster-test`: %w", wanderBinary, err)
	}

	dir, err := os.MkdirTemp("", "wander-it-node-")
	if err != nil {
		return fmt.Errorf("build context: %w", err)
	}
	defer os.RemoveAll(dir)

	files := map[string]string{
		"node/Dockerfile":    "Dockerfile",
		"node/entrypoint.sh": "entrypoint.sh",
		"node/agentctl":      "agentctl",
		wanderBinary:         "wander",
	}
	for src, dst := range files {
		if err := copyExecutable(src, filepath.Join(dir, dst)); err != nil {
			return fmt.Errorf("build context: %w", err)
		}
	}

	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return fmt.Errorf("docker provider: %w", err)
	}
	defer provider.Close()

	consul := consulImage
	_, err = provider.BuildImage(ctx, &testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:   dir,
			Repo:      nodeRepo,
			Tag:       nodeTag,
			BuildArgs: map[string]*string{"CONSUL_IMAGE": &consul},
			KeepImage: true,
		},
	})
	if err != nil {
		return fmt.Errorf("build %s: %w", nodeImage, err)
	}
	return nil
}

// copyExecutable copies src to dst with mode 0755, which the Dockerfile's COPY
// keeps in the image.
func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// -------------------------------------------------------------------------
// API CLIENTS
// -------------------------------------------------------------------------

// serverClients builds an API client for each Wander server from the host
// port Docker published for 4646.
func serverClients(ctx context.Context, stack *compose.DockerCompose) (map[string]*api.Client, error) {
	clients := make(map[string]*api.Client, len(serverNames))
	for _, name := range serverNames {
		ctr, err := stack.ServiceContainer(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("find %s: %w", name, err)
		}
		addr, err := ctr.PortEndpoint(ctx, "4646/tcp", "http")
		if err != nil {
			return nil, fmt.Errorf("published port for %s: %w", name, err)
		}
		client, err := newClient(addr)
		if err != nil {
			return nil, fmt.Errorf("client for %s: %w", name, err)
		}
		clients[name] = client
	}
	return clients, nil
}

// newClient returns an API client for addr; run scrubClusterEnv first.
func newClient(addr string) (*api.Client, error) {
	return api.NewClient(&api.Config{Address: addr})
}

// -------------------------------------------------------------------------
// FAILURE LOGS
// -------------------------------------------------------------------------

// saveLogs writes each service's container log, and the log files the node
// entrypoint keeps inside the container, to results/<UTC time>/.
func saveLogs(ctx context.Context, stack *compose.DockerCompose) {
	dir := filepath.Join("results", time.Now().UTC().Format("20060102T150405Z"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "integration: results dir: %v\n", err)
		return
	}
	for _, svc := range stack.Services() {
		ctr, err := stack.ServiceContainer(ctx, svc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "integration: find %s: %v\n", svc, err)
			continue
		}
		if err := saveContainerLog(ctx, ctr, filepath.Join(dir, svc+".log")); err != nil {
			fmt.Fprintf(os.Stderr, "integration: log for %s: %v\n", svc, err)
		}
		for _, path := range entrypointLogs(svc) {
			dst := filepath.Join(dir, svc+"."+filepath.Base(path))
			if err := saveContainerFile(ctx, ctr, path, dst); err != nil {
				fmt.Fprintf(os.Stderr, "integration: %s from %s: %v\n", path, svc, err)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "integration: container logs saved to %s\n", dir)
}

// entrypointLogs returns the files the node entrypoint logs to inside svc's
// container: the Consul agent on every Wander node, and dockerd on clients.
func entrypointLogs(svc string) []string {
	switch {
	case strings.HasPrefix(svc, "wander-server-"):
		return []string{"/var/log/consul.log"}
	case strings.HasPrefix(svc, "wander-client-"):
		return []string{"/var/log/consul.log", "/var/log/dockerd.log"}
	}
	return nil
}

// saveContainerLog writes the container's stdout and stderr to dst.
func saveContainerLog(ctx context.Context, ctr testcontainers.Container, dst string) error {
	logs, err := ctr.Logs(ctx)
	if err != nil {
		return err
	}
	return writeFile(dst, logs)
}

// saveContainerFile copies the file at path inside the container to dst.
func saveContainerFile(ctx context.Context, ctr testcontainers.Container, path, dst string) error {
	file, err := ctr.CopyFileFromContainer(ctx, path)
	if err != nil {
		return err
	}
	return writeFile(dst, file)
}

// writeFile copies r to a new file at dst and closes r.
func writeFile(dst string, r io.ReadCloser) error {
	defer r.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

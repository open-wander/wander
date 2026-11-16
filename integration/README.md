# Wander integration cluster tests

Go tests that run against a real multi-node Wander cluster in Docker, for what
a single `agent -dev` cannot reach: clustering, discovery through Consul, and
replication across servers.

```
consul-server-1..3   10.77.0.11-13   Consul servers
wander-server-1..3   10.77.0.31-33   Wander servers + local Consul agent
wander-client-1..3   10.77.0.41-43   Wander clients + local Consul agent + own dockerd
```

Servers and clients are given no join addresses. They find each other through
Consul, so a cluster that forms at all has exercised the Consul integration.

The tests live in their own Go module, so Testcontainers and its dependencies
never reach the release binary.

## Running

```
make integration-cluster-test
```

Run it from the repository root. The target generates the templ templates,
builds `pkg/linux_amd64/wander` from this checkout with cgo (the exec driver
needs it), and runs `go test -tags integration` in this directory. `TestMain`
then builds the `wander-it-node:local` image around that binary, starts
`compose.yml` through Testcontainers, runs the tests and removes the stack,
including its volumes.

Requirements: a Linux host with Docker and cgroups v2 (the nodes run
privileged), Go, a C compiler, and `templ` (`make templ-deps`). Allow about
3 GB of RAM.

After the binary is built, `go test -tags integration -count=1 -v ./...` in
this directory runs the tests alone.

**`TestMain` clears every `NOMAD_*`, `WANDER_*`, `CONSUL_*` and `VAULT_*`
variable before it starts**, and the API clients are built from explicit
addresses, so a shell configured for a real cluster cannot reach it from here.

When a test fails, every container's log is saved under `results/`.

## What the tests cover

| test | what it shows |
|---|---|
| `TestCluster_Formed` | 3 raft peers, a leader, 3 voters, autopilot healthy with failure tolerance 1, and 3 members alive advertising `build=1.6.5` |
| `TestCluster_ClientsReady` | 3 clients ready and eligible, with docker, exec and raw_exec healthy and the CNI bridge plugin fingerprinted |
| `TestVariables_Replicated` | a variable written through one server is read back from each server's own state with stale reads |

Consul service registration, Vault, Connect and failure recovery come in later
changes.

## Design notes

- **The tests reach the servers on published ports**, not on the
  `10.77.0.0/24` addresses, so they do not depend on the host routing to the
  Compose network.
- **The node image is built from the checkout's binary.** The Dockerfile copies
  `wander` from the build context, and the Consul and Debian images are pinned
  by digest.
- **The agent is supervised, not PID 1**, so later failure tests can
  `agentctl stop|kill|start|restart` it without taking the container (and its
  Consul agent and dockerd) with it.
- **Clients run their own dockerd.** A shared host socket breaks alloc-dir bind
  mounts and bridge-mode netns paths, and each client's dangling-container
  reconciler kills the other clients' containers.
- **Consul node IDs come from the hostname.** A recreated node gets a fresh
  Consul agent; with a random ID, the Consul servers refuse it as a rename of
  the still-registered node and drop everything it registers.
- **cgroups v2 nesting:** the entrypoint moves every process into a leaf and
  enables all controllers on the container's cgroup root before anything else
  starts. If it cannot, the node refuses to boot rather than come up with the
  exec driver and docker silently broken.

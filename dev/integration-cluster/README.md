# Wander integration cluster

A local multi-node Wander cluster with Consul and Vault, for testing what a
single `agent -dev` cannot reach: clustering, failure and recovery, Consul
service registration and discovery, Vault secrets, and Connect.

```
consul-server-1..3   10.77.0.11-13   Consul servers, Connect enabled
vault                10.77.0.20      Vault dev server (root token: root)
wander-server-1..3   10.77.0.31-33   Wander servers + local Consul agent
wander-client-1..3   10.77.0.41-43   Wander clients + Consul agent + own dockerd
```

Servers and clients are given no join addresses: they find each other through
Consul, so a cluster that forms at all has exercised the Consul integration.

Requires a Linux host with Docker and cgroups v2 (containers run privileged;
the addresses are routed from the host, so nothing is published). Roughly 3 GB
of RAM.

## Usage

```
make test                    # build this checkout, start the cluster, run the suite (~5 min)
make test TESTS="vault_secret connect"
make up                      # build and start only
make down                    # remove everything, including volumes

make up WANDER_VERSION=0.1.1 # a published release instead of this checkout
eval "$(make -s env)"        # point your shell's CLIs at the test cluster
```

Each target builds what it needs. `make test` runs `make up` first, which
builds Wander from this checkout with the release workflow's recipe (CSS,
templ templates, then the binary in `pkg/linux_amd64/wander`) and wraps it as
the `wander-local` image. An unchanged checkout rebuilds in a few seconds; a
changed one recreates the Wander containers on the new binary.

`WANDER_VERSION` or `WANDER_IMAGE` picks a published release or any image
instead, and the choice holds until `make down`. `CONSUL_VERSION` and
`VAULT_VERSION` pick the HashiCorp images (default 2.1.0 and 2.2.2). Results
are written to `results/`.

The source build needs Go, a C compiler and npm. The first build runs
`make templ-deps` and `npm ci`; `npm ci` rewrites `ui/static/js/alpine.min.js`
from `node_modules`, so restore it with `git checkout` if you don't mean to
commit it.

**Every script and make target clears `NOMAD_*`, `WANDER_*`, `CONSUL_*` and
`VAULT_*` from the environment first**, so a shell configured for a real
cluster cannot reach it from here. `make -s env` prints `unset` lines for the
same reason.

## What the suite covers

| test | what it shows |
|---|---|
| cluster_formed | 3 voters, autopilot healthy, servers advertise `build=1.6.5`; no `retry_join` -- servers found each other through Consul |
| clients_ready | 3 clients found the servers through Consul; docker/exec/raw_exec, Connect, Vault and CNI fingerprinted |
| consul_registration | servers and clients self-register in Consul and pass their checks |
| variables | written on one server, read from all three |
| raw_exec_spread | 6 allocs spread across 3 clients |
| exec_driver | chroot + cgroups v2 isolation |
| docker_consul_service | docker tasks registered in Consul with interpolated tags/meta and passing HTTP checks, reachable on their mapped ports |
| alloc_exec | `alloc exec` / `alloc fs` / `alloc status` from a server's CLI into another node |
| native_service_discovery | `provider = "nomad"` registrations and a `nomadService` template across nodes |
| consul_template | Consul KV and catalog templates, re-rendered on change |
| vault_secret | token integration: KV v2 secret rendered, token policy/role/metadata/TTL, revoked when the alloc stops |
| connect | bridge networking, Envoy sidecars, an upstream, deny intention enforced and lifted |
| leader_failover | leader killed; new leader issues and revokes Vault tokens; old leader rejoins |
| client_failure | client killed; node down, allocs lost and replaced, stale Consul registrations cleaned up on return |
| drain | node drain migrates docker, raw_exec and Connect workloads |
| quorum_loss | 2 of 3 servers stopped; writes refused, stale reads served, state intact after recovery |
| full_server_restart | all servers down; workloads keep running; same allocs afterwards, none rescheduled |
| snapshot | `operator snapshot save` / `inspect` |

Not covered: ACLs, TLS, CSI volumes, multiple regions, and task drivers other
than docker, exec and raw_exec.

## Design notes

- **The agent is supervised, not PID 1**, so the failure tests can
  `agentctl stop|kill|start|restart` it without taking the container (and its
  Consul agent and dockerd) with it.
- **Clients run their own dockerd.** A shared host socket breaks alloc-dir bind
  mounts, bridge-mode netns paths, and each client's dangling-container
  reconciler kills the others' containers.
- **Images are preloaded** from `.cache/images.tar` (busybox, the pause image,
  the Envoy version Consul advertises) so runs don't depend on Docker Hub from
  inside nested daemons. Delete `.cache/` after changing `ENVOY_VERSION`.
- **cgroups v2 nesting:** the entrypoint moves every process into a leaf and
  enables all controllers on the container's cgroup root before anything else
  starts. If it cannot, the node refuses to boot rather than come up with the
  exec driver and docker silently broken.

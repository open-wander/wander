# Wander

[![License: MPL 2.0](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](LICENSE)
[![Discuss](https://img.shields.io/badge/discuss-wander-2563EB?style=flat)](https://github.com/open-wander/wander/discussions)

Wander is a simple and flexible workload orchestrator to deploy and manage containers (Docker and Podman), non-containerized applications (executables and Java), and virtual machines (QEMU) across on-prem and clouds at scale.

Wander is supported on Linux, Windows, and macOS.

> **Note for Nomad users:** Wander 0.1.0 is intended to be a drop-in
> replacement for the open-source HashiCorp Nomad 1.6.5 server and client
> binaries. The release process tests clean installation, rolling server
> replacement, state preservation, rollback, and workload continuity from that
> exact version. Back up cluster state before migrating. Clusters using newer
> Nomad versions, Enterprise-only features, or behavior outside the documented
> 1.6.5 compatibility boundary are not supported migration sources.

* Website: https://openwander.org
* Getting started and migration: [Wander 0.1.0 operator runbook](docs/operations/0.1.0-runbook.md)
* Forum: [GitHub Discussions](https://github.com/open-wander/wander/discussions)
* Downloads and native packages: [GitHub Releases](https://github.com/open-wander/wander/releases)
* Container images: [`ghcr.io/open-wander/wander`](https://github.com/open-wander/wander/pkgs/container/wander)

## Features

* **Deploy Containers and Legacy Applications**: Wander's flexibility as an orchestrator enables an organization to run containers, legacy, and batch applications together on the same infrastructure. Wander brings core orchestration benefits to legacy applications without needing to containerize via pluggable task drivers.

* **Simple & Reliable**: Wander runs as a single binary and is entirely self contained - combining resource management and scheduling into a single system. Wander does not require any external services for storage or coordination. Wander automatically handles application, node, and driver failures. Wander is distributed and resilient, using leader election and state replication to provide high availability in the event of failures.

* **Device Plugins & GPU Support**: Wander offers built-in support for GPU workloads such as machine learning (ML) and artificial intelligence (AI). Wander uses device plugins to automatically detect and utilize resources from hardware devices such as GPU, FPGAs, and TPUs.

* **Federation for Multi-Region, Multi-Cloud**: Wander was designed to support infrastructure at a global scale. Wander supports federation out-of-the-box and can deploy applications across multiple regions and clouds.

* **Proven Scalability**: Wander is optimistically concurrent, which increases throughput and reduces latency for workloads. Wander has been proven to scale to clusters of 10K+ nodes in real-world production environments.

* **Ecosystem Integration**: Wander integrates seamlessly with Terraform, Consul, and Vault for provisioning, service discovery, and secrets management.

## Quick Start

### Testing

See the [Wander 0.1.0 operator runbook](docs/operations/0.1.0-runbook.md) for installation, local validation, and Nomad 1.6.5 migration instructions.

Optionally, find Terraform manifests for bringing up a development Wander cluster on a public cloud in the [`terraform`](terraform/) directory.

### Production

Start with the [Wander 0.1.0 operator runbook](docs/operations/0.1.0-runbook.md). Test backup, migration, rollback, and workload continuity in a representative environment before a production deployment.

## Documentation

The 0.1.x operator documentation is maintained in this repository, starting
with the [Wander 0.1.0 operator runbook](docs/operations/0.1.0-runbook.md).
The clean replacement website is being built in [`site/`](site/README.md) and
is not yet published. The inherited `website/` tree remains reference-only
while its text undergoes licensing, provenance, and content review.

Operators preparing a 0.1.x installation or Nomad 1.6.5 migration should also
read the [Wander 0.1.0 operator runbook](docs/operations/0.1.0-runbook.md).

## Companion Tools

* **Ramble** - Package manager for Wander, providing templated job definitions. Works with both Wander and HashiCorp Nomad. See [github.com/open-wander/ramble](https://github.com/open-wander/ramble).

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request. Wander
does not accept code copied from the HashiCorp Nomad repository. Every commit
needs a `Signed-off-by` line.

See the [`contributing`](contributing/) directory for developer documentation.

## Security

Report suspected vulnerabilities privately as described in the
[Wander security policy](SECURITY.md). Do not disclose vulnerability details
in a public issue or discussion.

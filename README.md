# Wander

[![License: MPL 2.0](https://img.shields.io/badge/License-MPL_2.0-brightgreen.svg)](LICENSE)
[![Discuss](https://img.shields.io/badge/discuss-wander-2563EB?style=flat)](https://github.com/open-wander/wander/discussions)

Wander is a simple and flexible workload orchestrator to deploy and manage containers ([docker](https://openwander.org/wander/docs/drivers/docker), [podman](https://openwander.org/wander/docs/drivers/podman)), non-containerized applications ([executable](https://openwander.org/wander/docs/drivers/exec), [Java](https://openwander.org/wander/docs/drivers/java)), and virtual machines ([qemu](https://openwander.org/wander/docs/drivers/qemu)) across on-prem and clouds at scale.

Wander is supported on Linux, Windows, and macOS.

> **Note for Nomad users:** Wander is forked from an earlier version of HashiCorp Nomad. Direct migration from an existing Nomad installation is not supported. To use Wander, you'll need to deploy fresh clusters and redeploy your workloads.

* Website: https://openwander.org
* Tutorials: [Wander Tutorials](https://openwander.org/wander/tutorials)
* Forum: [GitHub Discussions](https://github.com/open-wander/wander/discussions)

## Features

* **Deploy Containers and Legacy Applications**: Wander's flexibility as an orchestrator enables an organization to run containers, legacy, and batch applications together on the same infrastructure. Wander brings core orchestration benefits to legacy applications without needing to containerize via pluggable task drivers.

* **Simple & Reliable**: Wander runs as a single binary and is entirely self contained - combining resource management and scheduling into a single system. Wander does not require any external services for storage or coordination. Wander automatically handles application, node, and driver failures. Wander is distributed and resilient, using leader election and state replication to provide high availability in the event of failures.

* **Device Plugins & GPU Support**: Wander offers built-in support for GPU workloads such as machine learning (ML) and artificial intelligence (AI). Wander uses device plugins to automatically detect and utilize resources from hardware devices such as GPU, FPGAs, and TPUs.

* **Federation for Multi-Region, Multi-Cloud**: Wander was designed to support infrastructure at a global scale. Wander supports federation out-of-the-box and can deploy applications across multiple regions and clouds.

* **Proven Scalability**: Wander is optimistically concurrent, which increases throughput and reduces latency for workloads. Wander has been proven to scale to clusters of 10K+ nodes in real-world production environments.

* **Ecosystem Integration**: Wander integrates seamlessly with Terraform, Consul, and Vault for provisioning, service discovery, and secrets management.

## Quick Start

### Testing

See [Getting Started](https://openwander.org/wander/tutorials/getting-started) for instructions on setting up a local Wander cluster for non-production use.

Optionally, find Terraform manifests for bringing up a development Wander cluster on a public cloud in the [`terraform`](terraform/) directory.

### Production

See [Wander Reference Architecture](https://openwander.org/wander/docs/operations/reference-architecture) for recommended practices and a reference architecture for production deployments.

## Documentation

Full, comprehensive documentation is available on the Wander website: https://openwander.org/wander/docs

## Companion Tools

* **Ramble** - Package manager for Wander, providing templated job definitions. Works with both Wander and HashiCorp Nomad. See [github.com/open-wander/ramble](https://github.com/open-wander/ramble).

## Contributing

See the [`contributing`](contributing/) directory for more developer documentation.

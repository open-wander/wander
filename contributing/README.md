Wander Codebase Documentation
===

This directory contains some documentation about the Wander codebase,
aimed at readers who are interested in making code contributions.

If you're looking for information on _using_ Wander, please instead refer
to the [Wander 0.1.0 operator runbook](../docs/operations/0.1.0-runbook.md).

Source rule
---
Wander does not accept code copied from the HashiCorp Nomad repository. Read
[CONTRIBUTING.md](../CONTRIBUTING.md) before opening a pull request. The
documents in this directory describe the Wander source tree. They are not a
guide to the Nomad repository, and links in them point to files in this
repository.

Developing with Vagrant
---
A development environment is supplied via Vagrant to make getting started easier.

1. Install [Vagrant](https://www.vagrantup.com/docs/installation)
1. Install [Virtualbox](https://www.virtualbox.org/)
1. Bring up the Vagrant project
    ```sh
    $ git clone https://github.com/open-wander/wander.git
    $ cd wander
    $ vagrant up
    ```

    The virtual machine will launch, and a provisioning script will install the
    needed dependencies within the VM.

1. SSH into the VM
    ```sh
    $ vagrant ssh
    ```

Developing with Lima (Apple Silicon Macs)
---
Vagrant with VirtualBox does not work on Apple Silicon (M1/M2/M3) Macs. Lima provides
a lightweight Linux VM alternative that works natively on ARM-based Macs.

1. Install [Lima](https://lima-vm.io/)
    ```sh
    $ brew install lima
    ```

1. Create and start the VM using the convenience script
    ```sh
    $ ./scripts/lima-start.sh up
    ```

    This will create an Ubuntu 22.04 VM with all dependencies pre-installed:
    - Go 1.21.5
    - Docker
    - Consul
    - Vault
    - CNI plugins

1. SSH into the VM
    ```sh
    $ ./scripts/lima-start.sh ssh
    ```

1. Your home directory is mounted at the same path, so access the project at its normal location

1. First time inside VM - bootstrap and run tests
    ```sh
    $ cd ~/path/to/wander   # same path as on your Mac
    $ make bootstrap
    $ make test
    ```

1. When done, stop the VM
    ```sh
    $ ./scripts/lima-start.sh stop
    ```

**Lima Commands:**
- `./scripts/lima-start.sh up` - Create and start VM
- `./scripts/lima-start.sh ssh` - SSH into VM
- `./scripts/lima-start.sh stop` - Stop VM
- `./scripts/lima-start.sh destroy` - Delete VM completely
- `./scripts/lima-start.sh status` - Show VM status

Developing without Vagrant
---
1. Install the [Go](https://go.dev/) version named in `.go-version` *(Note: `gcc-go` is not supported)*
1. Clone this repo
   ```sh
   $ git clone https://github.com/open-wander/wander.git
   $ cd wander
   ```
1. Bootstrap your environment
   ```sh
   $ make bootstrap
   ```
1. (Optionally) Set a higher ulimit, as Wander creates many file handles during normal operations
   ```sh
   $ [ "$(ulimit -n)" -lt 1024 ] && ulimit -n 1024
   ```
1. Verify you can run smoke tests
   ```sh
   $ make test
   ```
   **Note:** You can think of this as a `smoke` test which runs a subset of
   tests and some may fail because of `operation not permitted` error which
   requires `root` access. You can use `go test` to test the specific subsystem
   which you are working on and let the CI run rest of the tests for you.

Running a development build
---
1. Compile a development binary
    ```sh
    $ make dev
    # find the built binary at ./bin/wander
    ```
1. Start the agent in dev mode
    ```sh
    $ sudo bin/wander agent -dev
    ```
1. (Optionally) Run Consul to enable service discovery and health checks
    1. Download [Consul](https://www.consul.io/downloads)
    1. Start Consul in dev mode
        ```sh
        $ consul agent -dev
        ```

Compiling Protobufs
---
If in the course of your development you change a Protobuf file (those ending in .proto), you'll need to recompile the protos.

1. Run `make boostrap` to install the [`buf`](https://github.com/bufbuild/buf)
   command.
1. Compile Protobufs
    ```sh
    $ make proto
    ```

Building the Web UI
---
The UI is built using HTMX, Templ, and Tailwind CSS. See the `ui/` directory for details.

To run the UI in development mode:
```sh
cd ui && make dev
```

Create a release binary
---
To create a release binary:

```sh
$ make prerelease
$ make release
$ ls ./pkg
```

This will generate all the static assets, compile Wander for multiple
platforms and place the resulting binaries into the `./pkg` directory.

API Compatibility
--------------------
Only the `api/` and `plugins/` packages are intended to be imported by other projects. The root Wander module does not follow semver and is not intended to be imported directly by other projects.

## Architecture

The code for Wander's major components is organized as follows. Directory,
package and type names inherited from the 1.6.5 baseline keep the `nomad` name
(see [the compatibility boundary](../docs/compatibility-boundary.md)):

* `api/` provides a Go client for Wander's HTTP API.
* `client/` contains Wander's client agent code.
* `command/` contains Wander's CLI code.
* `nomad/` contains Wander's server agent code.
* `ui/` contains Wander's UI code (HTMX, Templ, Tailwind CSS).
* `site/` contains the Wander website.
* `website/` contains the inherited Nomad website. It is reference-only and is not published.

The high level control flow for many Wander actions (via the CLI or UI) are:

```
# Read actions:
Client -> HTTP API -> RPC -> StateStore

# Actions that change state:
Client -> HTTP API -> RPC -> Raft -> FSM -> StateStore
```

Checklists
---

When adding new features to Wander there are often many places to make changes.
It is difficult to determine where changes must be made and easy to make
mistakes.

The following checklists are meant to be copied and pasted into PRs to give
developers and reviewers confidence that the proper changes have been made:

* [New `jobspec` entry](checklist-jobspec.md)
* [New CLI command](checklist-command.md)
* [New RPC endpoint](checklist-rpc-endpoint.md)

Tooling
---

* [Go tool versions](golang.md)

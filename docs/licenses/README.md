# Wander 0.1.0 license and provenance inventory

Audit date: 2026-10-23

This is an engineering inventory, not legal advice. It records the evidence
available in this repository and identifies decisions that still require the
project owner or qualified legal review.

## Release scope

The release workflow builds one Wander executable with the `ui` build tag for:

- Linux: `amd64`, `arm`, and `arm64`;
- macOS: `amd64` and `arm64`; and
- Windows: `amd64`.

The executable embeds everything under `ui/static`. The Docker images reuse the
Linux `amd64` or `arm64` executable. Website, screenshot, and browser-test
dependencies are repository tooling and are not part of the release binary.

## Status

**The repository is not ready for a 0.1.0 release on licensing evidence yet.**

| Area | Result | Release effect |
| --- | --- | --- |
| Project source | Root license is MPL-2.0; 2,249 tracked files carry an explicit MPL-2.0 SPDX marker. | Candidate baseline still needs project-owner approval. |
| Historical boundary | `v1.6.5` is the exact ancestor at `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f`. It contains 45 BUSL-1.1 files: 37 in the former UI and 8 elsewhere. All were removed by explicit reverts before the replacement UI began. | Exact excluded paths are recorded in `baseline-busl-exclusions.txt`. |
| Current tree | No current implementation file contains a BUSL marker. | Marker check passes, but marker absence alone is not provenance proof. |
| Go release dependencies | Representative scans cover the identical module closures for all release architectures: 296 third-party modules, 307 scanner records, and 2 manually classified omissions across Linux, macOS, and Windows. The requested BUSL-1.1 Vault Kubernetes auth module is replaced by its source-identical MPL-2.0 predecessor. | The CI boundary check pins the resolved replacement; the incomplete go-spin notice remains unresolved. |
| Browser runtime assets | Alpine CSP 3.15.4, htmx 2.0.8, xterm 5.5.0, and xterm addon-fit 0.10.0 have exact version/hash evidence. | Notices are now retained in this directory, but the release must still define how recipients receive them. |
| UI build dependencies | Ten locked packages, all declaring MIT. | They build embedded Alpine/Tailwind output; see `npm-release-packages.csv`. |
| Test-only UI dependencies | Seven locked packages: four Apache-2.0 and three MIT. | Not shipped; recorded below for repository hygiene. |
| Website and legacy tooling | Separate and not shipped in the executable. Several inherited lockfiles are too old to carry reliable license metadata. | Audit separately before publishing those artifacts; not a Wander binary blocker. |

## Exact candidate source boundary

The candidate inherited source set is defined reproducibly as all 5,299 paths
and Git blobs in commit `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f`
(`v1.6.5`) **minus** the 45 paths in `baseline-busl-exclusions.txt`. This leaves
5,254 baseline paths as candidates, not automatically approved files. The
commit object fixes the exact content and the exclusion file fixes the exact
mixed-license removals without duplicating thousands of Git blob identifiers
in documentation.

The candidate set becomes an allowed source manifest only after project-owner
review. Files without an explicit marker, generated files, later modifications,
and new Wander files still require the non-marker provenance process.

## Remediated blocker: compiled BUSL dependency

The release package closure imports a module requested as:

`github.com/hashicorp/vault/api/auth/kubernetes@v0.5.0`

Its distributed `LICENSE` identifies Vault 1.15.0 under the Business Source
License 1.1. Wander now resolves that request to the source-identical
MPL-2.0 `v0.4.1` release. The import path is:

`client/allocrunner/taskrunner/template` -> `consul-template/dependency` ->
`vault/api/auth/kubernetes`.

The repository-only marker check could not see licenses in the Go module cache,
which is why this was previously missed. The CI boundary check now verifies
that the effective version is `v0.4.1`, preventing an upgrade back across the
BUSL boundary. The replacement does not enable the optional authentication
path: Wander continues to supply task-scoped Vault tokens to consul-template.

## Source header coverage

The current tracked tree has 4,284 files. Explicit MPL-2.0 SPDX coverage for
the main implementation-oriented extensions is:

| Extension | Tracked | Explicit MPL-2.0 SPDX |
| --- | ---: | ---: |
| Go | 1,857 | 1,763 |
| JavaScript | 29 | 10 |
| TypeScript | 15 | 0 |
| Templ | 119 | 0 |
| CSS | 2 | 0 |
| shell | 57 | 49 |
| HCL | 201 | 190 |
| protobuf | 10 | 10 |

The root MPL-2.0 license may cover project files without per-file markers, but
that is not evidence of who authored them or what inputs were used. In
particular, the replacement Templ UI and its generated Go files need the
project-owner non-marker provenance review described in `release-review.md`.

## Manual review records

Automated classifiers need the following corrections or qualifications:

- `kernel.org/pub/linux/libs/security/libcap/psx@v1.2.66` was reported as
  GPL-2.0. Its `License` expressly offers a choice of BSD-3-Clause or GPL-2.0;
  Wander selects BSD-3-Clause. The complete notice must be retained.
- `github.com/hashicorp/vic@v1.5.1-0.20190403131502-bbfe86ec9443` was reported
  as AFL-2.0 because its aggregate VMware notice contains AFL terms for an
  unrelated operating-system component. The imported `pkg/vsphere/tags` files
  carry Apache-2.0 headers and the module's top-level work is Apache-2.0.
- `github.com/tj/go-spin@v1.1.0` declares MIT in its tagged README but the
  module archive omits a full license notice. Replace it or obtain and retain a
  defensible notice before release.
- The Google `go-licenses` scan omitted Vault Kubernetes auth and go-spin. The
  permanent CSV adds both as explicitly marked manual records and must be read
  with this document. Vault Kubernetes auth is manually classified from the
  effective `v0.4.1` replacement's MPL-2.0 license.

## Evidence files

- `go-release-modules.csv`: exact module versions, replacements, and release
  target membership for the compiled dependency union.
- `go-release-packages.csv`: package-level scanner output, license source URL,
  detected SPDX identifier, manual omissions, and release OS scope.
- `npm-release-packages.csv`: locked packages used to build the embedded UI.
- `browser-assets.md`: exact browser asset provenance and hashes.
- `baseline-busl-exclusions.txt`: all 45 BUSL-marked paths in the exact
  `v1.6.5` behavioral baseline.
- `third-party/`: exact retained license texts for vendored browser assets.
- `../plans/license-provenance-audit.md`: Git-history analysis for the source
  boundary and replacement UI.

## Reproduction

The Go package scan used pinned `github.com/google/go-licenses@v1.6.0` against
the root package with `ui` enabled. Linux `amd64`, Linux `arm`, and Linux
`arm64` have the same 292-module third-party closure; both macOS architectures
have the same 273-module closure; Windows `amd64` has 276. Their union is 296.
The CSV is the union of representative Linux, macOS, and Windows package scans.

The complete `go list -m all` graph contains 747 modules, but that graph
includes modules not compiled into the release. It is useful for supply-chain
maintenance and is not mislabeled here as shipped code.

## Required next decisions

1. Resolve the incomplete go-spin notice, preferably by removing or replacing
   that small transitive dependency.
2. Decide how the executable/archive presents all required third-party notices
   to recipients; files only in the source repository are not a release
   distribution process.
3. Have the project owner approve the exact source baseline, the 45 exclusions,
   and the independently developed UI provenance record.
4. Repeat the inventory against the final release commit and attach the output
   to the 0.1.0 release record/SBOM.

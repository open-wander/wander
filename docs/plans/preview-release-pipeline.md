# Preview Release Pipeline

## Goal

Publish reproducible Wander prerelease binaries early enough for external UI
and compatibility testing without presenting them as the stable 0.1.0 release.

## Release contract

- Preview versions use semantic prerelease identifiers such as
  `0.1.0-preview.1`.
- The existing multi-platform build remains the single source of archives:
  Linux amd64/arm/arm64, macOS amd64/arm64, and Windows amd64.
- Release archives contain one Wander binary, the embedded UI, the project
  license, and third-party notices.
- Release builds consume the reviewed, version-controlled codec sources. The
  packaging workflow does not rerun compatibility-era generators or delete
  their tracked outputs after publication.
- Linux amd64 must pass the packaged UI smoke test and the clean-install,
  rolling-restart, 1.6.5 upgrade, and 1.6.5 rollback rehearsal.
- GitHub release assets include all archives and native packages, an SPDX JSON
  SBOM for the canonical Linux amd64 archive, and one SHA-256 checksum
  manifest covering every published asset.
- GitHub artifact attestations provide signed build provenance for every
  published asset and bind the Linux amd64 archive to its SPDX SBOM.
- Versions containing a prerelease identifier are marked as GitHub
  prereleases. Stable versions are not.
- A preview release clearly states that final 0.1.0 parity work is still in
  progress and links to the revival checklist.
- Publishing is opt-in through the manually dispatched release workflow. A
  normal branch build never creates a tag or release.
- Preview releases publish the current `UNRELEASED` changelog without closing
  it. The stable release converts that section to the dated version and the
  post-release commit opens a fresh `UNRELEASED` section.

## Removed assumptions

- No HashiCorp signing key, S3 bucket, `hc-releases`, Slack channel, or elevated
  organization token is part of the Wander GitHub release path.
- Artifacts use `wander_` names. `scripts/dist.sh` is now an offline packaging
  helper only: it creates versioned local artifacts and checksums but cannot
  tag, sign, upload, or publish a release.
- Workflow dispatch uses the repository-scoped GitHub token. Local GitHub
  access continues to use Quartermaster and `gh-qm`.

## First preview procedure

1. Commit and push a reviewed checkpoint.
2. Dispatch `Release` from that branch with version `0.1.0-preview.1`.
3. Confirm the generated release commit and build reference.
4. Require every build, archive verification, UI smoke, lifecycle rehearsal,
   and compatibility job to pass.
5. Verify the GitHub prerelease assets and checksum manifest from a clean
   download before inviting testers.

## Stable publication channels

- Archives and native `.deb`/`.rpm` packages are published through
  [GitHub Releases](https://github.com/open-wander/wander/releases).
- Container images are published as `ghcr.io/open-wander/wander`.
- GitHub's keyless Sigstore attestations are the 0.1.0 signing identity.
- GitHub Issues and Discussions are the public support channels.
- GitHub private vulnerability reporting is the confidential security channel.
- Dedicated APT/YUM repositories and Apple Developer ID signing/notarization
  are distribution improvements planned after 0.1.0.

Preview releases provide the same checksum, SBOM, provenance, and attestation
path intended for stable 0.1.0. Preview `v0.1.0-preview.2` verified the
attestation identity, exact asset digests, and release-commit binding.

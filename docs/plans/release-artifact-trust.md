# Release Artifact Trust

## Release Contract

The publish job binds every downloadable Wander release artifact to the exact
workflow and build commit:

1. All platform archives and native packages are downloaded into `dist`.
2. Anchore Syft 1.56.0 generates an SPDX JSON SBOM by scanning the canonical
   Linux amd64 release archive, including its Go binary and embedded assets.
3. The SBOM is included in the SHA-256 manifest and published as a release
   asset.
4. GitHub's `actions/attest` creates signed SLSA build provenance for every
   subject in the checksum manifest.
5. A second signed attestation binds the Linux amd64 archive to its SPDX SBOM.

The action implementations are commit-pinned. GitHub uses a short-lived OIDC
identity and Sigstore certificate for the signatures; no long-lived Wander
signing key is stored in repository secrets.

## Consumer Verification

After downloading an asset, a consumer can check its bytes against the
published manifest and verify its signed provenance:

```sh
sha256sum --check wander_VERSION_SHA256SUMS
gh attestation verify wander_VERSION_linux_amd64.zip \
  --repo open-wander/wander
```

The GitHub attestation record is the approved signing equivalent for preview
releases. A detached-key signing scheme can be added later without replacing
the checksum, SBOM, or provenance contract.

The same GitHub/Sigstore identity is approved for 0.1.0. macOS archives are
not yet signed with an Apple Developer ID or submitted for notarization, so
operators may encounter the platform's unsigned-download warning. Developer
ID signing and notarization can be added after the project can enroll its
registered company in the Apple Developer Program; this will complement, not
replace, the cross-platform attestations.

## Release Gate

Preview `v0.1.0-preview.2`, produced by release workflow run `42356326274` on
November 7, 2026, proves the release-artifact trust contract:

- the SPDX file is a downloadable asset and is listed in the checksum manifest;
- all 14 checksum subjects verify after downloading the published assets;
- both provenance and SBOM attestations verify for the Linux amd64 archive;
- attestation subjects match the published asset digests; and
- the release job retains only the permissions documented by the workflow.

The first preview exposed GitHub's normalization of `~` in native-package asset
names. Publication now normalizes those names before generating checksums,
SBOMs, attestations, and release assets. The second preview independently
verified that every published archive and native package matches the manifest
using its downloadable filename.

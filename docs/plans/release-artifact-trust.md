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

## Release Gate

The implementation is present, but the 0.1.0 checklist remains open until a
preview release proves that:

- the SPDX file is a downloadable asset and is listed in the checksum manifest;
- both provenance and SBOM attestations verify for at least the Linux amd64
  archive;
- attestation subjects match the published asset digests; and
- the release job retains only the permissions documented by the workflow.

# Release license and provenance review

Run this review against the exact commit that will become a release. A marker
scan is one input; it is not approval of source ownership or provenance.

## Evidence refresh

1. Record the release commit, tag, build tags, Go version, Node version, and
   lockfile hashes.
2. Confirm `v1.6.5` resolves to
   `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f` and remains an ancestor.
3. Run the repository license-boundary check.
4. Generate package closures for every release OS/architecture and scan the
   compiled Go dependencies. Treat missing/unrecognized licenses as failures,
   not omissions.
5. Recompute hashes for every vendored browser asset and compare them with
   `browser-assets.md` and its immutable upstream version.
6. Re-read every new or changed dependency license; do not infer a license
   from its organization, an older version, or the root repository.
7. Confirm the release distribution includes the project license and all
   required third-party notices/attributions.
8. Attach the final inventory, SBOM, checksums, and review results to the
   release record.

## Non-marker provenance review

For files added or materially rewritten since the last approved release:

1. Review the author, commit message, issue/PR context, generator inputs, and
   any declared external reference.
2. Confirm implementation inputs comply with
   `docs/plans/license-safe-parity-method.md`.
3. Do not use later Nomad source, patches, tests, generated assets, or
   implementation details to fill evidence gaps.
4. If provenance cannot be established, remove and independently rewrite the
   affected implementation or explicitly omit the feature.
5. Record the decision, reviewer, date, and evidence. Silence is not approval.

## Sign-off record

- Release commit: `171040d011081e9a76d274bca0356ccf748acbff`
- Engineering reviewer/date: Codex engineering review, 2026-11-07
- Project-owner provenance approval/date: 2026-11-07. The project owner
  explicitly approved preview commit
  `0c8147faf18e47ed5cc67e9f3a62f1ade0d0ba03`, the Nomad 1.6.5 source
  baseline, the 45-file BUSL exclusion manifest, and the independently
  developed Wander UI provenance for 0.1.0. The stable commit descends from
  that commit and changes only `CHANGELOG.md` and `version/version.go`.
- Legal review, if requested/date: not requested; this remains an engineering
  inventory and is not legal advice.
- Exceptions and remediation references: the manually classified dependency
  records and retained notices in this directory; no release exception was
  required.

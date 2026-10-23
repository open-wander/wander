# Wander License Provenance Audit

## Scope

This audit records repository evidence about the `v1.6.5` source baseline, the
removal of BUSL-marked files, and the later HTMX/Templ rewrite. It is an
engineering provenance record, not legal advice or a substitute for
project-owner review. The release dependency inventory is maintained in
[`docs/licenses`](../licenses/README.md).

## Findings

1. The local `v1.6.5` tag resolves to
   `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f`, which is the exact merge-base
   and an ancestor of the current branch. Its root `LICENSE` is MPL-2.0, but
   the tag is mixed-license at file level.
2. That tag contains 45 files whose SPDX header is `BUSL-1.1`: 37 under `ui/`
   and 8 test/helper files elsewhere. The exact file-level exclusion manifest
   is [`baseline-busl-exclusions.txt`](../licenses/baseline-busl-exclusions.txt).
3. Nine commits on 2024-11-14 explicitly reverted the changes that introduced
   those files. Five removed the eight non-UI files:
   - `56492992db` reverted a CSI manager test refactor;
   - `dff3166eb1` reverted a CSI NodeExpandVolume backport;
   - `a8c1441c45` reverted a rescheduling E2E modernization;
   - `eda97f92d8` reverted exponential-backoff changes; and
   - `f1d629ccea` reverted a state iterator test backport.

   Four then removed the 37 UI files:
   - `073c24c5a6` reverted the access-control name-field backport;
   - `9c7ac8ca60` reverted ACL roles and role/policy/token management;
   - `4463bc5b42` reverted the BUSL-marked job-editor review change; and
   - `74646778be` reverted the BUSL-marked JSON-to-HCL change.
4. Immediately before that cleanup sequence, the repository contained all 45
   BUSL-marked files. At `74646778be`, the count is zero.
5. The new UI first appears in the separate `ui-htmx` tree at `6492604e5e` on
   2026-01-25. The BUSL cleanup commit is an ancestor of this commit.
6. The Ember UI was removed at `d80490dca5` on 2026-01-26, after the new UI had
   begun. The new tree was renamed from `ui-htmx` to `ui` at `121a97c678` on
   2026-01-27.
7. None of the 45 BUSL-marked paths exists in the current working tree.
8. Git content-similarity detection at a 20 percent threshold reports no file
   rename or copy from the `v1.6.5` UI into the initial renamed rewrite.
9. The current implementation source contains no BUSL SPDX headers or Business
   Source License headers. Documentation mentions those identifiers only to
   record and enforce this boundary.
10. The compiled dependency graph is a separate boundary. It currently contains
    `github.com/hashicorp/vault/api/auth/kubernetes@v0.5.0`, whose own license
    is BUSL-1.1. This is a release blocker even though the repository source
    marker check passes.

## Engineering conclusion

The repository history shows that all known BUSL-marked baseline files were
removed before the current UI was created, and there is no path continuity or
Git-detectable file copy into the rewrite. No current repository source file
needs deletion solely because it retains one of those files or headers.

That conclusion does not clear the release: the compiled BUSL dependency must
be removed or replaced, and the candidate source baseline still needs owner
approval.

This does not prove legal ownership of every line in a large rewrite. Before
release, the project owner must still confirm the allowed source baseline and
review the rewrite's provenance. If that review finds unapproved derivation,
the affected implementation must be removed and independently rewritten or
documented as an intentional omission.

## Rules for remaining parity work

- Do not open or copy any of the 45 BUSL-marked implementation files to build a
  feature.
- Treat the approved runtime behavior and API contract—not mixed-license
  source—as the compatibility target.
- Use only project-approved behavioral evidence, public documentation, and
  independently designed tests.
- Record the evidence and implementation decision for each restored feature.
- Run `ci/check-license-boundary.sh` in CI and before release. Passing the
  marker check is necessary but does not replace provenance review.

## Remaining actions

- [ ] Project owner confirms the exact candidate source baseline, the 45-path
      exclusion manifest, and clean-room rules.
- [ ] Project owner reviews the initial HTMX/Templ commits and confirms their
      non-Git provenance.
- [x] Generate the release source/dependency engineering inventory.
- [x] Review and hash third-party browser assets and UI build dependencies.
- [ ] Remove or replace the compiled BUSL Vault Kubernetes auth dependency.
- [ ] Resolve the incomplete go-spin license notice.
- [ ] Define how required license texts/notices accompany release recipients.
- [ ] Approve this audit as part of the 0.1.0 release record.

# Wander UI License Provenance Audit

## Scope

This audit records repository evidence about the old Ember UI, the removal of
BUSL-marked files, and the later HTMX/Templ rewrite. It is an engineering
provenance record, not legal advice or a substitute for project-owner review.

## Findings

1. The local `v1.6.5` tag contains 37 files under `ui/` whose SPDX header is
   `BUSL-1.1`. They primarily implement ACL roles/tokens and access-control UI,
   with additional job-editor conversion/review files and related tests.
2. Four commits on 2024-11-14 explicitly reverted the changes that introduced
   those files:
   - `073c24c5a6` reverted the access-control name-field backport;
   - `9c7ac8ca60` reverted ACL roles and role/policy/token management;
   - `4463bc5b42` reverted the BUSL-marked job-editor review change; and
   - `74646778be` reverted the BUSL-marked JSON-to-HCL change.
3. Immediately before that cleanup sequence, the UI contained 37 BUSL-marked
   files. At `74646778be`, the count is zero.
4. The new UI first appears in the separate `ui-htmx` tree at `6492604e5e` on
   2026-01-25. The BUSL cleanup commit is an ancestor of this commit.
5. The Ember UI was removed at `d80490dca5` on 2026-01-26, after the new UI had
   begun. The new tree was renamed from `ui-htmx` to `ui` at `121a97c678` on
   2026-01-27.
6. None of the 37 BUSL-marked paths exists in the current working tree.
7. Git content-similarity detection at a 20 percent threshold reports no file
   rename or copy from the `v1.6.5` UI into the initial renamed rewrite.
8. The current implementation source contains no BUSL SPDX headers or Business
   Source License headers. Documentation mentions those identifiers only to
   record and enforce this boundary.

## Engineering conclusion

The repository history shows that the known BUSL-marked UI files were removed
before the current UI was created, and there is no path continuity or Git-
detectable file copy into the rewrite. No current source file needs deletion
solely because it retains one of those files or headers.

This does not prove legal ownership of every line in a large rewrite. Before
release, the project owner must still confirm the allowed source baseline and
review the rewrite's provenance. If that review finds unapproved derivation,
the affected implementation must be removed and independently rewritten or
documented as an intentional omission.

## Rules for remaining parity work

- Do not open or copy BUSL-marked implementation source to build a feature.
- Treat the approved runtime behavior and API contract—not mixed-license
  source—as the compatibility target.
- Use only project-approved behavioral evidence, public documentation, and
  independently designed tests.
- Record the evidence and implementation decision for each restored feature.
- Run `ci/check-license-boundary.sh` in CI and before release. Passing the
  marker check is necessary but does not replace provenance review.

## Remaining actions

- [ ] Project owner confirms the exact source baseline and clean-room rules.
- [ ] Review the initial HTMX/Templ commits and their external generation inputs.
- [ ] Generate a complete source and dependency license inventory.
- [ ] Review third-party browser assets and build dependencies separately.
- [ ] Approve this audit as part of the 0.1.0 release record.

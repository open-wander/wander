# Norman Progress

Append-only execution log for crash recovery and reusable project patterns.

## 2026-11-08 — Dynamic host volumes imported

- Imported the approved clean-room dynamic-host-volumes PRD into
  `prds/backlog/prd-dynamic-host-volumes.md`.
- Created 31 dependency-ordered tasks across six phases in `prds/TASKS.md`.
- Recorded project commands, model strategy, verification rigor, and mandatory
  clean-room/privacy constraints in `prds/config.md`.
- Left the unrelated update, security advisory, and usage reporting PRD in
  `prds/research/`.
- Next task: 1.1, establish the public compatibility matrix and provenance
  record.

PATTERN: provenance - Post-1.6.5 public documentation and independently
observed black-box behavior may define interfaces, but never implementation.

PATTERN: privacy - Private migration evidence stays local and is translated
into synthetic acceptance cases before anything is committed or published.

PATTERN: migration - Existing directories are adopted through explicit
registration into new Wander Raft records; registration does not move, copy,
or mutate directory contents.

## 2026-11-08 — Task 1.1 complete

- Added `docs/compatibility/dynamic-host-volumes.json`, a 200-entry interface
  ledger spanning all 11 required product surfaces. Every runtime test remains
  explicitly planned and unresolved behavior remains null rather than inferred.
- Added `docs/compatibility/dynamic-host-volumes-provenance.md` with permitted
  evidence, prohibited inputs, mutable-documentation limitations, public
  contradictions, and the extension workflow.
- Added an offline validator and five unit tests with 42 mutation cases. The
  required red test first failed because the matrix was absent; later red tests
  exposed invalid PRD references and insufficient observation provenance.
- Final evidence: five contract tests passed; direct validation passed; the
  licence-boundary check passed; Deslop reported no new duplication.
- Advisor plan verdict: PROCEED. Advisor code review: PASS after both minor
  hardening findings were fixed and re-reviewed.
- The shared PRD remains in `prds/active/` because later project tasks still
  depend on it.
- Next ready tasks: 1.2 and 1.3.

PATTERN: provenance - Record operations separately from inputs, outputs,
errors, authorization, concurrency, and lifecycle; unsupported facets remain
explicitly unresolved.

PATTERN: provenance - Validate requirement references against their actual
definitions and require structured observation records rather than accepting
arbitrary anchors.

PATTERN: tooling - `git diff HEAD` excludes untracked additions, so supplement
the required scanner with no-index input until new files are tracked.

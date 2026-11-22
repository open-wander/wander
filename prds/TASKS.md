# Norman Tasks

Task status lives only in this file and `TASKS-archive.md`. All implementation
must follow the clean-room and private-data rules in the linked PRD and
`docs/plans/license-safe-parity-method.md`.

## Phase 1: Clean-room foundation — 6 tasks completed 2026-11-15 (see [TASKS-archive.md](TASKS-archive.md))

## Phase 2: Client and plugins — 6 tasks completed 2026-11-19 (see [TASKS-archive.md](TASKS-archive.md))

## Phase 3: Scheduling and claims

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 3.1 | Add the complete host-volume ACL capability model | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-15) | Added the five host-volume namespace capabilities with the documented implications and read/write shorthands, and a table-driven allow/deny matrix |
| 3.2 | Implement node selection for volume creation | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-20) | Added a pure server-side selection over a state snapshot: explicit node and pool rules, readiness, plugin health, constraints, per-node name uniqueness, a documented tie-break and a bounded diagnostic |
| 3.3 | Add scheduler feasibility checks for capabilities and access modes | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-21) | Host volume requests may carry the documented attachment and access modes; the scheduler accepts a dynamic volume only when its record is in the job namespace, on the candidate node, ready and has an exact capability match |
| 3.4 | Add atomic writer and attachment accounting | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-21) | Added the two missing access modes and exclusive-writer accounting for single-node-single-writer, derived from allocations and checked in the scheduler and the plan applier; rules taken from a recorded observation |
| 3.5 | Mount selected dynamic paths and release attachments correctly | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-21) | The task volume hook reads the node's current host volumes, waits for the first restore after an agent restart, and offers a dynamic volume only to its own namespace; the release point of a single-writer volume is pinned with tests |
| 3.6 | Implement sticky task-group claims | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-21) | Added the sticky field, the claim placement rule in the scheduler and the plan applier, and claim creation in the same Raft apply as the allocation; rules taken from a recorded two-node observation. Claim removal is in 3.7 |
| 3.7 | Add claim and volume garbage collection and unavailable-node handling | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | DONE (2026-11-22) | Claims are removed with their job (purge and garbage collection) and with their volume; a volume delete is refused while an allocation uses it; tests pin that a down or collected node keeps its records and claims, from a recorded two-node observation |

## Phase 4: Public APIs

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 4.1 | Implement HTTP list and read operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 1.6, 3.1 |
| 4.2 | Implement HTTP create, register, and update operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 2.6, 3.2, 3.3, 4.1 |
| 4.3 | Implement HTTP delete, deregister, and claim operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.6, 3.7, 4.2 |
| 4.4 | Add the complete typed Go API client | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.1, 4.2, 4.3 |

## Phase 5: Operator interfaces

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 5.1 | Implement CLI parsing, initialization, create, and register workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.4 |
| 5.2 | Implement CLI list, status, delete, deregister, and claim workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.4, 5.1 |
| 5.3 | Add UI dynamic-volume list and detail pages | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.1, 4.4 |
| 5.4 | Add UI create, register, update, delete, deregister, and claim workflows | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.2, 4.3, 5.3 |

## Phase 6: Hardening and release

| # | Task | PRD | Status | Notes |
|---|------|-----|--------|-------|
| 6.1 | Harden concurrency, failover, idempotency, and destructive operations | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 3.7, 4.3 |
| 6.2 | Add bounded metrics, diagnostics, and audit logging | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 4.3, 6.1 |
| 6.3 | Run synthetic end-to-end, namespace, ACL, restart, failover, upgrade, amd64, and arm64 verification | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: all implementation tasks |
| 6.4 | Complete provenance review, migration runbook, compatibility documentation, and release gate | [prd-dynamic-host-volumes.md](active/prd-dynamic-host-volumes.md) | TODO | Needs: 6.3 |

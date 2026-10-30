# Gosec release-notice exclusion

## Context

Release builds copy third-party Go source into
`pkg/*/.release-notices/licenses/` for license compliance. Those generated,
ignored artifacts are not Wander source, but a repository-wide `gosec ./...`
scan discovers them and reports both dependency findings and type-check errors.

## Research findings

- `pkg/` is ignored build output populated by `GNUmakefile` release targets.
- Gosec supports excluding a directory name everywhere with
  `-exclude-dir=.release-notices`.
- A high-severity verification scan with that option reported no paths below
  `.release-notices`, while retaining findings in Wander source.
- `.loop/config.md` is the repository's existing source for local validation
  commands.

## Technical approach

Record the repository-wide gosec command in `.loop/config.md` with the exact
directory-name exclusion. Do not exclude all of `pkg/`, and do not suppress
any gosec rules or findings in Wander source.

## Acceptance criteria

- [x] The documented security command contains
      `-exclude-dir=.release-notices`.
- [x] A verification scan reports no `.release-notices` paths.
- [x] Genuine Wander-source findings remain visible.

## Risks and mitigations

- A broader exclusion could hide product code. Excluding only the generated
  directory name keeps all other scanned paths and rules active.

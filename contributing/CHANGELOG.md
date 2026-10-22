# Wander changelog process

Wander uses [`go-changelog`](https://github.com/hashicorp/go-changelog) to
assemble release notes from small files committed with each change. The
existing `CHANGELOG*.md` files and `.changelog` entries through Nomad v1.6.5
are retained as the inherited project history. New entries describe only work
implemented in this independent Wander fork.

## Adding an entry

Every pull request with a user-visible change must include
`.changelog/<PR-number>.txt`. Documentation-only, test-only, and internal
refactors may omit an entry when they have no operator-visible effect.

Run `make cl` to create an entry, then use one or more of these categories:

- `security`: vulnerability fixes or security hardening;
- `breaking-change`: an intentional compatibility break;
- `feature`: significant new operator-facing capability;
- `improvement`: an incremental behavior or usability enhancement;
- `bug`: a defect fix;
- `deprecation`: behavior scheduled for removal;
- `note`: important release or migration information.

Entries use this format (without the indentation):

    ```release-note:bug
    ui: prevent allocation detail pages from failing when task data is incomplete
    ```

Write from the operator's perspective, name the affected component, and avoid
claims that are not covered by a release test. A change with multiple distinct
effects may contain multiple release-note blocks.

## Building a release changelog

Install the project dependencies with `make deps`, commit all entry files, then
run:

```shell
make changelog
```

`LAST_RELEASE` in `GNUmakefile` marks the comparison boundary. For the first
Wander release it is `v1.6.5`, ensuring later Nomad history is never imported.
The release workflow always prepends the generated notes to `CHANGELOG.md`;
changelog generation is not optional for a release.

After a final release, the workflow advances `LAST_RELEASE` to the new Wander
tag. Corrections to an already published release are made directly in
`CHANGELOG.md` and called out in the correcting pull request.

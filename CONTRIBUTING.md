# Contributing to Wander

Wander is licensed under MPL-2.0. It continues the MPL-2.0 part of the
HashiCorp Nomad 1.6.5 codebase and has been developed independently since.

## Source rule: no code from the Nomad repository

Wander does not accept code copied from the HashiCorp Nomad repository.

- The inherited source is fixed. It is commit
  `a7cfff372cd52b02232a02bd3ffb5e5d35f88a0f` (`v1.6.5`) minus the 45 paths in
  [`docs/licenses/baseline-busl-exclusions.txt`](docs/licenses/baseline-busl-exclusions.txt).
  Nothing further is imported from the Nomad repository, from any branch, tag,
  release, pull request or fork.
- HashiCorp relicensed Nomad under the Business Source License 1.1 in 2023.
  Source from later Nomad releases and from Nomad Enterprise must not be
  copied, ported, translated, paraphrased or used as an implementation
  reference.
- The rule covers source files, tests, patches, commits, generated assets,
  configuration, website content and documentation.
- The rule covers bug fixes. A bug that Wander inherited from 1.6.5 is fixed
  independently, even when Nomad has fixed the same bug.
- The rule covers security fixes. They are written from public vulnerability
  information, not from the Nomad patch.
- The rule covers tool output. Code produced by an AI assistant or a code
  generator that was given later Nomad source as input is treated as derived
  from that source.
- Do not paste later Nomad source into issues, pull requests, discussions or
  review comments.

A matching file name, API route, field name or command name is not permission
to copy the implementation behind it.

## What a contribution may be based on

- The current Wander source, tests and Git history.
- The evidence listed in
  [`docs/plans/license-safe-parity-method.md`](docs/plans/license-safe-parity-method.md),
  which includes black-box behaviour of an official binary and public
  documentation.
- Your own design and your own tests.
- Third-party code under a licence compatible with MPL-2.0, with its origin
  and licence stated in the pull request.

[`docs/compatibility-boundary.md`](docs/compatibility-boundary.md) explains
which Nomad names Wander keeps for compatibility and which names new code uses.

## Pull requests

- State where the change comes from. Name any external reference you used.
- If you have read the later Nomad implementation of the behaviour you are
  changing, say so in the pull request.
- Sign off every commit with `git commit -s`. The DCO check requires a
  `Signed-off-by` line. The sign-off certifies that you have the right to
  submit the work under MPL-2.0.
- Add tests for new behaviour and run the checks in
  [`contributing/testing.md`](contributing/testing.md).

A pull request whose provenance cannot be established is not merged. The
affected code is rewritten independently or the change is dropped, as described
in [`docs/licenses/release-review.md`](docs/licenses/release-review.md).

## Developer documentation

Build, test and architecture notes are in the [`contributing`](contributing/)
directory. Report suspected vulnerabilities as described in
[SECURITY.md](SECURITY.md).

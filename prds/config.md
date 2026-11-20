# Norman Config

## Session Limits

max_tasks_per_session: 15
warn_at_tasks: 12

## Project

name: Wander Dynamic Host Volumes
repo: .
created: 2026-11-08

## Commands

build: make dev
test: make test
test_core: make test-wander
lint: make check
security: govulncheck ./...
generate: make generate-all
ui_generate: make ui-generate
ui_test: go test ./internal/web/... ./internal/templates/...
deslop: git diff --no-ext-diff --unified=3 HEAD | deslop . --diff - --only-changed --output /tmp/wander-deslop-check --nohtml --no-color

## Subagent Defaults

default_subagent: default

## Model Strategy

advisor_mode: always
advisor_model: gpt-6-astra
default_model: gpt-6.1-sol
quick_model: gpt-6-luna
progress_compress_after: 10
verify_rigor: adversarial
security_max_reqs_per_prd: 8

## Cleanup

sweep_mode: offer

## Project Constraints

- Use public post-1.6.5 interface documentation and independent black-box
  behavior only as clean-room compatibility evidence.
- Never use post-1.6.5 Nomad source, tests, patches, generated assets, commits,
  or implementation details.
- Keep private infrastructure artifacts local. Never commit, publish, log, or
  send their values to external services.
- Use synthetic data for every committed fixture, example, screenshot, and
  test.
- Write a failing test for each acceptance criterion before implementation.
- Run the licence-boundary checks and Deslop before every implementation
  commit.

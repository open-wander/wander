# Loop Config

## Session Limits
max_tasks_per_session: 15
warn_at_tasks: 12

## Project
name: UI Feature Parity - API Integration
repo: /Users/stokvis/dev/wander/wander
created: 2026-01-26

## Commands
typecheck: go build ./...
lint: golangci-lint run ./ui-htmx/...
test: go test ./ui-htmx/...
generate: make ui-generate

## Subagent Defaults
default_subagent: golang-pro
# phase_1_subagent: security-auditor (ACL/Auth)
# phase_3_subagent: golang-pro (Write operations)

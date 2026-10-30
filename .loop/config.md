# Loop Config

## Session Limits
max_tasks_per_session: 15
warn_at_tasks: 12

## Project
name: UI Migration - Rename and Feature Parity
repo: /Users/stokvis/dev/wander/wander
created: 2026-01-27

## Commands
typecheck: go build ./...
lint: golangci-lint run ./ui/...
test: go test ./ui/...
generate: cd ui && templ generate

## Security
vulnerabilities: govulncheck ./...
static_analysis: gosec -quiet -severity high -confidence high -exclude-dir=.release-notices -nosec-require-rules -nosec-require-justification ./...

## Subagent Defaults
default_subagent: general-purpose
# Comparison tasks: Explore agent to read both codebases
# Fix tasks: golang-pro for implementation


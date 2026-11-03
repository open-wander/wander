#!/usr/bin/env bash

set -euo pipefail

shopt -s nullglob

readonly commit_pattern='^[^[:space:]@]+@[0-9a-f]{40}$'
readonly digest_pattern='^docker://[^[:space:]]+@sha256:[0-9a-f]{64}$'

failed=0
files=(
  .github/workflows/*.yml
  .github/workflows/*.yaml
  .github/actions/*/action.yml
)

for file in "${files[@]}"; do
  while IFS=: read -r line_number line; do
    reference="${line#*uses:}"
    reference="${reference%%#*}"
    reference="${reference#"${reference%%[![:space:]]*}"}"
    reference="${reference%"${reference##*[![:space:]]}"}"

    if [[ "${reference}" == ./* ]]; then
      continue
    fi

    if [[ ! "${reference}" =~ ${commit_pattern} && ! "${reference}" =~ ${digest_pattern} ]]; then
      echo "${file}:${line_number}: external action is not pinned to a commit or digest: ${reference}" >&2
      failed=1
      continue
    fi

    if [[ "${line}" != *'#'* ]]; then
      echo "${file}:${line_number}: pinned action is missing a human-readable version comment" >&2
      failed=1
    fi
  done < <(grep -n -E '^[[:space:]]*(-[[:space:]]*)?uses:' "${file}" || true)
done

if git grep -n -E 'go install [^[:space:]]+@(latest|main|master)([[:space:]]|$)' -- \
  GNUmakefile Makefile 2>/dev/null; then
  echo "release tooling contains a mutable go install reference" >&2
  failed=1
fi

if (( failed != 0 )); then
  exit 1
fi

echo "workflow and release-tool pins verified"

#!/usr/bin/env bash

set -euo pipefail

readonly prohibited_pattern='SPDX-License-Identifier:[[:space:]]*BUSL-1[.]1|Business Source[[:space:]]+License'

if git grep -n -E "${prohibited_pattern}" -- \
  '*.go' '*.templ' '*.js' '*.ts' '*.css' '*.scss' '*.hbs' \
  '*.hcl' '*.sh' '*.yml' '*.yaml'; then
  echo "prohibited source license marker found" >&2
  exit 1
fi

echo "license boundary marker check passed"

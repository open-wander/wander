#!/usr/bin/env bash

set -euo pipefail

readonly prohibited_pattern='SPDX-License-Identifier:[[:space:]]*BUSL-1[.]1|Business Source[[:space:]]+License'
readonly vault_kubernetes_auth_module='github.com/hashicorp/vault/api/auth/kubernetes'
readonly allowed_vault_kubernetes_auth_version='v0.4.1'
readonly prohibited_release_package='github.com/tj/go-spin'

if git grep -n -E "${prohibited_pattern}" -- \
  '*.go' '*.templ' '*.js' '*.ts' '*.css' '*.scss' '*.hbs' \
  '*.hcl' '*.sh' '*.yml' '*.yaml'; then
  echo "prohibited source license marker found" >&2
  exit 1
fi

resolved_vault_kubernetes_auth_version="$({
  go list -m -f '{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}' \
    "${vault_kubernetes_auth_module}"
})"

if [[ "${resolved_vault_kubernetes_auth_version}" != "${allowed_vault_kubernetes_auth_version}" ]]; then
  echo "prohibited Vault Kubernetes auth dependency: resolved ${resolved_vault_kubernetes_auth_version}, expected ${allowed_vault_kubernetes_auth_version}" >&2
  exit 1
fi

if go list -deps -tags ui . | grep -Fqx "${prohibited_release_package}"; then
  echo "prohibited release dependency found: ${prohibited_release_package}" >&2
  exit 1
fi

echo "license boundary checks passed"

#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
readonly script_dir
repo_root=$(cd "${script_dir}/.." && pwd)
readonly repo_root
readonly requested_dir=${1:?usage: prepare-release-notices.sh PACKAGE_DIR}

mkdir -p "${requested_dir}"
package_dir=$(cd "${requested_dir}" && pwd)
readonly package_dir
case "${package_dir}" in
  "${repo_root}"/pkg/*_*) ;;
  *)
    echo "release notices may only be generated beneath ${repo_root}/pkg/GOOS_GOARCH" >&2
    exit 1
    ;;
esac

readonly notice_dir="${package_dir}/.release-notices"
staging_dir=$(mktemp -d "${package_dir}/.release-notices.XXXXXX")
readonly staging_dir
trap 'rm -rf "${staging_dir}"' EXIT

mkdir -p "${staging_dir}/licenses/go" "${staging_dir}/licenses/browser"

go run github.com/google/go-licenses@v1.6.0 save . \
  --save_path "${staging_dir}/licenses/go" \
  --force \
  --ignore github.com/open-wander/wander \
  --ignore github.com/hashicorp/vault/api/auth/kubernetes

vault_auth_dir=$(go list -m -f '{{.Dir}}' github.com/hashicorp/vault/api/auth/kubernetes)
readonly vault_auth_dir
if [[ ! -f "${vault_auth_dir}/LICENSE" ]]; then
  echo "Vault Kubernetes auth replacement has no LICENSE at ${vault_auth_dir}" >&2
  exit 1
fi

readonly vault_notice_dir="${staging_dir}/licenses/go/github.com/hashicorp/vault/api/auth/kubernetes"
mkdir -p "${vault_notice_dir}"
cp "${vault_auth_dir}/LICENSE" "${vault_notice_dir}/LICENSE"

cp "${repo_root}"/docs/licenses/third-party/*.txt "${staging_dir}/licenses/browser/"
cp "${repo_root}/LICENSE" "${staging_dir}/LICENSE.txt"
cp "${repo_root}/docs/licenses/RELEASE-NOTICES.md" "${staging_dir}/THIRD_PARTY_NOTICES.md"

rm -rf "${notice_dir}"
rm -rf "${package_dir}/licenses"
rm -f "${package_dir}/LICENSE.txt" "${package_dir}/THIRD_PARTY_NOTICES.md"
mv "${staging_dir}" "${notice_dir}"

echo "prepared release notices in ${notice_dir}"

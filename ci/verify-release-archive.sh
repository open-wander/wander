#!/usr/bin/env bash

set -euo pipefail

archive=${1:?usage: verify-release-archive.sh ARCHIVE GOOS}
goos=${2:?usage: verify-release-archive.sh ARCHIVE GOOS}

binary_name=wander
if [[ ${goos} == windows ]]; then
  binary_name=wander.exe
fi

binary_count=$(unzip -Z1 "${archive}" | awk -F/ -v binary="${binary_name}" '$NF == binary { count++ } END { print count + 0 }')
if [[ ${binary_count} -ne 1 ]]; then
  echo "expected exactly one ${binary_name} in ${archive}, found ${binary_count}" >&2
  exit 1
fi

verify_dir=$(mktemp -d)
trap 'rm -rf "${verify_dir}"' EXIT
unzip -q "${archive}" -d "${verify_dir}"

for required_path in \
  LICENSE.txt \
  THIRD_PARTY_NOTICES.md \
  licenses/go/github.com/hashicorp/vault/api/auth/kubernetes/LICENSE \
  licenses/browser/alpinejs-csp-MIT.txt \
  licenses/browser/htmx-0BSD.txt \
  licenses/browser/ui-build-MIT-notices.txt \
  licenses/browser/xterm-MIT.txt \
  licenses/browser/xterm-addon-fit-MIT.txt; do
  if [[ ! -s "${verify_dir}/${required_path}" ]]; then
    echo "release archive is missing required notice ${required_path}" >&2
    exit 1
  fi
done

go_license_count=$(find "${verify_dir}/licenses/go" -type f \( -name 'LICENSE*' -o -name 'NOTICE*' \) | wc -l | tr -d ' ')
if [[ ${go_license_count} -lt 2 ]]; then
  echo "release archive contains too few Go dependency notices: ${go_license_count}" >&2
  exit 1
fi

binary_path=$(find "${verify_dir}" -type f -name "${binary_name}" -print -quit)
build_metadata=$(go version -m "${binary_path}")
build_tags=$(awk '/^[[:space:]]*build[[:space:]]+-tags=/ { sub(/^[[:space:]]*build[[:space:]]+-tags=/, ""); gsub(/"/, ""); print }' <<<"${build_metadata}")
normalized_tags=",${build_tags// /,},"
if [[ ${normalized_tags} != *,ui,* ]]; then
  echo "${binary_name} was not built with the ui tag" >&2
  echo "${build_metadata}" >&2
  exit 1
fi

echo "verified ${archive}: one ${binary_name}, embedded UI build tag and release notices present"

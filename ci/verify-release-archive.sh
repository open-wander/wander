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

binary_path=$(find "${verify_dir}" -type f -name "${binary_name}" -print -quit)
build_metadata=$(go version -m "${binary_path}")
build_tags=$(awk '/^[[:space:]]*build[[:space:]]+-tags=/ { sub(/^[[:space:]]*build[[:space:]]+-tags=/, ""); gsub(/"/, ""); print }' <<<"${build_metadata}")
normalized_tags=",${build_tags// /,},"
if [[ ${normalized_tags} != *,ui,* ]]; then
  echo "${binary_name} was not built with the ui tag" >&2
  echo "${build_metadata}" >&2
  exit 1
fi

echo "verified ${archive}: one ${binary_name}, embedded UI build tag present"

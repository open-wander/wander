#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

version=${1:?usage: scripts/dist.sh VERSION}
if [[ ! ${version} =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z]+([.-][0-9A-Za-z]+)*)?$ ]]; then
  echo "invalid semantic release version: ${version}" >&2
  exit 1
fi

project_root=$(cd -P "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
package_dir="${project_root}/pkg"
dist_dir="${package_dir}/dist"

rm -rf "${dist_dir}"
mkdir -p "${dist_dir}"

artifact_count=0
while IFS= read -r -d '' artifact; do
  filename=$(basename "${artifact}")
  case ${filename} in
    linux_*.zip|darwin_*.zip|windows_*.zip|freebsd_*.zip|*.deb|*.rpm)
      cp "${artifact}" "${dist_dir}/wander_${version}_${filename}"
      artifact_count=$((artifact_count + 1))
      ;;
  esac
done < <(find "${package_dir}" -mindepth 1 -maxdepth 1 -type f \
  \( -name '*.zip' -o -name '*.deb' -o -name '*.rpm' \) -print0)

if [[ ${artifact_count} -eq 0 ]]; then
  echo "no release archives or native packages found in ${package_dir}" >&2
  exit 1
fi

(
  cd "${dist_dir}"
  find . -maxdepth 1 -type f ! -name '*_SHA256SUMS' -print0 |
    sort -z |
    xargs -0 shasum -a 256 >"wander_${version}_SHA256SUMS"
)

echo "prepared ${artifact_count} Wander artifacts and checksums in ${dist_dir}"
echo "publishing is intentionally handled by the manually dispatched GitHub Release workflow"

#!/usr/bin/env bash

set -euo pipefail

binary=${1:?usage: smoke-release-ui.sh WANDER_BINARY}
smoke_dir=$(mktemp -d)
agent_pid=

stop_agent() {
  if [[ -n ${agent_pid} ]] && kill -0 "${agent_pid}" 2>/dev/null; then
    kill -INT "${agent_pid}"
    wait "${agent_pid}" || true
  fi
  agent_pid=
}

cleanup() {
  stop_agent
  rm -rf "${smoke_dir}"
}
trap cleanup EXIT

wait_for_api() {
  for _ in $(seq 1 100); do
    if curl --fail --silent --output /dev/null http://127.0.0.1:4646/v1/agent/self; then
      return
    fi
    sleep 0.1
  done
  echo "Wander API did not become ready" >&2
  return 1
}

start_agent() {
  local data_dir=$1
  shift
  "${binary}" agent -dev -bind=127.0.0.1 -data-dir="${data_dir}" -log-level=WARN "$@" >"${smoke_dir}/agent.log" 2>&1 &
  agent_pid=$!
  wait_for_api
}

start_agent "${smoke_dir}/enabled"

headers=$(curl --silent --dump-header - --output /dev/null 'http://127.0.0.1:4646/?region=global')
grep -Eq '^HTTP/[^ ]+ 307' <<<"${headers}"
grep -Eiq '^location: /ui/\?region=global\r?$' <<<"${headers}"

ui_headers=$(curl --fail --location --silent --dump-header - --output "${smoke_dir}/ui.html" http://127.0.0.1:4646/ui/)
grep -q 'Wander' "${smoke_dir}/ui.html"
grep -Eiq '^content-security-policy: ' <<<"${ui_headers}"
curl --fail --silent --output /dev/null http://127.0.0.1:4646/ui/static/css/output.css
curl --fail --silent --output /dev/null http://127.0.0.1:4646/v1/agent/self

stop_agent
printf 'ui { enabled = false }\n' >"${smoke_dir}/disabled.hcl"
start_agent "${smoke_dir}/disabled" -config="${smoke_dir}/disabled.hcl"

curl --fail --silent http://127.0.0.1:4646/ui/ | grep -q 'Wander UI is disabled'
curl --fail --silent --output /dev/null http://127.0.0.1:4646/v1/agent/self

echo "verified packaged Wander runtime: embedded UI, redirect, CSP, API, and UI disablement"

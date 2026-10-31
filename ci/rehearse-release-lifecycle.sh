#!/usr/bin/env bash

set -euo pipefail

wander_archive=${1:?usage: rehearse-release-lifecycle.sh WANDER_ARCHIVE NOMAD_1_6_5_ARCHIVE}
nomad_archive=${2:?usage: rehearse-release-lifecycle.sh WANDER_ARCHIVE NOMAD_1_6_5_ARCHIVE}

lock_dir="${TMPDIR:-/tmp}/wander-release-lifecycle-${UID}.lock"
if ! mkdir "${lock_dir}" 2>/dev/null; then
  echo "another release lifecycle rehearsal is already running (${lock_dir})" >&2
  exit 1
fi

rehearsal_dir=$(mktemp -d)
base_port=${WANDER_REHEARSAL_BASE_PORT:-$((20000 + ($$ % 10000)))}
declare -a agent_pids=("" "" "")

stop_node() {
  local index=$1
  local pid=${agent_pids[index]}
  if [[ -n ${pid} ]] && kill -0 "${pid}" 2>/dev/null; then
    kill -INT "${pid}"
    wait "${pid}" || true
  fi
  agent_pids[index]=""
}

stop_cluster() {
  local index
  for index in 0 1 2; do
    stop_node "${index}"
  done
}

cleanup() {
  stop_cluster
  rm -rf "${rehearsal_dir}"
  rmdir "${lock_dir}"
}
trap cleanup EXIT

extract_binary() {
  local archive=$1
  local binary_name=$2
  local destination=$3
  local extract_dir="${rehearsal_dir}/extract-${binary_name}"

  mkdir -p "${extract_dir}"
  unzip -q "${archive}" -d "${extract_dir}"
  local source_path
  source_path=$(find "${extract_dir}" -type f -name "${binary_name}" -print -quit)
  if [[ -z ${source_path} ]]; then
    echo "${archive} does not contain ${binary_name}" >&2
    return 1
  fi
  cp "${source_path}" "${destination}"
  chmod +x "${destination}"
}

capture_cli_surface() {
  local binary=$1
  local output_prefix=$2
  local root_help="${output_prefix}-root-help"

  "${binary}" -help >"${root_help}" 2>&1
  awk '/^    [a-z][a-z-]+[[:space:]][[:space:]]/ { print $1 }' "${root_help}" | sort -u >"${output_prefix}-commands"

  local subcommand
  while read -r subcommand; do
    "${binary}" "${subcommand}" -help >"${output_prefix}-${subcommand}-help" 2>&1 || true
    sed -n 's/^  \(-[-a-z0-9][^ =]*\).*/\1/p' "${output_prefix}-${subcommand}-help" | \
      sort -u >"${output_prefix}-${subcommand}-flags"
  done <"${output_prefix}-commands"
}

command_exit_code() {
  local binary=$1
  shift
  local status
  set +e
  "${binary}" "$@" >/dev/null 2>&1
  status=$?
  set -e
  echo "${status}"
}

compare_cli_surface() {
  local baseline_prefix="${rehearsal_dir}/baseline-cli"
  local wander_prefix="${rehearsal_dir}/wander-cli"
  capture_cli_surface "${nomad_binary}" "${baseline_prefix}"
  capture_cli_surface "${wander_binary}" "${wander_prefix}"

  diff -u "${baseline_prefix}-commands" "${wander_prefix}-commands"
  local subcommand
  while read -r subcommand; do
    diff -u "${baseline_prefix}-${subcommand}-flags" "${wander_prefix}-${subcommand}-flags"
  done <"${baseline_prefix}-commands"

  local invocation
  for invocation in help no-command invalid-command; do
    local baseline_status wander_status
    case ${invocation} in
      help)
        baseline_status=$(command_exit_code "${nomad_binary}" -help)
        wander_status=$(command_exit_code "${wander_binary}" -help)
        ;;
      no-command)
        baseline_status=$(command_exit_code "${nomad_binary}")
        wander_status=$(command_exit_code "${wander_binary}")
        ;;
      invalid-command)
        baseline_status=$(command_exit_code "${nomad_binary}" not-a-command)
        wander_status=$(command_exit_code "${wander_binary}" not-a-command)
        ;;
    esac
    if [[ ${baseline_status} != "${wander_status}" ]]; then
      echo "CLI exit code differs for ${invocation}: Nomad=${baseline_status}, Wander=${wander_status}" >&2
      return 1
    fi
  done
}

http_port() {
  echo $((base_port + $1))
}

rpc_port() {
  echo $((base_port + 100 + $1))
}

serf_port() {
  echo $((base_port + 200 + $1))
}

write_configs() {
  local phase_dir=$1
  local retry_join
  retry_join="\"127.0.0.1:$(serf_port 0)\", \"127.0.0.1:$(serf_port 1)\", \"127.0.0.1:$(serf_port 2)\""

  local index
  for index in 0 1 2; do
    mkdir -p "${phase_dir}/node-${index}/data"
    cat >"${phase_dir}/node-${index}/agent.hcl" <<EOF
name = "rehearsal-${index}"
data_dir = "${phase_dir}/node-${index}/data"
bind_addr = "127.0.0.1"
log_level = "WARN"

ports {
  http = $(http_port "${index}")
  rpc  = $(rpc_port "${index}")
  serf = $(serf_port "${index}")
}

advertise {
  http = "127.0.0.1:$(http_port "${index}")"
  rpc  = "127.0.0.1:$(rpc_port "${index}")"
  serf = "127.0.0.1:$(serf_port "${index}")"
}

server {
  enabled          = true
  bootstrap_expect = 3
  server_join {
    retry_join = [${retry_join}]
  }
}

client {
  enabled = false
}
EOF
  done
}

start_node() {
  local phase_dir=$1
  local index=$2
  local binary=$3
  local log_path="${phase_dir}/node-${index}/agent.log"

  "${binary}" agent -config="${phase_dir}/node-${index}/agent.hcl" >"${log_path}" 2>&1 &
  agent_pids[index]=$!
}

dump_logs() {
  local phase_dir=$1
  local index
  for index in 0 1 2; do
    echo "--- node ${index} ---" >&2
    tail -100 "${phase_dir}/node-${index}/agent.log" >&2 || true
  done
}

wait_for_cluster() {
  local phase_dir=$1
  local leader_url raft_url
  leader_url="http://127.0.0.1:$(http_port 0)/v1/status/leader"
  raft_url="http://127.0.0.1:$(http_port 0)/v1/operator/raft/configuration"
  local attempt leader peers

  for ((attempt = 0; attempt < 300; attempt++)); do
    local index
    for index in 0 1 2; do
      if [[ -z ${agent_pids[index]} ]] || ! kill -0 "${agent_pids[index]}" 2>/dev/null; then
        echo "server ${index} exited while waiting for the cluster" >&2
        dump_logs "${phase_dir}"
        return 1
      fi
    done

    leader=$(curl --connect-timeout 1 --max-time 2 --fail --silent "${leader_url}" 2>/dev/null || true)
    peers=$(curl --connect-timeout 1 --max-time 2 --fail --silent "${raft_url}" 2>/dev/null | grep -o '"ID"' | wc -l | tr -d ' ' || true)
    if [[ ${leader} != '""' && ${peers} == 3 ]]; then
      return
    fi
    sleep 0.1
  done

  echo "three-server cluster did not become healthy" >&2
  dump_logs "${phase_dir}"
  return 1
}

start_cluster() {
  local phase_dir=$1
  local binary=$2
  local index
  for index in 0 1 2; do
    start_node "${phase_dir}" "${index}" "${binary}"
  done
  wait_for_cluster "${phase_dir}"
}

put_marker() {
  local marker=$1
  curl --connect-timeout 1 --max-time 5 --fail --silent \
    --request PUT \
    --header 'Content-Type: application/json' \
    --data "{\"Items\":{\"marker\":\"${marker}\"}}" \
    "http://127.0.0.1:$(http_port 0)/v1/var/rehearsal/state" >/dev/null
}

require_marker() {
  local marker=$1
  curl --connect-timeout 1 --max-time 5 --fail --silent \
    "http://127.0.0.1:$(http_port 0)/v1/var/rehearsal/state" | \
    grep -q "\"marker\":\"${marker}\""
}

roll_cluster() {
  local phase_dir=$1
  local binary=$2
  local marker=$3
  local index
  for index in 0 1 2; do
    stop_node "${index}"
    start_node "${phase_dir}" "${index}" "${binary}"
    wait_for_cluster "${phase_dir}"
    require_marker "${marker}"
  done
}

wander_binary="${rehearsal_dir}/wander"
nomad_binary="${rehearsal_dir}/nomad"
extract_binary "${wander_archive}" wander "${wander_binary}"
extract_binary "${nomad_archive}" nomad "${nomad_binary}"
compare_cli_surface
echo "verified inherited top-level commands and command flags"

clean_dir="${rehearsal_dir}/clean-install"
write_configs "${clean_dir}"
start_cluster "${clean_dir}" "${wander_binary}"
put_marker clean-install
roll_cluster "${clean_dir}" "${wander_binary}" clean-install
stop_cluster
echo "verified clean Wander install and three-server rolling restart"

upgrade_dir="${rehearsal_dir}/upgrade-rollback"
write_configs "${upgrade_dir}"
start_cluster "${upgrade_dir}" "${nomad_binary}"
put_marker upgrade-rollback
roll_cluster "${upgrade_dir}" "${wander_binary}" upgrade-rollback
echo "verified rolling upgrade from Nomad 1.6.5 to Wander"
roll_cluster "${upgrade_dir}" "${nomad_binary}" upgrade-rollback
echo "verified rollback from Wander to Nomad 1.6.5"

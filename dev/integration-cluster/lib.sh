# shellcheck shell=bash
# -------------------------------------------------------------------------------
# lib.sh -- Shared helpers for the integration cluster scripts
#
# Project: Wander
#
# Sourcing it first clears every NOMAD_*, WANDER_*, CONSUL_* and VAULT_*
# variable, so nothing here can reach a real cluster the operator's shell is
# configured for. Every address then points at the compose network.
# -------------------------------------------------------------------------------

# -------------------------------------------------------------------------
# ENVIRONMENT
# -------------------------------------------------------------------------

while read -r _var; do unset "$_var"; done < <(env | grep -oE '^(NOMAD|WANDER|CONSUL|VAULT)_[A-Za-z0-9_]*' || true)
unset _var

IT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export IT_DIR

SERVERS=(wander-server-1 wander-server-2 wander-server-3)
declare -A IP=(
  [consul-server-1]=10.77.0.11 [consul-server-2]=10.77.0.12 [consul-server-3]=10.77.0.13
  [vault]=10.77.0.20
  [wander-server-1]=10.77.0.31 [wander-server-2]=10.77.0.32 [wander-server-3]=10.77.0.33
  [wander-client-1]=10.77.0.41 [wander-client-2]=10.77.0.42 [wander-client-3]=10.77.0.43
)

export CONSUL_HTTP_ADDR=http://10.77.0.11:8500
export VAULT_ADDR=http://10.77.0.20:8200
export VAULT_TOKEN=root

# -------------------------------------------------------------------------
# CONTAINERS
# -------------------------------------------------------------------------

container() { echo "wander-it-$1-1"; }

is_running() { [[ "$(docker inspect -f '{{.State.Running}}' "$(container "$1")" 2>/dev/null)" == true ]]; }

# --- wait_for SECS DESC CMD...: poll CMD once a second ---
wait_for() {
  local secs=$1 desc=$2; shift 2
  local i
  for ((i = 0; i < secs; i++)); do
    if "$@" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  echo "timed out after ${secs}s waiting for: $desc" >&2
  return 1
}

# -------------------------------------------------------------------------
# API CLIENTS
#
# api talks to SERVER when it is set, otherwise to the first server that
# accepts a connection. Only a refused or timed-out connection moves on to the
# next server; an HTTP error is the answer.
# -------------------------------------------------------------------------

api() {
  local method=$1 path=$2 body=${3:-} s rc=1
  local -a candidates
  if [[ -n "${SERVER:-}" ]]; then candidates=("$SERVER"); else candidates=("${SERVERS[@]}"); fi
  for s in "${candidates[@]}"; do
    if [[ -n "$body" ]]; then
      curl -sf --connect-timeout 1 -m 15 -X "$method" -H 'Content-Type: application/json' \
        --data "$body" "http://${IP[$s]}:4646$path"
    else
      curl -sf --connect-timeout 1 -m 15 -X "$method" "http://${IP[$s]}:4646$path"
    fi
    rc=$?
    case $rc in 7|28) continue ;; *) return $rc ;; esac
  done
  return $rc
}

# --- cli ARGS...: the CLI inside a server, matching its agent ---
cli() {
  local s=${SERVER:-} c
  if [[ -z "$s" ]]; then
    for c in "${SERVERS[@]}"; do is_running "$c" && { s=$c; break; }; done
  fi
  [[ -n "$s" ]] || { echo "no server container running" >&2; return 1; }
  docker exec -i -e NOMAD_ADDR=http://127.0.0.1:4646 "$(container "$s")" wander "$@"
}

consul_api() { curl -sf -m 10 "$CONSUL_HTTP_ADDR$1"; }

vault_api() {
  local method=$1 path=$2 body=${3:-}
  if [[ -n "$body" ]]; then
    curl -s -m 10 -X "$method" -H "X-Vault-Token: $VAULT_TOKEN" --data "$body" "$VAULT_ADDR/v1/$path"
  else
    curl -s -m 10 -X "$method" -H "X-Vault-Token: $VAULT_TOKEN" "$VAULT_ADDR/v1/$path"
  fi
}

leader_container() {
  local addr s
  addr=$(api GET /v1/status/leader | tr -d '"') || return 1
  for s in "${SERVERS[@]}"; do
    [[ "$addr" == "${IP[$s]}:4647" ]] && { echo "$s"; return 0; }
  done
  return 1
}

# -------------------------------------------------------------------------
# JOBS AND ALLOCATIONS
# -------------------------------------------------------------------------

run_job() { cli job run -detach - < "$IT_DIR/jobs/$1.nomad.hcl" >/dev/null; }

# --- run_job_as FILE NAME: submit jobs/FILE under another ID ---
run_job_as() {
  sed "s/^job \"$1\"/job \"$2\"/" "$IT_DIR/jobs/$1.nomad.hcl" | cli job run -detach - >/dev/null
}

stop_job() { api DELETE "/v1/job/$1?purge=true" >/dev/null; }

running_allocs() {
  api GET "/v1/job/$1/allocations" | jq '[.[] | select(.ClientStatus == "running")] | length'
}

job_running_count_is() { [[ "$(running_allocs "$1")" == "$2" ]]; }

# --- alloc_ids JOB [GROUP]: running alloc IDs ---
alloc_ids() {
  api GET "/v1/job/$1/allocations" |
    jq -r --arg g "${2:-}" '.[] | select(.ClientStatus == "running" and ($g == "" or .TaskGroup == $g)) | .ID'
}

first_alloc() { alloc_ids "$@" | head -1; }

node_id() { api GET /v1/nodes | jq -r --arg n "$1" '.[] | select(.Name == $n) | .ID'; }

alloc_node_container() { api GET "/v1/allocation/$1" | jq -r '"wander-it-" + .NodeName + "-1"'; }

# --- alloc_file ALLOC TASK PATH: off disk, reaching secrets/ ---
alloc_file() { docker exec "$(alloc_node_container "$1")" cat "/var/lib/wander/alloc/$1/$2/$3"; }

# --- fs_cat ALLOC PATH: through a server, which forwards it ---
fs_cat() { api GET "/v1/client/fs/cat/$1?path=$2"; }

# --- task_logs ALLOC TASK [stdout|stderr] ---
task_logs() { api GET "/v1/client/fs/logs/$1?task=$2&type=${3:-stdout}&plain=true&origin=start&offset=0"; }

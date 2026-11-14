#!/usr/bin/env bash
# -------------------------------------------------------------------------------
# tests.sh -- Multi-node, Consul and Vault integration suite
#
# Project: Wander
#
# Runs every test in order, or only the tests named as arguments (the t_ prefix
# is optional). Order matters: the failure tests act on workloads the earlier
# tests deploy, then check that Consul, Vault and Connect still behave. Output
# is also written to results/<time>.txt.
# -------------------------------------------------------------------------------

set -uo pipefail
# shellcheck source=lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

STATE=$IT_DIR/results/.state
mkdir -p "$STATE"

# -------------------------------------------------------------------------
# ASSERTIONS
#
# Each assertion prints one ok or FAIL line, and a FAIL marks the running test
# failed. Command output is captured before it is matched: under pipefail,
# `cmd | grep -q` fails whenever grep exits early and cmd dies of SIGPIPE.
# -------------------------------------------------------------------------

T_FAILED=0
ok()   { printf '    ok    %s\n' "$*"; }
nok()  { printf '    FAIL  %s\n' "$*"; T_FAILED=1; return 1; }
note() { printf '    ..    %s\n' "$*"; }

# --- expect DESC CMD...: ok when CMD succeeds ---
expect() { local d=$1; shift; if "$@" >/dev/null 2>&1; then ok "$d"; else nok "$d"; fi; }

# --- eventually SECS DESC CMD...: ok once CMD succeeds ---
eventually() {
  local s=$1 d=$2 start=$SECONDS; shift 2
  if wait_for "$s" "$d" "$@" 2>/dev/null; then ok "$d ($((SECONDS - start))s)"; else nok "$d (gave up after ${s}s)"; fi
}

# --- on SERVER CMD...: dynamic scoping pins api and cli ---
on()  { local SERVER=$1; shift; "$@"; }
not() { ! "$@"; }

api_jq()    { local out; out=$(api GET "$1") && jq -e "$2" <<<"$out" >/dev/null; }
consul_jq() { local out; out=$(consul_api "$1") && jq -e "$2" <<<"$out" >/dev/null; }
json_true() { jq -e "$2" <<<"$1" >/dev/null; }

# --- outputs PATTERN CMD...: CMD succeeds, output matches ---
outputs() {
  local pat=$1 out; shift
  out=$("$@" 2>/dev/null) || return 1
  grep -E -i -q -- "$pat" <<<"$out"
}

log_contains()   { outputs "$3" task_logs "$1" "$2"; }
last_logs_are()  { [[ "$(task_logs "$1" "$2" | tail -n 2 | grep -cx -- "$3")" == 2 ]]; }
fs_contains()    { outputs "$3" fs_cat "$1" "$2"; }
fs_count()       { [[ "$(fs_cat "$1" "$2" | grep -c -- "$3")" == "$4" ]]; }
alloc_file_has() { outputs "$4" alloc_file "$1" "$2" "$3"; }

has_leader() { local l; l=$(api GET /v1/status/leader) && [[ "$l" == *:4647* ]]; }
no_leader()  { local l; l=$(api GET /v1/status/leader 2>/dev/null) || return 0; [[ "$l" == '""' ]]; }
autopilot_healthy() { api_jq /v1/operator/autopilot/health '.Healthy and ([.Servers[] | select(.Healthy)] | length == 3)'; }

marker()    { cat "$STATE/marker" 2>/dev/null; }
marker_on() { SERVER=$1 api_jq /v1/var/wander-it/state ".Items.marker == \"$(marker)\""; }

running_alloc_set() { api GET /v1/allocations | jq -r '.[] | select(.ClientStatus == "running") | .ID' | sort; }

# -------------------------------------------------------------------------
# CLUSTER AND CONSUL
# -------------------------------------------------------------------------

t_cluster_formed() {
  expect "3 raft peers"                           api_jq /v1/status/peers 'length == 3'
  expect "a leader is elected"                    has_leader
  expect "3 voters in the raft configuration"     api_jq /v1/operator/raft/configuration '[.Servers[] | select(.Voter)] | length == 3'
  expect "autopilot healthy, failure tolerance 1" api_jq /v1/operator/autopilot/health '.Healthy and .FailureTolerance == 1'
  expect "all servers alive, advertising build 1.6.5 for feature gates" \
    api_jq /v1/agent/members '[.Members[] | select(.Status == "alive" and .Tags.build == "1.6.5")] | length == 3'
  note "servers have no retry_join: they found each other through Consul"
}

t_clients_ready() {
  expect "3 clients ready and eligible" \
    api_jq /v1/nodes '[.[] | select(.Status == "ready" and .SchedulingEligibility == "eligible")] | length == 3'
  local id name
  for id in $(api GET /v1/nodes | jq -r '.[].ID'); do
    name=$(api GET "/v1/node/$id" | jq -r .Name)
    expect "$name: docker, exec, raw_exec healthy" \
      api_jq "/v1/node/$id" '.Drivers.docker.Healthy and .Drivers.exec.Healthy and .Drivers.raw_exec.Healthy'
    expect "$name: consul.connect, vault.accessible, CNI bridge fingerprinted" \
      api_jq "/v1/node/$id" '.Attributes["consul.connect"] == "true" and .Attributes["vault.accessible"] == "true" and .Attributes["plugins.cni.version.bridge"] != null'
  done
  note "clients have no servers list: they found the servers through Consul"
}

# --- Consul runs each new check within one 10s interval ---
t_consul_registration() {
  eventually 30 "3 servers registered in Consul (nomad/rpc, passing)" consul_jq '/v1/health/service/nomad?tag=rpc&passing' 'length == 3'
  eventually 30 "3 servers registered in Consul (nomad/http, passing)" consul_jq '/v1/health/service/nomad?tag=http&passing' 'length == 3'
  eventually 30 "3 clients registered in Consul (nomad-client, passing)" consul_jq '/v1/health/service/nomad-client?passing' 'length == 3'
}

t_variables() {
  local m s
  m="m-$RANDOM-$(date +%s)"
  echo "$m" > "$STATE/marker"
  expect "write a variable via wander-server-1" \
    on wander-server-1 api PUT /v1/var/wander-it/state "{\"Items\":{\"marker\":\"$m\"}}"
  for s in "${SERVERS[@]}"; do
    expect "read it back via $s" marker_on "$s"
  done
}

# -------------------------------------------------------------------------
# WORKLOADS
# -------------------------------------------------------------------------

t_raw_exec_spread() {
  run_job sleep-spread || { nok "submit sleep-spread"; return 1; }
  eventually 90 "6 raw_exec allocs running" job_running_count_is sleep-spread 6
  expect "spread across all 3 clients" \
    api_jq /v1/job/sleep-spread/allocations '[.[] | select(.ClientStatus == "running") | .NodeID] | unique | length == 3'
}

t_exec_driver() {
  run_job exec-batch || { nok "submit exec-batch"; return 1; }
  eventually 90 "exec batch alloc completes" api_jq /v1/job/exec-batch/allocations 'any(.[]; .ClientStatus == "complete")'
  local a
  a=$(api GET /v1/job/exec-batch/allocations | jq -r '[.[] | select(.ClientStatus == "complete")][0].ID')
  expect "stdout has exec-ok"               log_contains "$a" t exec-ok
  expect "task ran in a nomad.slice cgroup" log_contains "$a" t nomad.slice
  note "cgroup: $(task_logs "$a" t | grep '^0::' || true)"
}

web_instances_answer() {
  local addr n=0
  for addr in $(consul_api '/v1/health/service/it-web?passing' | jq -r '.[] | "\(.Service.Address):\(.Service.Port)"'); do
    outputs 'hello from' curl -sf -m 3 "http://$addr/" || return 1
    n=$((n + 1))
  done
  [[ $n == 3 ]]
}

t_docker_consul_service() {
  run_job web-consul || { nok "submit web-consul"; return 1; }
  eventually 120 "3 docker allocs running" job_running_count_is web-consul 3
  eventually 60  "3 it-web instances passing in Consul" consul_jq '/v1/health/service/it-web?passing' 'length == 3'
  expect "registered on 3 distinct client addresses" \
    consul_jq '/v1/health/service/it-web?passing' '[.[].Service.Address] | unique | length == 3'
  expect "tags and meta interpolated" \
    consul_jq '/v1/health/service/it-web?passing' 'length == 3 and all(.[]; (.Service.Tags | index("wander-it")) != null and (.Service.Meta.node | startswith("wander-client-")))'
  expect "every instance answers on its mapped port" web_instances_answer
}

exec_shows_alloc() { outputs "$1" cli alloc exec -i=false -t=false -task httpd "$1" cat /www/index.html; }
fs_lists_local()   { outputs local cli alloc fs "$1" httpd/; }
status_running()   { outputs 'client status += running' cli alloc status "$1"; }

t_alloc_exec() {
  local a node
  a=$(first_alloc web-consul) || { nok "find a web-consul alloc"; return 1; }
  node=$(api GET "/v1/allocation/$a" | jq -r .NodeName)
  note "alloc ${a:0:8} on $node, CLI run on wander-server-1"
  expect "alloc exec into a docker task on another node" on wander-server-1 exec_shows_alloc "$a"
  expect "alloc fs lists the task dir"                   on wander-server-1 fs_lists_local "$a"
  expect "alloc status renders"                          on wander-server-1 status_running "$a"
}

t_native_service_discovery() {
  run_job native-svc || { nok "submit native-svc"; return 1; }
  eventually 90 "4 allocs running" job_running_count_is native-svc 4
  eventually 30 "3 it-native registrations" api_jq /v1/service/it-native 'length == 3'
  expect "registrations span 3 clients" api_jq /v1/service/it-native '[.[].NodeID] | unique | length == 3'
  local r
  r=$(first_alloc native-svc reader)
  eventually 30 "nomadService template lists all 3" fs_count "$r" r/local/services.txt ':' 3
}

t_consul_template() {
  curl -sf -X PUT --data hello-1 "$CONSUL_HTTP_ADDR/v1/kv/wander-it/message" >/dev/null || { nok "consul kv put"; return 1; }
  run_job consul-template || { nok "submit consul-template"; return 1; }
  eventually 60 "alloc running" job_running_count_is consul-template 1
  local a
  a=$(first_alloc consul-template)
  eventually 30 "Consul KV rendered"                 fs_contains "$a" t/local/out.txt 'message=hello-1'
  eventually 30 "Consul catalog rendered (3 it-web)" fs_count "$a" t/local/out.txt '^web=' 3
  curl -sf -X PUT --data hello-2 "$CONSUL_HTTP_ADDR/v1/kv/wander-it/message" >/dev/null
  eventually 30 "re-rendered after the KV changed"   fs_contains "$a" t/local/out.txt 'message=hello-2'
}

# -------------------------------------------------------------------------
# VAULT AND CONNECT
# -------------------------------------------------------------------------

# --- vault_job_works JOB: secret rendered, task token valid ---
vault_job_works() {
  local job=$1 a token lookup
  eventually 90 "$job: alloc running" job_running_count_is "$job" 1 || return 1
  a=$(first_alloc "$job")
  eventually 30 "$job: KV v2 secret rendered into secrets/" \
    alloc_file_has "$a" t secrets/db.env 'password=correct-horse-battery-staple' || return 1
  expect "$job: VAULT_TOKEN in task env" log_contains "$a" t 'VAULT_TOKEN present'
  token=$(alloc_file "$a" t secrets/vault_token)
  lookup=$(vault_api POST auth/token/lookup "{\"token\":\"$token\"}")
  expect "$job: token carries the job's policy"   json_true "$lookup" '.data.policies | index("wander-it-read") != null'
  expect "$job: token derived from nomad-cluster" json_true "$lookup" '.data.role == "nomad-cluster"'
  expect "$job: token metadata names the alloc"   json_true "$lookup" ".data.meta.AllocationID == \"$a\""
  expect "$job: token is renewable with a TTL"    json_true "$lookup" '.data.renewable and .data.ttl > 0'
  echo "$token" > "$STATE/token-$job"
}

token_revoked() {
  vault_api POST auth/token/lookup "{\"token\":\"$1\"}" | jq -e '.errors' >/dev/null
}

t_vault_secret() {
  run_job vault-secret || { nok "submit vault-secret"; return 1; }
  vault_job_works vault-secret || return 1
  stop_job vault-secret
  eventually 60 "task token revoked once the alloc stopped" token_revoked "$(cat "$STATE/token-vault-secret")"
}

sidecar_passing() { consul_jq '/v1/health/service/it-api-sidecar-proxy?passing' 'length == 1'; }

sidecar_checks() {
  consul_api /v1/health/service/it-api-sidecar-proxy | jq -r '[.[].Checks[] | "\(.Name)=\(.Status)"] | join(", ")'
}

t_connect() {
  run_job connect || { nok "submit connect"; return 1; }
  eventually 180 "api + downstream allocs running" job_running_count_is connect 2
  local start=$SECONDS a
  # --- on a miss, record the checks and when they do pass ---
  eventually 60 "it-api sidecar passing in Consul" sidecar_passing || {
    note "sidecar checks: $(sidecar_checks)"
    wait_for 240 "sidecar passing" sidecar_passing 2>/dev/null &&
      note "sidecar passing after $((SECONDS - start))s"
  }
  a=$(first_alloc connect downstream)
  eventually 60 "downstream reaches api through Envoy" last_logs_are "$a" client connect-ok || {
    note "downstream tail: $(task_logs "$a" client | tail -n 3 | tr '\n' ' ')"
    note "sidecar stderr: $(task_logs "$a" connect-proxy-it-api-client stderr | tail -n 5 | tr '\n' ' ')"
    return 1
  }
  curl -sf -X PUT "$CONSUL_HTTP_ADDR/v1/config" \
    --data '{"Kind":"service-intentions","Name":"it-api","Sources":[{"Name":"it-api-client","Action":"deny"}]}' >/dev/null
  eventually 60 "deny intention enforced"            last_logs_are "$a" client FAIL
  curl -sf -X DELETE "$CONSUL_HTTP_ADDR/v1/config/service-intentions/it-api" >/dev/null
  eventually 60 "traffic allowed again once removed" last_logs_are "$a" client connect-ok
}

# -------------------------------------------------------------------------
# FAILURE AND RECOVERY
# -------------------------------------------------------------------------

leader_is_not() { local l; l=$(leader_container) && [[ $l != "$1" ]]; }

t_leader_failover() {
  local old new
  old=$(leader_container) || { nok "find the leader"; return 1; }
  note "killing leader $old"
  docker kill "$(container "$old")" >/dev/null
  eventually 30 "a new leader is elected" leader_is_not "$old"
  new=$(leader_container); note "new leader $new"
  expect "writes accepted with 2 of 3 servers" api PUT /v1/var/wander-it/failover '{"Items":{"ok":"1"}}'
  run_job_as vault-secret vault-secret-failover || nok "submit vault-secret-failover"
  vault_job_works vault-secret-failover
  stop_job vault-secret-failover
  eventually 60 "new leader revokes task tokens" token_revoked "$(cat "$STATE/token-vault-secret-failover" 2>/dev/null)"
  docker start "$(container "$old")" >/dev/null
  eventually 120 "old leader rejoins, autopilot healthy" autopilot_healthy
  expect "variable written before the failover survives" marker_on "$old"
}

allocs_all_lost() {
  local a
  for a in "$@"; do api_jq "/v1/allocation/$a" '.ClientStatus == "lost"' || return 1; done
}

t_client_failure() {
  local victim=wander-client-3 vid
  vid=$(node_id "$victim")
  local -a lost
  mapfile -t lost < <(api GET "/v1/node/$vid/allocations" | jq -r '.[] | select(.ClientStatus == "running" and .JobID == "sleep-spread") | .ID')
  expect "$victim hosts sleep-spread allocs (${#lost[@]})" test "${#lost[@]}" -gt 0 || return 1
  note "killing $victim (agent, Consul agent and dockerd)"
  docker kill "$(container "$victim")" >/dev/null
  eventually 120 "node marked down" api_jq "/v1/node/$vid" '.Status == "down"'
  eventually 30  "its allocs marked lost" allocs_all_lost "${lost[@]}"
  eventually 120 "sleep-spread back to 6 running on the survivors" job_running_count_is sleep-spread 6
  eventually 120 "web-consul back to 3 passing in Consul" consul_jq '/v1/health/service/it-web?passing' 'length == 3'
  docker start "$(container "$victim")" >/dev/null
  eventually 180 "$victim ready again" api_jq "/v1/node/$vid" '.Status == "ready"'
  eventually 120 "no stale it-web registrations left in Consul" consul_jq '/v1/catalog/service/it-web' 'length == 3'
  expect "lost allocs not resurrected (still 6)" job_running_count_is sleep-spread 6
}

t_drain() {
  local target=wander-client-1 tid a
  tid=$(node_id "$target")
  cli node drain -enable -yes -detach -deadline 3m "$tid" >/dev/null || { nok "start drain"; return 1; }
  eventually 200 "drain completes" api_jq "/v1/node/$tid" '.DrainStrategy == null and .SchedulingEligibility == "ineligible"'
  expect "nothing left running on $target" \
    api_jq "/v1/node/$tid/allocations" '[.[] | select(.ClientStatus == "running")] | length == 0'
  eventually 60  "sleep-spread 6 running" job_running_count_is sleep-spread 6
  eventually 60  "web-consul 3 passing"   consul_jq '/v1/health/service/it-web?passing' 'length == 3'
  eventually 120 "connect 2 running"      job_running_count_is connect 2
  a=$(first_alloc connect downstream)
  eventually 90 "Connect works after the drain" last_logs_are "$a" client connect-ok
  if cli node eligibility -enable "$tid" >/dev/null; then ok "$target eligible again"; else nok "re-enable eligibility"; fi
}

t_quorum_loss() {
  local leader follower survivor s
  leader=$(leader_container)
  for s in "${SERVERS[@]}"; do [[ $s != "$leader" ]] && { follower=$s; break; }; done
  for s in "${SERVERS[@]}"; do [[ $s != "$leader" && $s != "$follower" ]] && survivor=$s; done
  note "stopping $leader and $follower, leaving $survivor alone"
  docker stop -t 10 "$(container "$leader")" "$(container "$follower")" >/dev/null
  eventually 60 "no leader without quorum" on "$survivor" no_leader
  expect "writes rejected without quorum" not on "$survivor" api PUT /v1/var/wander-it/noquorum '{"Items":{"x":"1"}}'
  expect "stale reads still served"       on "$survivor" api_jq '/v1/jobs?stale=true' 'length > 0'
  docker start "$(container "$leader")" "$(container "$follower")" >/dev/null
  eventually 90  "leader re-elected" has_leader
  eventually 120 "autopilot healthy" autopilot_healthy
  expect "variable marker preserved" marker_on "$survivor"
  expect "jobs preserved"            api_jq /v1/jobs '[.[].ID] | index("sleep-spread") != null and index("connect") != null'
}

docker_tasks_running() { outputs . docker exec "$(container "$1")" docker ps -q; }

t_full_server_restart() {
  local before after
  before=$(running_alloc_set)
  note "$(wc -l <<<"$before") allocs running; stopping all 3 servers"
  docker stop -t 10 "$(container wander-server-1)" "$(container wander-server-2)" "$(container wander-server-3)" >/dev/null
  sleep 15
  expect "raw_exec workloads keep running with no servers" \
    docker exec "$(container wander-client-2)" pgrep -f 'sleep infinity'
  expect "docker workloads keep running with no servers" docker_tasks_running wander-client-2
  docker start "$(container wander-server-1)" "$(container wander-server-2)" "$(container wander-server-3)" >/dev/null
  eventually 120 "leader elected after full restart" has_leader
  eventually 120 "autopilot healthy" autopilot_healthy
  eventually 120 "all 3 clients ready" api_jq /v1/nodes '[.[] | select(.Status == "ready")] | length == 3'
  sleep 10
  after=$(running_alloc_set)
  expect "same allocs still running, none rescheduled" test "$before" == "$after"
  [[ "$before" == "$after" ]] || note "diff: $(diff <(echo "$before") <(echo "$after") | grep '^[<>]' | tr '\n' ' ')"
  expect "variable marker preserved" marker_on wander-server-2
  run_job_as vault-secret vault-secret-restart || nok "submit vault-secret-restart"
  vault_job_works vault-secret-restart
  stop_job vault-secret-restart
}

snapshot_inspects() { outputs index cli operator snapshot inspect /tmp/it.snap; }

t_snapshot() {
  expect "operator snapshot save"    on wander-server-1 cli operator snapshot save /tmp/it.snap
  expect "operator snapshot inspect" on wander-server-1 snapshot_inspects
}

# -------------------------------------------------------------------------
# MAIN
# -------------------------------------------------------------------------

ALL=(
  cluster_formed clients_ready consul_registration variables
  raw_exec_spread exec_driver docker_consul_service alloc_exec
  native_service_discovery consul_template vault_secret connect
  leader_failover client_failure drain quorum_loss full_server_restart snapshot
)

main() {
  local -a tests=("$@")
  [[ ${#tests[@]} -gt 0 ]] || tests=("${ALL[@]}")

  local out
  mkdir -p "$IT_DIR/results"
  out="$IT_DIR/results/$(date +%Y%m%dT%H%M%S).txt"
  exec > >(tee "$out") 2>&1

  echo "wander: $(docker exec "$(container wander-server-1)" wander version | tr '\n' ' ')"
  echo "consul: $(consul_api /v1/agent/self | jq -r .Config.Version)"
  echo "vault:  $(vault_api GET sys/seal-status | jq -r .version)"

  local -a results
  local t start
  for t in "${tests[@]}"; do
    t=${t#t_}
    printf '\n=== %s\n' "$t"
    T_FAILED=0; start=$SECONDS
    "t_$t" || T_FAILED=1
    if [[ $T_FAILED == 0 ]]; then results+=("PASS  $t ($((SECONDS - start))s)"); else results+=("FAIL  $t ($((SECONDS - start))s)"); fi
  done

  printf '\n=== summary\n'
  printf '  %s\n' "${results[@]}"
  echo "  results: $out"
  ! printf '%s\n' "${results[@]}" | grep -q '^FAIL'
}

# --- run only when executed, so the helpers can be sourced ---
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi

#!/bin/bash
# -------------------------------------------------------------------------------
# node-entrypoint -- Boots one integration node: cgroups, dockerd, Consul, agent
#
# Project: Wander
#
# Prepares the container's cgroup v2 tree, starts dockerd on clients and a
# local Consul agent everywhere, then runs the Wander agent under a restart
# loop. The agent is not PID 1, so agentctl can stop, kill and restart it
# without taking the container, its Consul agent or its dockerd down. ROLE
# (server or client) and CONSUL_JOIN (Consul server addresses) come from
# compose.yml.
# -------------------------------------------------------------------------------

set -euo pipefail

: "${ROLE:?ROLE must be server or client}"
ip=$(ip -4 -o addr show dev eth0 | awk '{print $4}' | cut -d/ -f1)
readonly ip

log() { printf '[entrypoint %s] %s\n' "$(hostname)" "$*"; }

# -------------------------------------------------------------------------
# CGROUP V2 NESTING
#
# dockerd and the exec driver enable controllers on the cgroup root, which v2
# refuses while any process sits directly in it, so every process moves into a
# leaf first, as the docker:dind image does. The PID list is read once per pass
# because cgroup.procs shifts while it is being emptied.
# -------------------------------------------------------------------------

if [[ -f /sys/fs/cgroup/cgroup.controllers ]]; then
  mkdir -p /sys/fs/cgroup/init
  for _ in $(seq 1 20); do
    mapfile -t pids < /sys/fs/cgroup/cgroup.procs
    for pid in "${pids[@]}"; do
      echo "$pid" > /sys/fs/cgroup/init/cgroup.procs 2>/dev/null || true
    done
    if sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers \
         > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null; then
      break
    fi
    sleep 0.2
  done
  if [[ -z "$(cat /sys/fs/cgroup/cgroup.subtree_control)" ]]; then
    log "could not enable cgroup controllers; root still holds: $(tr '\n' ' ' < /sys/fs/cgroup/cgroup.procs)"
    exit 1
  fi
  log "cgroup controllers: $(cat /sys/fs/cgroup/cgroup.subtree_control)"
fi

# --- /run survives a container restart; a host's tmpfs would not ---
rm -rf /run/docker /run/docker.pid /run/agent.pid /run/agent.hold

log "agent: $(wander version | head -1)"

# -------------------------------------------------------------------------
# DOCKERD (CLIENTS ONLY)
# -------------------------------------------------------------------------

if [[ "$ROLE" == client ]]; then
  dockerd --host=unix:///var/run/docker.sock \
          --exec-opt native.cgroupdriver=cgroupfs \
          --bip=172.31.250.1/24 \
          >/var/log/dockerd.log 2>&1 &
  for _ in $(seq 1 60); do docker info >/dev/null 2>&1 && break; sleep 1; done
  docker info >/dev/null 2>&1 || { log "dockerd did not start"; tail -50 /var/log/dockerd.log; exit 1; }
  # --- preloaded images; no Docker Hub pulls from nested daemons ---
  if [[ -f /preload/images.tar ]]; then
    docker load -q -i /preload/images.tar | sed "s/^/[entrypoint $(hostname)] /"
  fi
fi

# -------------------------------------------------------------------------
# LOCAL CONSUL AGENT
#
# Every Wander host runs one, as in production: Wander registers services and
# discovers its servers through 127.0.0.1:8500. Connect sidecars need gRPC on
# 8502, which Consul leaves off on client agents by default.
# -------------------------------------------------------------------------

join_args=()
for addr in ${CONSUL_JOIN:-}; do join_args+=("-retry-join=$addr"); done
consul agent \
  -data-dir=/var/lib/consul \
  -node="$(hostname)" \
  -bind="$ip" \
  -client=0.0.0.0 \
  -hcl='ports { grpc = 8502 }' \
  "${join_args[@]}" \
  >/var/log/consul.log 2>&1 &
for _ in $(seq 1 60); do consul members >/dev/null 2>&1 && break; sleep 1; done

# -------------------------------------------------------------------------
# AGENT LOOP
# -------------------------------------------------------------------------

readonly pidfile=/run/agent.pid
readonly hold=/run/agent.hold

trap 'kill -TERM "$(cat $pidfile 2>/dev/null)" 2>/dev/null; exit 0' TERM INT

while true; do
  # --- agentctl stop leaves a hold file; wait for it to go ---
  while [[ -f "$hold" ]]; do sleep 1; done
  wander agent -config=/etc/wander &
  echo $! > "$pidfile"
  wait $! || true
  sleep 1
done

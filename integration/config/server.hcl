# Copyright (c) Open Wander contributors
# SPDX-License-Identifier: MPL-2.0

# -------------------------------------------------------------------------------
# server.hcl -- Wander server agent config for the integration cluster
#
# Project: Wander
#
# Three servers bootstrap one Raft cluster with no server_join or retry_join.
# They find each other through the local Consul agent (consul.server_auto_join,
# default true), so a cluster that forms has exercised the Consul integration.
# -------------------------------------------------------------------------------

datacenter = "dc1"
region     = "global"
data_dir   = "/var/lib/wander"
bind_addr  = "0.0.0.0"
log_level  = "INFO"

# --- eth0, the compose network, on every node ---
advertise {
  http = "{{ GetInterfaceIP \"eth0\" }}"
  rpc  = "{{ GetInterfaceIP \"eth0\" }}"
  serf = "{{ GetInterfaceIP \"eth0\" }}"
}

server {
  enabled          = true
  bootstrap_expect = 3
}

consul {
  address = "127.0.0.1:8500"
}

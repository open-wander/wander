# -------------------------------------------------------------------------------
# server.hcl -- Wander server agent config for the integration cluster
#
# Project: Wander
#
# Three servers bootstrap one Raft cluster with no server_join or retry_join.
# They find each other through the local Consul agent (consul.server_auto_join,
# default true), so a cluster that forms has exercised the Consul integration.
# Vault uses the token integration, the only one Nomad 1.6.5 has.
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

# --- vault-init creates this token with a fixed ID ---
vault {
  enabled          = true
  address          = "http://10.77.0.20:8200"
  token            = "wander-it-server-token"
  create_from_role = "nomad-cluster"
}

# Copyright (c) Open Wander contributors
# SPDX-License-Identifier: MPL-2.0

# -------------------------------------------------------------------------------
# client.hcl -- Wander client agent config for the integration cluster
#
# Project: Wander
#
# Clients have no servers list: they discover the servers through the local
# Consul agent (consul.client_auto_join, default true). docker, exec and
# raw_exec are enabled, and bridge networking uses the Debian CNI plugins.
# -------------------------------------------------------------------------------

datacenter = "dc1"
region     = "global"
data_dir   = "/var/lib/wander"
bind_addr  = "0.0.0.0"
log_level  = "INFO"

# --- explicit: docker0 and the CNI bridge confuse GetPrivateIP ---
advertise {
  http = "{{ GetInterfaceIP \"eth0\" }}"
  rpc  = "{{ GetInterfaceIP \"eth0\" }}"
  serf = "{{ GetInterfaceIP \"eth0\" }}"
}

client {
  enabled           = true
  network_interface = "eth0"
  cni_path          = "/usr/lib/cni"
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

consul {
  address = "127.0.0.1:8500"
}

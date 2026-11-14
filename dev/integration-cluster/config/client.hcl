# -------------------------------------------------------------------------------
# client.hcl -- Wander client agent config for the integration cluster
#
# Project: Wander
#
# Clients have no servers list: they discover the servers through the local
# Consul agent (consul.client_auto_join, default true). docker, exec and
# raw_exec are enabled, bridge networking uses the Debian CNI plugins, and
# Connect sidecars reach Consul over gRPC on 8502.
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

plugin "docker" {
  config {
    volumes {
      enabled = true
    }
    # --- images are preloaded; GC would force Docker Hub pulls ---
    gc {
      image = false
    }
  }
}

consul {
  address      = "127.0.0.1:8500"
  grpc_address = "127.0.0.1:8502"
}

vault {
  enabled = true
  address = "http://10.77.0.20:8200"
}

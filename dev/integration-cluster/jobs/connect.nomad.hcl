# -------------------------------------------------------------------------------
# connect -- Consul Connect service mesh between two task groups
#
# Project: Wander
#
# An api group behind an Envoy sidecar, and a downstream group that calls it
# every 2s through its own sidecar's upstream on 127.0.0.1:8080, printing
# connect-ok or FAIL. Both groups use bridge networking through CNI, and the
# suite toggles a Consul intention to show Envoy enforcing it.
# -------------------------------------------------------------------------------

job "connect" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: api
  # ---------------------------------------------------------------------------

  group "api" {
    network {
      mode = "bridge"
    }

    service {
      name = "it-api"
      port = "9001"

      connect {
        sidecar_service {}
      }
    }

    # -------------------------------------------------------------------------
    # Task: api
    # -------------------------------------------------------------------------

    task "api" {
      driver = "docker"

      config {
        image   = "busybox:1.37"
        command = "sh"
        args    = ["-c", "mkdir -p /www && echo connect-ok > /www/index.html && exec httpd -f -p 9001 -h /www"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }

  # ---------------------------------------------------------------------------
  # Task Group: downstream
  # ---------------------------------------------------------------------------

  group "downstream" {
    network {
      mode = "bridge"
    }

    # -------------------------------------------------------------------------
    # Service
    # -------------------------------------------------------------------------

    service {
      name = "it-api-client"
      port = "9002"

      connect {
        sidecar_service {
          proxy {
            upstreams {
              destination_name = "it-api"
              local_bind_port  = 8080
            }
          }
        }
      }
    }

    # -------------------------------------------------------------------------
    # Task: client
    # -------------------------------------------------------------------------

    task "client" {
      driver = "docker"

      config {
        image   = "busybox:1.37"
        command = "sh"
        args    = ["-c", "while true; do wget -q -T 2 -O- http://127.0.0.1:8080/ 2>/dev/null || echo FAIL; sleep 2; done"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}

# -------------------------------------------------------------------------------
# web-consul -- Docker httpd registered in Consul with an HTTP health check
#
# Project: Wander
#
# Three busybox httpd tasks, one per client, each answering with its alloc ID
# on a mapped port. The Consul registration carries interpolated tags and meta,
# which the suite checks along with the health check and the port mapping.
# -------------------------------------------------------------------------------

job "web-consul" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: web
  # ---------------------------------------------------------------------------

  group "web" {
    count = 3

    # --- one per client ---
    spread {
      attribute = "${node.unique.name}"
    }

    network {
      port "http" {
        to = 8080
      }
    }

    # --- short health window keeps the drain test quick ---
    migrate {
      min_healthy_time = "2s"
    }

    # -------------------------------------------------------------------------
    # Service
    # -------------------------------------------------------------------------

    service {
      name = "it-web"
      port = "http"
      tags = ["wander-it", "index-${NOMAD_ALLOC_INDEX}"]

      meta {
        node = "${node.unique.name}"
      }

      check {
        type     = "http"
        path     = "/"
        interval = "3s"
        timeout  = "2s"
      }
    }

    # -------------------------------------------------------------------------
    # Task: httpd
    # -------------------------------------------------------------------------

    task "httpd" {
      driver = "docker"

      config {
        image   = "busybox:1.37"
        ports   = ["http"]
        command = "sh"
        args    = ["-c", "mkdir -p /www && echo \"hello from ${NOMAD_ALLOC_ID}\" > /www/index.html && exec httpd -f -p 8080 -h /www"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}

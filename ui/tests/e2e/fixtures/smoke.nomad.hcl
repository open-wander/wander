job "wander-ui-smoke" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
    scaling {
      enabled = true
      min     = 1
      max     = 2
    }

    service {
      name     = "wander-ui-http"
      provider = "nomad"
      port     = "http"
      tags     = ["ui", "smoke"]
    }

    network {
      port "http" {}
    }

    task "fixture" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "i=1; mkdir -p local/deep; while [ $i -le 30 ]; do echo fixture-$i > local/deep/file-$i.txt; i=$((i + 1)); done; while true; do echo wander-ui-smoke; sleep 5; done"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}

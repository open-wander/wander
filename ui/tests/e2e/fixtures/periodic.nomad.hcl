job "wander-ui-periodic" {
  datacenters = ["dc1"]
  type        = "batch"

  periodic {
    crons            = ["0 0 1 1 *"]
    prohibit_overlap = true
  }

  group "periodic" {
    task "complete" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "echo wander-ui-periodic"]
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

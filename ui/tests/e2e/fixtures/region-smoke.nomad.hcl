job "wander-ui-region-smoke" {
  datacenters = ["dc1"]
  type        = "service"

  group "remote" {
    task "fixture" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "while true; do echo wander-ui-region-smoke; sleep 5; done"]
      }

      resources {
        cpu    = 25
        memory = 16
      }
    }
  }
}

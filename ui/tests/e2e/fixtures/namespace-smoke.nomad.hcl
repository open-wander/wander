job "wander-ui-team-smoke" {
  datacenters = ["dc1"]
  type        = "service"

  group "worker" {
    task "fixture" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "while true; do echo wander-ui-team-smoke; sleep 5; done"]
      }

      resources {
        cpu    = 25
        memory = 16
      }
    }
  }
}

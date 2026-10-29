job "wander-ui-system" {
  datacenters = ["dc1"]
  type        = "system"

  group "daemon" {
    task "fixture" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "while true; do sleep 30; done"]
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

job "wander-ui-batch" {
  datacenters = ["dc1"]
  type        = "batch"

  group "batch" {
    task "complete" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "echo wander-ui-batch"]
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

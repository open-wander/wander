job "wander-ui-sysbatch" {
  datacenters = ["dc1"]
  type        = "sysbatch"

  group "batch" {
    task "complete" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "echo wander-ui-sysbatch"]
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

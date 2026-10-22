job "wander-ui-smoke" {
  datacenters = ["dc1"]
  type        = "service"

  group "web" {
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

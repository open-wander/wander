job "wander-ui-smoke" {
  datacenters = ["dc1"]
  type        = "service"
  node_pool   = "default"

  meta {
    test-purpose = "job-overview"
  }

  group "web" {
    update {
      max_parallel      = 1
      min_healthy_time  = "1s"
      healthy_deadline  = "1m"
      progress_deadline = "2m"
    }

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

    task "prepare" {
      driver = "raw_exec"

      lifecycle {
        hook    = "prestart"
        sidecar = false
      }

      config {
        command = "/bin/sh"
        args    = ["-c", "echo prepared > local/prepared.txt"]
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }

    task "fixture" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "i=1; mkdir -p local/deep; while [ $i -le 30 ]; do echo fixture-$i > local/deep/file-$i.txt; i=$((i + 1)); done; { echo FILE-HEAD-MARKER; i=1; while [ $i -le 12000 ]; do echo file-line-$i; i=$((i + 1)); done; echo FILE-TAIL-MARKER; } > local/deep/large.txt; echo wander-ui-smoke-head; while true; do echo wander-ui-smoke; sleep 5; done"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }

    task "rotation" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "echo ROTATION-PURGED-MARKER; i=1; while [ $i -le 50000 ]; do printf 'rotation-bulk-%06d-abcdefghijklmnopqrstuvwxyz-0123456789-ABCDEFGHIJKLMNOPQRSTUVWXYZ\\n' $i; i=$((i + 1)); done; i=1; while true; do echo rotation-live-$i; i=$((i + 1)); sleep 1; done"]
      }

      logs {
        max_files     = 2
        max_file_size = 1
      }

      resources {
        cpu    = 10
        memory = 10
      }
    }
  }
}

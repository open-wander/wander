# -------------------------------------------------------------------------------
# sleep-spread -- raw_exec workload spread across every client
#
# Project: Wander
#
# Six sleeping raw_exec tasks, two per client. They are what the failover,
# lost-client, drain and full-restart tests act on and count afterwards.
# -------------------------------------------------------------------------------

job "sleep-spread" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: sleep
  # ---------------------------------------------------------------------------

  group "sleep" {
    count = 6

    # --- two per client ---
    spread {
      attribute = "${node.unique.name}"
    }

    # --- short health windows keep the drain test quick ---
    migrate {
      max_parallel     = 2
      min_healthy_time = "2s"
      healthy_deadline = "1m"
    }

    update {
      min_healthy_time = "2s"
    }

    task "sleep" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

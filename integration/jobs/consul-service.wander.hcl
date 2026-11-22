# Copyright (c) Open Wander contributors
# SPDX-License-Identifier: MPL-2.0

# -------------------------------------------------------------------------------
# consul-service -- raw_exec tasks registered in Consul with a script check
#
# Project: Wander
#
# Three sleep tasks, one per client, each registering it-svc with a tag and
# its node's name interpolated into meta. The script check runs inside the
# task through the driver, so the service passes with nothing listening and no
# image to pull.
# -------------------------------------------------------------------------------

job "consul-service" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: svc
  # ---------------------------------------------------------------------------

  group "svc" {
    count = 3

    # --- one per client ---
    constraint {
      operator = "distinct_hosts"
      value    = "true"
    }

    # -------------------------------------------------------------------------
    # Task: sleep
    # -------------------------------------------------------------------------

    task "sleep" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      service {
        name = "it-svc"
        tags = ["wander-it"]

        meta {
          node = "${node.unique.name}"
        }

        check {
          type     = "script"
          command  = "/bin/true"
          interval = "3s"
          timeout  = "2s"
        }
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

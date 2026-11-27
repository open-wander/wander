# Copyright (c) Open Wander contributors
# SPDX-License-Identifier: MPL-2.0

# -------------------------------------------------------------------------------
# vault-secret -- Vault token integration: task token and a rendered secret
#
# Project: Wander
#
# The servers derive a task token from the wander-cluster role with the
# wander-it-read policy, and the client renders a KV v2 secret with it. The
# test checks the token's policy, role, metadata and TTL, and that it is
# revoked once the alloc stops.
# -------------------------------------------------------------------------------

job "vault-secret" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: secret
  # ---------------------------------------------------------------------------

  group "secret" {

    # -------------------------------------------------------------------------
    # Task: sleep
    # -------------------------------------------------------------------------

    task "sleep" {
      driver = "raw_exec"

      vault {
        policies    = ["wander-it-read"]
        change_mode = "noop"
      }

      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      # --- local/, since the fs API refuses to read secrets/ ---
      template {
        data        = <<EOT
password={{ with secret "secret/data/wander-it/db" }}{{ .Data.data.password }}{{ end }}
EOT
        destination = "local/db.env"
        change_mode = "noop"
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

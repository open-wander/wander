# -------------------------------------------------------------------------------
# vault-secret -- Vault token integration: task token and a rendered secret
#
# Project: Wander
#
# The server derives a task token from the nomad-cluster role with the
# wander-it-read policy, and the client renders a KV v2 secret with it. The
# suite checks the token's policy, role, metadata and TTL, and that it is
# revoked once the alloc stops.
# -------------------------------------------------------------------------------

job "vault-secret" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: g
  # ---------------------------------------------------------------------------

  group "g" {
    task "t" {
      driver = "raw_exec"

      vault {
        policies    = ["wander-it-read"]
        change_mode = "noop"
      }

      config {
        command = "/bin/sh"
        args    = ["-c", "test -n \"$VAULT_TOKEN\" && echo VAULT_TOKEN present; exec sleep infinity"]
      }

      template {
        data        = <<EOT
password={{ with secret "secret/data/wander-it/db" }}{{ .Data.data.password }}{{ end }}
EOT
        destination = "secrets/db.env"
        change_mode = "noop"
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

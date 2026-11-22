# Copyright (c) Open Wander contributors
# SPDX-License-Identifier: MPL-2.0

# -------------------------------------------------------------------------------
# consul-template -- Consul KV and catalog rendered by the client's templates
#
# Project: Wander
#
# One raw_exec task whose template reads a Consul KV key and the Consul
# servers' own consul service. The test compares the file with what Consul's
# API reports, changes the key, and checks the file is rendered again.
# -------------------------------------------------------------------------------

job "consul-template" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: render
  # ---------------------------------------------------------------------------

  group "render" {

    # -------------------------------------------------------------------------
    # Task: sleep
    # -------------------------------------------------------------------------

    task "sleep" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      template {
        data        = <<EOT
message={{ key "wander-it/message" }}
{{ range service "consul" }}consul={{ .Address }}:{{ .Port }}
{{ end }}
EOT
        destination = "local/out.txt"
        change_mode = "noop"
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

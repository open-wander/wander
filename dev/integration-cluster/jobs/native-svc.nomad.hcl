# -------------------------------------------------------------------------------
# native-svc -- Built-in service discovery across clients, without Consul
#
# Project: Wander
#
# Three registrations with provider = "nomad", one per client, and a reader
# whose nomadService template lists them. The suite checks the registrations
# span all three clients and the template renders all three addresses.
# -------------------------------------------------------------------------------

job "native-svc" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: svc
  # ---------------------------------------------------------------------------

  group "svc" {
    count = 3

    # --- one per client ---
    spread {
      attribute = "${node.unique.name}"
    }

    network {
      port "p" {}
    }

    service {
      provider = "nomad"
      name     = "it-native"
      port     = "p"
    }

    task "t" {
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

  # ---------------------------------------------------------------------------
  # Task Group: reader
  # ---------------------------------------------------------------------------

  group "reader" {
    task "r" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      template {
        data        = <<EOT
{{ range nomadService "it-native" }}{{ .Address }}:{{ .Port }}
{{ end }}
EOT
        destination = "local/services.txt"
        change_mode = "noop"
      }

      resources {
        cpu    = 20
        memory = 16
      }
    }
  }
}

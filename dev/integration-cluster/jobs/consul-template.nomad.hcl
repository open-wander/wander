# -------------------------------------------------------------------------------
# consul-template -- Consul KV and catalog rendered by the client's templates
#
# Project: Wander
#
# One raw_exec task whose template reads a Consul KV key and the it-web service
# catalog. The suite changes the key and checks the file is re-rendered.
# -------------------------------------------------------------------------------

job "consul-template" {
  datacenters = ["dc1"]

  # ---------------------------------------------------------------------------
  # Task Group: g
  # ---------------------------------------------------------------------------

  group "g" {
    task "t" {
      driver = "raw_exec"

      config {
        command = "/bin/sleep"
        args    = ["infinity"]
      }

      template {
        data        = <<EOT
message={{ key "wander-it/message" }}
{{ range service "it-web" }}web={{ .Address }}:{{ .Port }}
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

job "wander-ui-parameterized" {
  datacenters = ["dc1"]
  type        = "batch"

  parameterized {
    payload       = "required"
    meta_required = ["environment"]
    meta_optional = ["version"]
  }

  group "worker" {
    task "fixture" {
      driver = "raw_exec"

      config {
        command = "/bin/sh"
        args    = ["-c", "cat $NOMAD_TASK_DIR/dispatch_payload && echo $NOMAD_META_environment $NOMAD_META_version"]
      }

      resources {
        cpu    = 25
        memory = 16
      }
    }
  }
}

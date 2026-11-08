package templates

import "github.com/open-wander/wander/site/examples"

// HelloWanderJob is the tested job specification shown in the first-job
// tutorial. Keeping the example in Go makes the rendered page and parser test
// consume exactly the same source.
const HelloWanderJob = `job "hello-wander" {
  datacenters = ["dc1"]
  type        = "service"

  group "hello" {
    count = 1

    task "messages" {
      driver = "docker"

      config {
        image   = "busybox:1.36"
        command = "/bin/sh"
        args = ["-c", "while true; do date; echo 'hello from Wander'; sleep 5; done"]
      }

      resources {
        cpu    = 50
        memory = 32
      }
    }
  }
}`

const (
	MultiNodeServerConfig    = examples.MultiNodeServerConfig
	MultiNodeClientOneConfig = examples.MultiNodeClientOneConfig
	MultiNodeClientTwoConfig = examples.MultiNodeClientTwoConfig
)

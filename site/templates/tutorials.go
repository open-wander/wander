package templates

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

// MultiNodeServerConfig is the single-server configuration used by the
// disposable multi-node tutorial.
const MultiNodeServerConfig = `name      = "wander-server"
bind_addr = "127.0.0.1"

server {
  enabled          = true
  bootstrap_expect = 1
}`

// MultiNodeClientOneConfig and MultiNodeClientTwoConfig keep each local
// client's HTTP endpoint distinct. Docker cleanup is disabled because both
// agents share the same local Docker daemon in this disposable lab.
const MultiNodeClientOneConfig = `name = "wander-client-1"

client {
  enabled = true
  servers = ["127.0.0.1:4647"]
}

ports {
  http = 5656
}

plugin "docker" {
  config {
    gc {
      dangling_containers {
        enabled = false
      }
    }
  }
}`

const MultiNodeClientTwoConfig = `name = "wander-client-2"

client {
  enabled = true
  servers = ["127.0.0.1:4647"]
}

ports {
  http = 5657
}

plugin "docker" {
  config {
    gc {
      dangling_containers {
        enabled = false
      }
    }
  }
}`

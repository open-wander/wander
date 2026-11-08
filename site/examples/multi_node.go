// Package examples contains configuration and job examples published on the
// Wander documentation site. Keeping executable examples outside the rendered
// templates lets their owning packages validate them without depending on
// generated site code.
package examples

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

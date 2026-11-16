# -------------------------------------------------------------------------------
# wander-it-node -- Wander server or client host for the integration cluster
#
# Project: Wander
#
# One image for every Wander node in the test cluster; ROLE picks server or
# client at runtime. TestMain builds it from a context holding this file, the
# scripts beside it and the checkout's linux/amd64 wander binary, and passes
# CONSUL_IMAGE. Clients run their own dockerd and CNI plugins.
# -------------------------------------------------------------------------------

ARG CONSUL_IMAGE
ARG DEBIAN_IMAGE=debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2

# -------------------------------------------------------------------------------
# Stage: consul -- the Consul binary from its published image
# -------------------------------------------------------------------------------

FROM ${CONSUL_IMAGE} AS consul

# -------------------------------------------------------------------------------
# Stage: node -- Debian runtime with dockerd, CNI and iptables
#
# Clients run their own dockerd rather than the host's socket. A shared daemon
# resolves alloc-dir bind mounts on the host's filesystem, puts bridge-mode
# netns paths in the wrong PID namespace, and lets each client's
# dangling-container reconciler kill the other clients' containers.
# -------------------------------------------------------------------------------

FROM ${DEBIAN_IMAGE}

# --- dockerd, CNI plugins, and the ip/iptables tools they call ---
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates curl jq procps iproute2 iptables tini \
      docker.io docker-cli containernetworking-plugins; \
    rm -rf /var/lib/apt/lists/*

# --- TestMain writes these with mode 0755; COPY keeps it ---
COPY wander                    /usr/local/bin/wander
COPY --from=consul /bin/consul /usr/local/bin/consul
COPY entrypoint.sh             /usr/local/sbin/node-entrypoint
COPY agentctl                  /usr/local/sbin/agentctl

ENTRYPOINT ["/usr/bin/tini", "-g", "--", "/usr/local/sbin/node-entrypoint"]

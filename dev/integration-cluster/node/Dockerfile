# -------------------------------------------------------------------------------
# wander-it-node -- Wander server or client host for the integration cluster
#
# Project: Wander
#
# One image for every Wander node in the test cluster; ROLE picks server or
# client at runtime. It carries the Wander binary from WANDER_IMAGE (a release,
# or a source build wrapped by `make local-image`), a Consul agent, and for
# clients their own dockerd and CNI plugins.
# -------------------------------------------------------------------------------

ARG WANDER_IMAGE=ghcr.io/open-wander/wander:0.1.1
ARG CONSUL_IMAGE=hashicorp/consul:2.1.0

# -------------------------------------------------------------------------------
# Stage: binaries -- Wander and Consul from their published images
# -------------------------------------------------------------------------------

FROM ${WANDER_IMAGE} AS wander
FROM ${CONSUL_IMAGE} AS consul

# -------------------------------------------------------------------------------
# Stage: node -- Debian runtime with dockerd, CNI and iptables
#
# Clients run their own dockerd rather than the host's socket. A shared daemon
# resolves alloc-dir bind mounts on the host's filesystem, puts bridge-mode
# netns paths in the wrong PID namespace, and lets each client's
# dangling-container reconciler kill the other clients' containers.
# -------------------------------------------------------------------------------

FROM debian:trixie-slim

# --- dockerd, CNI plugins, and the ip/iptables tools they call ---
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates curl jq procps iproute2 iptables tini \
      docker.io docker-cli containernetworking-plugins; \
    rm -rf /var/lib/apt/lists/*

COPY --from=wander /bin/wander /usr/local/bin/wander
COPY --from=consul /bin/consul /usr/local/bin/consul

COPY --chmod=755 entrypoint.sh /usr/local/sbin/node-entrypoint
COPY --chmod=755 agentctl      /usr/local/sbin/agentctl

ENTRYPOINT ["/usr/bin/tini", "-g", "--", "/usr/local/sbin/node-entrypoint"]

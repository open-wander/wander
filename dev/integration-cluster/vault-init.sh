#!/bin/sh
# -------------------------------------------------------------------------------
# vault-init -- Configures the dev Vault for Wander's token integration
#
# Project: Wander
#
# Writes the server policy and the nomad-cluster token role from Nomad 1.6's
# Vault integration guide, a read policy and KV v2 secret for the test jobs,
# and a periodic orphan token for the servers. The token has a fixed ID so
# config/server.hcl can stay static, and every step is idempotent.
# -------------------------------------------------------------------------------

set -eu

export VAULT_ADDR=http://10.77.0.20:8200
export VAULT_TOKEN=root

until vault status >/dev/null 2>&1; do sleep 1; done

# --- server policy, and the role task tokens are derived from ---
vault policy write nomad-server /init/nomad-server-policy.hcl
vault write auth/token/roles/nomad-cluster @/init/nomad-cluster-role.json

# --- what the test jobs' task tokens may read ---
vault policy write wander-it-read - <<'EOF'
path "secret/data/wander-it/*" {
  capabilities = ["read"]
}
EOF

vault kv put secret/wander-it/db password=correct-horse-battery-staple

# --- the servers' token, with the ID server.hcl names ---
if ! vault token lookup wander-it-server-token >/dev/null 2>&1; then
  vault token create -id=wander-it-server-token -policy=nomad-server \
    -period=72h -orphan -display-name=wander-servers >/dev/null
fi

echo "vault-init: done"

#!/bin/sh
# Copyright (c) Open Wander contributors
# SPDX-License-Identifier: MPL-2.0

# -------------------------------------------------------------------------------
# vault-init -- Configures the dev Vault for Wander's token integration
#
# Project: Wander
#
# Writes the wander-server policy and the wander-cluster token role, a read
# policy and KV v2 secret for the test job, and a periodic orphan token for
# the servers. The token has a fixed ID so config/server.hcl can stay static,
# and every step is idempotent.
# -------------------------------------------------------------------------------

set -eu

export VAULT_ADDR=http://10.77.0.20:8200
export VAULT_TOKEN=root

until vault status >/dev/null 2>&1; do sleep 1; done

# --- what the servers' token needs to derive and revoke task tokens ---
vault policy write wander-server - <<'EOF'
path "auth/token/create/wander-cluster" {
  capabilities = ["update"]
}
path "auth/token/roles/wander-cluster" {
  capabilities = ["read"]
}
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
path "auth/token/lookup" {
  capabilities = ["update"]
}
path "auth/token/revoke-accessor" {
  capabilities = ["update"]
}
path "sys/capabilities-self" {
  capabilities = ["update"]
}
path "auth/token/renew-self" {
  capabilities = ["update"]
}
EOF

# --- the role task tokens are derived from ---
vault write auth/token/roles/wander-cluster \
  disallowed_policies=wander-server orphan=false renewable=true \
  token_period=72h token_explicit_max_ttl=0

# --- what the test job's task token may read ---
vault policy write wander-it-read - <<'EOF'
path "secret/data/wander-it/*" {
  capabilities = ["read"]
}
EOF

vault kv put secret/wander-it/db password=correct-horse-battery-staple

# --- the servers' token, with the ID server.hcl names ---
if ! vault token lookup wander-it-server-token >/dev/null 2>&1; then
  vault token create -id=wander-it-server-token -policy=wander-server \
    -period=72h -orphan -display-name=wander-servers >/dev/null
fi

echo "vault-init: done"

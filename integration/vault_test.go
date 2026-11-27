// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

// -------------------------------------------------------------------------------
// Vault Integration Tests
//
// Project: Wander
//
// Clients reach Vault, and the servers derive a token for each task that asks
// for one: the task renders a secret with it, and the token is revoked when
// the alloc stops. Each test reads Vault's API with the dev root token.
// -------------------------------------------------------------------------------

//go:build integration

package integration

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/shoenig/test/must"
)

// vaultTimeout bounds how long Vault and the clients may take to reflect a
// change in the cluster.
const vaultTimeout = time.Minute

// TestVault_ClientsFingerprint proves every client reaches Vault and reports
// it in the vault.accessible node attribute.
func TestVault_ClientsFingerprint(t *testing.T) {
	client := servers["wander-server-1"]
	eventually(t, vaultTimeout, func() error {
		nodes, _, err := client.Nodes().List(nil)
		if err != nil {
			return err
		}
		if len(nodes) != 3 {
			return fmt.Errorf("%d nodes, want 3", len(nodes))
		}
		for _, stub := range nodes {
			node, _, err := client.Nodes().Info(stub.ID, nil)
			if err != nil {
				return err
			}
			if got := node.Attributes["vault.accessible"]; got != "true" {
				return fmt.Errorf("%s: vault.accessible is %q, want true", node.Name, got)
			}
		}
		return nil
	})
}

// TestVault_TaskToken proves the servers derive a task token from the
// wander-cluster role with the job's policy, the client renders a KV v2 secret
// with it, and the token is revoked once the alloc stops.
func TestVault_TaskToken(t *testing.T) {
	client := servers["wander-server-1"]
	jobID := runJob(t, client, "vault-secret")
	allocID := runningAllocs(t, client, jobID, 1)[0].ID

	eventually(t, vaultTimeout, func() error {
		out, err := allocFile(client, allocID, "sleep/local/db.env")
		if err != nil {
			return err
		}
		if !strings.Contains(out, "password=correct-horse-battery-staple") {
			return fmt.Errorf("secret not rendered, db.env is %q", out)
		}
		return nil
	})

	var accessor string
	var token *vaultapi.Secret
	eventually(t, vaultTimeout, func() error {
		var err error
		accessor, token, err = allocToken(allocID)
		return err
	})

	policies, err := token.TokenPolicies()
	must.NoError(t, err)
	must.SliceContains(t, policies, "wander-it-read")
	role, _ := token.Data["role"].(string)
	must.Eq(t, "wander-cluster", role)
	renewable, err := token.TokenIsRenewable()
	must.NoError(t, err)
	must.True(t, renewable, must.Sprint("task token is renewable"))
	ttl, err := token.TokenTTL()
	must.NoError(t, err)
	must.True(t, ttl > 0, must.Sprintf("task token TTL is %s", ttl))

	_, _, err = client.Jobs().Deregister(jobID, false, nil)
	must.NoError(t, err)

	// The accessor was live above, so its absence from a good list is a revoke.
	eventually(t, vaultTimeout, func() error {
		accessors, err := tokenAccessors()
		if err != nil {
			return err
		}
		if slices.Contains(accessors, accessor) {
			return fmt.Errorf("task token %s is not revoked", accessor)
		}
		return nil
	})
}

// -------------------------------------------------------------------------
// HELPERS
// -------------------------------------------------------------------------

// allocToken looks up every token accessor in Vault and returns the accessor
// and lookup of the token whose metadata names allocID.
func allocToken(allocID string) (string, *vaultapi.Secret, error) {
	accessors, err := tokenAccessors()
	if err != nil {
		return "", nil, err
	}
	for _, acc := range accessors {
		token, err := vaultClient.Auth().Token().LookupAccessor(acc)
		if err != nil {
			return "", nil, err
		}
		meta, err := token.TokenMetadata()
		if err != nil {
			return "", nil, err
		}
		if meta["AllocationID"] == allocID {
			return acc, token, nil
		}
	}
	return "", nil, fmt.Errorf("no token for alloc %s among %d accessors", allocID, len(accessors))
}

// tokenAccessors lists the accessor of every live token in Vault. An empty
// list is an error, since the root and server tokens always exist.
func tokenAccessors() ([]string, error) {
	secret, err := vaultClient.Logical().List("auth/token/accessors")
	if err != nil {
		return nil, err
	}
	var keys []any
	if secret != nil {
		keys, _ = secret.Data["keys"].([]any)
	}
	accessors := make([]string, 0, len(keys))
	for _, k := range keys {
		if s, ok := k.(string); ok {
			accessors = append(accessors, s)
		}
	}
	if len(accessors) == 0 {
		return nil, errors.New("no token accessors listed")
	}
	return accessors, nil
}

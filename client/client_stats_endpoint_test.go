// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/nomad/mock"
	nstructs "github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestClientStats_Stats(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	client, cleanup := TestClient(t, nil)
	defer cleanup()

	req := &nstructs.NodeSpecificRequest{}
	var resp structs.ClientStatsResponse
	require.Nil(client.ClientRPC("ClientStats.Stats", &req, &resp))
	require.NotNil(resp.HostStats)
	require.NotNil(resp.HostStats.AllocDirStats)
	require.NotZero(resp.HostStats.Uptime)
}

func TestClientStats_Stats_ACL(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	server, addr, root, cleanupS := testACLServer(t, nil)
	defer cleanupS()

	client, cleanupC := TestClient(t, func(c *config.Config) {
		c.Servers = []string{addr}
		c.ACLEnabled = true
	})
	defer cleanupC()

	// Try request without a token and expect failure
	{
		req := &nstructs.NodeSpecificRequest{}
		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)
		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with an invalid token and expect failure
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1005, "invalid", mock.NodePolicy(acl.PolicyDeny))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)

		require.NotNil(err)
		require.EqualError(err, nstructs.ErrPermissionDenied.Error())
	}

	// Try request with a valid token
	{
		token := mock.CreatePolicyAndToken(t, server.State(), 1007, "valid", mock.NodePolicy(acl.PolicyRead))
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = token.SecretID

		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)

		require.Nil(err)
		require.NotNil(resp.HostStats)
	}

	// Try request with a management token
	{
		req := &nstructs.NodeSpecificRequest{}
		req.AuthToken = root.SecretID

		var resp structs.ClientStatsResponse
		err := client.ClientRPC("ClientStats.Stats", &req, &resp)

		require.Nil(err)
		require.NotNil(resp.HostStats)
	}
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/helper/pointer"
	"github.com/open-wander/wander/nomad"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

func TestNodeMeta_ACL(t *testing.T) {
	ci.Parallel(t)

	s, _, cleanupS := nomad.TestACLServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.ACLEnabled = true
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	// Dynamic node metadata endpoints should fail without auth
	applyReq := &structs.NodeMetaApplyRequest{
		NodeID: c1.NodeID(),
		Meta: map[string]*string{
			"foo": pointer.Of("bar"),
		},
	}

	resp := structs.NodeMetaResponse{}
	err := c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())

	readReq := &structs.NodeSpecificRequest{
		NodeID: c1.NodeID(),
	}
	err = c1.ClientRPC("NodeMeta.Read", readReq, &resp)
	must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())

	// Create a token to make it work
	policyGood := mock.NodePolicy(acl.PolicyWrite)
	tokenGood := mock.CreatePolicyAndToken(t, s.State(), 1009, "meta", policyGood)

	applyReq.AuthToken = tokenGood.SecretID
	err = c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.NoError(t, err)
	must.Eq(t, "bar", resp.Meta["foo"])

	readReq.AuthToken = tokenGood.SecretID
	err = c1.ClientRPC("NodeMeta.Read", readReq, &resp)
	must.NoError(t, err)
}

func TestNodeMeta_Validation(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
	})
	defer cleanup()

	applyReq := &structs.NodeMetaApplyRequest{
		NodeID: c1.NodeID(),
		Meta:   map[string]*string{},
	}

	resp := struct{}{}

	// An empty map is an error
	err := c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.ErrorContains(t, err, "missing required Meta")

	// empty keys are prohibited
	applyReq.Meta[""] = pointer.Of("bad")
	err = c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.ErrorContains(t, err, "empty")

	// * is prohibited in keys
	delete(applyReq.Meta, "")
	applyReq.Meta["*"] = pointer.Of("bad")
	err = c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.ErrorContains(t, err, "*")
}

func TestNodeMeta_unset(t *testing.T) {
	ci.Parallel(t)

	s, cleanupS := nomad.TestServer(t, nil)
	defer cleanupS()
	testutil.WaitForLeader(t, s.RPC)

	c1, cleanup := TestClient(t, func(c *config.Config) {
		c.Servers = []string{s.GetConfig().RPCAddr.String()}
		c.Node.Meta["static_meta"] = "true"
	})
	defer cleanup()

	// Set dynamic node metadata.
	applyReq := &structs.NodeMetaApplyRequest{
		NodeID: c1.NodeID(),
		Meta: map[string]*string{
			"dynamic_meta": pointer.Of("true"),
		},
	}
	var resp structs.NodeMetaResponse
	err := c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.NoError(t, err)

	// Check static_meta:
	//   1. must be present in Static.
	//   2. must be present in Meta
	must.Eq(t, resp.Static["static_meta"], "true")
	must.Eq(t, resp.Meta["static_meta"], "true")

	// Check dynamic_meta:
	//   1. must be present in Dynamic.
	//   2. must be present in Meta
	must.Eq(t, *resp.Dynamic["dynamic_meta"], "true")
	must.Eq(t, resp.Meta["dynamic_meta"], "true")

	// Unset static node metada.
	applyReq = &structs.NodeMetaApplyRequest{
		NodeID: c1.NodeID(),
		Meta: map[string]*string{
			"static_meta": nil,
		},
	}
	err = c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.NoError(t, err)

	// Check static_meta:
	//   1. must be present in Static.
	//   2. must not be present in Meta
	//   3. must be nil in Dynamic to persist its removal even on restarts.
	must.Eq(t, resp.Static["static_meta"], "true")
	must.MapNotContainsKey(t, resp.Meta, "static_meta")
	must.Nil(t, resp.Dynamic["static_meta"])

	// Check dynamic_meta:
	//   1. must be present in Dynamic.
	//   2. must be present in Meta
	must.Eq(t, *resp.Dynamic["dynamic_meta"], "true")
	must.Eq(t, resp.Meta["dynamic_meta"], "true")

	// Unset dynamic node metada.
	applyReq = &structs.NodeMetaApplyRequest{
		NodeID: c1.NodeID(),
		Meta: map[string]*string{
			"dynamic_meta": nil,
		},
	}
	err = c1.ClientRPC("NodeMeta.Apply", applyReq, &resp)
	must.NoError(t, err)

	// Check static_meta:
	//   1. must be present in Static.
	//   2. must not be present in Meta
	//   3. must be nil in Dynamic to persist its removal even on restarts.
	must.Eq(t, resp.Static["static_meta"], "true")
	must.MapNotContainsKey(t, resp.Meta, "static_meta")
	must.Nil(t, resp.Dynamic["static_meta"])

	// Check dynamic_meta:
	//   1. must not be present in Dynamic.
	//   2. must not be present in Meta
	must.MapNotContainsKey(t, resp.Dynamic, "dynamic_meta")
	must.MapNotContainsKey(t, resp.Meta, "dynamic_meta")
}

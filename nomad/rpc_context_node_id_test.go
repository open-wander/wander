// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"net"
	"sync"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// One client connection runs its RPCs concurrently, and every node RPC may
// be the first to bind the connection to its node. Run under -race, this
// fails when the binding is not guarded.
func TestRPCContext_NodeIDConcurrentBind(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	node := mock.Node()
	must.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 10, node))

	local, remote := net.Pipe()
	t.Cleanup(func() { _ = local.Close(); _ = remote.Close() })
	ctx := &RPCContext{Conn: local}
	endpoint := NewNodeEndpoint(s, ctx)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			args := &structs.NodeUpdateStatusRequest{
				NodeID:       node.ID,
				Status:       structs.NodeStatusReady,
				WriteRequest: structs.WriteRequest{Region: "global"},
			}
			_ = endpoint.UpdateStatus(args, &structs.NodeUpdateResponse{})
		}()
		go func() {
			defer wg.Done()
			args := &structs.NodeSpecificRequest{
				NodeID:       node.ID,
				SecretID:     node.SecretID,
				QueryOptions: structs.QueryOptions{Region: "global"},
			}
			_ = endpoint.GetClientAllocs(args, &structs.NodeClientAllocsResponse{})
		}()
	}
	wg.Wait()

	must.Eq(t, node.ID, ctx.nodeID())
	conn, ok := s.getNodeConn(node.ID)
	must.True(t, ok)
	must.EqOp(t, ctx, conn.Ctx)
}

func TestRPCContext_BindNodeIDOnce(t *testing.T) {
	ci.Parallel(t)

	var nilCtx *RPCContext
	must.False(t, nilCtx.bindNodeID("node-a"))
	must.Eq(t, "", nilCtx.nodeID())

	ctx := &RPCContext{}
	must.False(t, ctx.bindNodeID(""))
	must.True(t, ctx.bindNodeID("node-a"))
	must.False(t, ctx.bindNodeID("node-b"))
	must.Eq(t, "node-a", ctx.nodeID())
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"testing"
	"time"

	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
)

// TestClient_ACLEnabled_NodeSecretRPCs checks that a client agent, which
// authenticates every RPC with its node secret, still registers, heartbeats,
// fetches its allocations and reports allocation status while ACLs are
// enabled on the servers.
func TestClient_ACLEnabled_NodeSecretRPCs(t *testing.T) {
	ci.Parallel(t)

	s1, _, _, cleanupS1 := testACLServer(t, nil)
	defer cleanupS1()
	testutil.WaitForLeader(t, s1.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = s1
	})
	defer cleanupC1()

	// Registration and heartbeat: the node becomes ready on the server.
	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			node, err := s1.State().NodeByID(nil, c1.NodeID())
			return err == nil && node != nil && node.Status == structs.NodeStatusReady
		}),
		wait.Timeout(10*time.Second),
		wait.Gap(10*time.Millisecond),
	))

	// Allocation fetch and status update: the client runs the allocation and
	// reports it as running.
	job := mock.Job()
	job.Constraints = nil
	job.TaskGroups[0].Constraints = nil
	job.TaskGroups[0].Count = 1
	job.TaskGroups[0].Networks = nil
	task := job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{"run_for": "1m"}
	task.Services = nil
	task.Resources = &structs.Resources{CPU: 50, MemoryMB: 25}

	alloc := mock.Alloc()
	alloc.Job = job
	alloc.JobID = job.ID
	alloc.NodeID = c1.NodeID()

	state := s1.State()
	must.NoError(t, state.UpsertJob(structs.MsgTypeTestSetup, 100, nil, job))
	must.NoError(t, state.UpsertJobSummary(101, mock.JobSummary(job.ID)))
	must.NoError(t, state.UpsertAllocs(structs.MsgTypeTestSetup, 102, []*structs.Allocation{alloc}))

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			out, err := state.AllocByID(nil, alloc.ID)
			return err == nil && out != nil && out.ClientStatus == structs.AllocClientStatusRunning
		}),
		wait.Timeout(15*time.Second),
		wait.Gap(10*time.Millisecond),
	))
}

// TestClient_SelfDrainConfig_ACLEnabled checks that drain on shutdown, which
// the client sends with its node secret, still takes effect while ACLs are
// enabled.
func TestClient_SelfDrainConfig_ACLEnabled(t *testing.T) {
	ci.Parallel(t)

	srv, _, _, cleanupSRV := testACLServer(t, nil)
	defer cleanupSRV()
	testutil.WaitForLeader(t, srv.RPC)

	c1, cleanupC1 := TestClient(t, func(c *config.Config) {
		c.RPCHandler = srv
		c.DevMode = false
		c.Drain = &config.DrainConfig{Deadline: 10 * time.Second}
	})
	defer cleanupC1()

	must.Wait(t, wait.InitialSuccess(
		wait.BoolFunc(func() bool {
			node, err := srv.State().NodeByID(nil, c1.NodeID())
			return err == nil && node != nil
		}),
		wait.Timeout(5*time.Second),
		wait.Gap(10*time.Millisecond),
	))

	testCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- c1.Leave() }()

	select {
	case err := <-errCh:
		must.NoError(t, err)
	case <-testCtx.Done():
		t.Fatal("expected drain complete before deadline")
	}

	node, err := srv.State().NodeByID(nil, c1.NodeID())
	must.NoError(t, err)
	must.NotNil(t, node.LastDrain, must.Sprint("self-drain was not recorded by the server"))
}

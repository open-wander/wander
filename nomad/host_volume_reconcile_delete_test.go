// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ci"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

func (e *dhvEnv) waitGone(id string) {
	e.t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		return e.get("default", id) == nil, errors.New("volume is still stored")
	}, func(err error) { must.NoError(e.t, err) })
}

func (e *dhvEnv) waitCode(id, code string) *structs.DynamicHostVolume {
	e.t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		v := e.get("default", id)
		if v == nil || v.StatusDescription != code {
			return false, errors.New("status description is not the wanted code yet")
		}
		return true, nil
	}, func(err error) { must.NoError(e.t, err) })
	return e.get("default", id)
}

// waitRemovalCalls waits until at least n delete or deregister calls arrived.
func (e *dhvEnv) waitRemovalCalls(n int) {
	e.t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		d, r := e.mock.removalCounts()
		return d+r >= n, errors.New("the client was not called often enough")
	}, func(err error) { must.NoError(e.t, err) })
}

func TestHostVolumeReconcile_DeletingRemoves(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name       string
		origin     structs.DynamicHostVolumeOrigin
		noNode     bool
		wantDelete int
		wantDereg  int
	}{
		{"plugin created", structs.DynamicHostVolumeOriginPluginCreated, false, 1, 0},
		{"registered", structs.DynamicHostVolumeOriginRegistered, false, 0, 1},
		{"no node needs no call", structs.DynamicHostVolumeOriginPluginCreated, true, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDHVEnv(t, false)
			node := e.nodeID
			if tc.noNode {
				node = ""
			}
			e.seedVolumes(dhvSeedVolume("vol-1", "data", node, structs.DynamicHostVolumeStateDeleting, tc.origin))
			e.reconcile(nil)

			e.waitGone("vol-1")
			d, r := e.mock.removalCounts()
			must.Eq(t, tc.wantDelete, d)
			must.Eq(t, tc.wantDereg, r)
			if tc.wantDelete > 0 {
				first, _ := e.mock.firstRemovals()
				must.Eq(t, "vol-1", first.VolumeID)
				must.Eq(t, e.nodeID, first.NodeID)
			}
		})
	}
}

func TestHostVolumeReconcile_DeletingFailingClientStaysDeleting(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name     string
		origin   structs.DynamicHostVolumeOrigin
		setup    func(m *mockClientHostVolume)
		wantCode string
	}{
		{"transport failure", structs.DynamicHostVolumeOriginPluginCreated,
			func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") }, hostVolumeCodeClientUnreachable},
		{"transport failure registered", structs.DynamicHostVolumeOriginRegistered,
			func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") }, hostVolumeCodeClientUnreachable},
		{"known rejection", structs.DynamicHostVolumeOriginPluginCreated,
			func(m *mockClientHostVolume) {
				m.deleteReply = func(*cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
					return cstructs.HostVolumeDeleteResponse{Reason: "nonzero_exit"}
				}
			}, "nonzero_exit"},
		{"unknown rejection text becomes a fixed code", structs.DynamicHostVolumeOriginRegistered,
			func(m *mockClientHostVolume) {
				m.deregReply = func(*cstructs.HostVolumeDeregisterRequest) cstructs.HostVolumeDeregisterResponse {
					return cstructs.HostVolumeDeregisterResponse{Reason: "/synthetic/secret/path"}
				}
			}, hostVolumeReasonOther},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDHVEnv(t, false)
			e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStateDeleting, tc.origin))
			e.mock.set(tc.setup)
			// A bound far shorter than the test proves a failing call is never
			// recorded failed.
			passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 20 * time.Millisecond })

			stored := e.waitCode("vol-1", tc.wantCode)
			must.Eq(t, structs.DynamicHostVolumeStateDeleting, stored.State)
			e.waitRemovalCalls(6)
			waitPasses(t, passes, 5)

			after := e.get("default", "vol-1")
			must.Eq(t, structs.DynamicHostVolumeStateDeleting, after.State)
			must.Eq(t, tc.wantCode, after.StatusDescription)
			// The code is written once: no further write while it is unchanged.
			must.Eq(t, stored.ModifyIndex, after.ModifyIndex)
			must.Eq(t, stored.ModifyTime, after.ModifyTime)

			e.mock.set(func(m *mockClientHostVolume) { m.err, m.deleteReply, m.deregReply = nil, nil, nil })
			e.waitGone("vol-1")
		})
	}
}

func TestHostVolumeReconcile_DeletingUnknownNode(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.seedVolumes(dhvSeedVolume("vol-1", "data", uuid.Generate(), structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated))
	passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 20 * time.Millisecond })

	stored := e.waitCode("vol-1", hostVolumeCodeNodeNotFound)
	waitPasses(t, passes, 10)
	after := e.get("default", "vol-1")
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, after.State)
	must.Eq(t, stored.ModifyIndex, after.ModifyIndex)
}

// deletingRetryEnv seeds one deleting volume and returns a reconciler that is
// not running, so a test drives deletingRetry itself and no loop races it.
func deletingRetryEnv(t *testing.T, set dynamicHostVolumeReconcileSettings) (*dhvEnv, *dynamicHostVolumeReconciler) {
	t.Helper()
	e := newDHVEnv(t, false)
	e.s.stopDynamicHostVolumeReconciler()
	e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated))
	return e, e.s.newDynamicHostVolumeReconciler(set)
}

// The status description is written once per change of the code, however often
// the same failure repeats.
func TestHostVolumeReconcile_DeletingWritesCodeOnlyOnChange(t *testing.T) {
	ci.Parallel(t)
	e, r := deletingRetryEnv(t, defaultDynamicHostVolumeReconcileSettings())
	var writes atomic.Int64
	e.s.dhvCoordinator.setHook(func(point string) {
		if point == "reconcile-outcome-apply" {
			writes.Add(1)
		}
	})
	retry := func(code string) *structs.DynamicHostVolume {
		cur := e.get("default", "vol-1").Copy()
		r.deletingRetry(context.Background(), "default/vol-1", cur, code)
		return e.get("default", "vol-1")
	}

	first := retry(hostVolumeCodeClientUnreachable)
	must.Eq(t, hostVolumeCodeClientUnreachable, first.StatusDescription)
	must.Eq(t, int64(1), writes.Load())
	for range 5 {
		retry(hostVolumeCodeClientUnreachable)
	}
	must.Eq(t, int64(1), writes.Load())
	must.Eq(t, first.ModifyIndex, e.get("default", "vol-1").ModifyIndex)

	// A different code is a change and is written once.
	second := retry(hostVolumeCodeVolumeInUse)
	must.Eq(t, hostVolumeCodeVolumeInUse, second.StatusDescription)
	must.Eq(t, int64(2), writes.Load())
	retry(hostVolumeCodeVolumeInUse)
	must.Eq(t, int64(2), writes.Load())
}

// The delay keeps growing across retries, including the retry that writes the
// code, reaches the cap and never goes past it.
func TestHostVolumeReconcile_DeletingBackoffReachesCap(t *testing.T) {
	ci.Parallel(t)
	set := defaultDynamicHostVolumeReconcileSettings()
	set.BackoffBase, set.BackoffCap = 10*time.Millisecond, 80*time.Millisecond
	e, r := deletingRetryEnv(t, set)
	const key = "default/vol-1"

	for i := 1; i <= 8; i++ {
		before := time.Now()
		cur := e.get("default", "vol-1").Copy()
		r.deletingRetry(context.Background(), key, cur, hostVolumeCodeClientUnreachable)
		after := time.Now()

		b := r.backoff[key]
		must.NotNil(t, b, must.Sprintf("retry %d", i))
		must.Eq(t, i, b.tries, must.Sprintf("retry %d", i))
		// The delay is jittered between half and all of the doubled value, so
		// the bounds below hold however the jitter falls.
		must.LessEq(t, set.BackoffCap, b.next.Sub(after), must.Sprintf("retry %d is past the cap", i))
		if i >= 4 { // 10ms doubled three times is 80ms, the cap
			must.GreaterEq(t, set.BackoffCap/2, b.next.Sub(before), must.Sprintf("retry %d did not reach the cap", i))
		}
	}
}

func TestHostVolumeReconcile_DeletingInUse(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated)
	e.seedVolumes(vol)
	e.use(e.nodeID, "data", structs.AllocClientStatusRunning)
	passes := e.reconcile(nil)

	stored := e.waitCode("vol-1", hostVolumeCodeVolumeInUse)
	waitPasses(t, passes, 8)
	d, r := e.mock.removalCounts()
	must.Eq(t, 0, d+r)
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, e.get("default", "vol-1").State)
	must.Eq(t, stored.ModifyIndex, e.get("default", "vol-1").ModifyIndex)

	// Once the allocation ends the removal goes through.
	snap, err := e.s.State().Snapshot()
	must.NoError(t, err)
	allocs, err := snap.AllocsByNode(nil, e.nodeID)
	must.NoError(t, err)
	must.SliceNotEmpty(t, allocs)
	done := allocs[0].Copy()
	done.ClientStatus = structs.AllocClientStatusComplete
	must.NoError(t, e.s.State().UpdateAllocsFromClient(structs.MsgTypeTestSetup, nextDHVIndex(), []*structs.Allocation{done}))
	e.waitGone("vol-1")
	d, _ = e.mock.removalCounts()
	must.Eq(t, 1, d)
}

func TestHostVolumeReconcile_DeletingInFlight(t *testing.T) {
	ci.Parallel(t)

	t.Run("the reconciler skips a volume the RPC holds", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated))
		must.True(t, e.s.dhvCoordinator.begin("default/vol-1"))
		passes := e.reconcile(nil)
		waitPasses(t, passes, 6)
		d, r := e.mock.removalCounts()
		must.Eq(t, 0, d+r)
		must.NotNil(t, e.get("default", "vol-1"))

		e.s.dhvCoordinator.end("default/vol-1")
		e.waitGone("vol-1")
	})

	t.Run("a call past its deadline stays in flight and its answer is recorded", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated))
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		e.mock.set(func(m *mockClientHostVolume) {
			m.hook = func() {
				once.Do(func() { close(entered) })
				<-release
			}
		})
		passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.CallTimeout = 30 * time.Millisecond })
		<-entered
		waitPasses(t, passes, 6)
		d, _ := e.mock.removalCounts()
		must.Eq(t, 1, d)
		must.True(t, e.s.dhvCoordinator.isInFlight("default/vol-1"))
		must.NotNil(t, e.get("default", "vol-1"))

		close(release)
		e.waitGone("vol-1")
		d, _ = e.mock.removalCounts()
		must.Eq(t, 1, d)
	})
}

// No log line and no stored description carries a host path, a parameter or
// client error text.
func TestHostVolumeReconcile_DeletingLogsNoRecordValues(t *testing.T) {
	ci.Parallel(t)
	logs := &lockedBuffer{}
	s, cleanup := TestServer(t, func(c *Config) {
		c.Logger = hclog.NewInterceptLogger(&hclog.LoggerOptions{Name: "nomad", Output: logs, Level: hclog.Trace})
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)
	e := &dhvEnv{t: t, s: s, mock: &mockClientHostVolume{}, seed: 1}
	c := startMockHostVolumeClient(t, s, e.mock)
	e.nodeID = c.NodeID()
	e.advertisePlugin()

	created := dhvSeedVolume("vol-new", "data", e.nodeID, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated)
	created.HostPath = "/synthetic/secret-path"
	created.Parameters = map[string]string{"token": "hunter2"}
	reg := dhvSeedVolume("vol-reg", "ext", e.nodeID, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginRegistered)
	reg.HostPath = "/synthetic/secret-path-2"
	e.seedVolumes(created, reg)
	e.mock.set(func(m *mockClientHostVolume) {
		m.deleteReply = func(*cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
			return cstructs.HostVolumeDeleteResponse{Reason: "synthetic-client-secret /synthetic/secret-path"}
		}
		m.deregReply = func(*cstructs.HostVolumeDeregisterRequest) cstructs.HostVolumeDeregisterResponse {
			return cstructs.HostVolumeDeregisterResponse{Reason: "synthetic-client-secret"}
		}
	})
	passes := e.reconcile(nil)
	e.waitCode("vol-new", hostVolumeReasonOther)
	e.waitCode("vol-reg", hostVolumeReasonOther)
	waitPasses(t, passes, 5)

	// The transport failure path carries its error text too.
	e.mock.set(func(m *mockClientHostVolume) {
		m.deleteReply, m.deregReply = nil, nil
		m.err = errors.New("synthetic-client-secret")
	})
	e.waitCode("vol-new", hostVolumeCodeClientUnreachable)
	e.waitCode("vol-reg", hostVolumeCodeClientUnreachable)
	waitPasses(t, passes, 5)

	out := logs.String()
	must.StrContains(t, out, "removal is waiting")
	for _, secret := range []string{"secret-path", "hunter2", "synthetic-client-secret", "token"} {
		must.StrNotContains(t, out, secret)
		for _, id := range []string{"vol-new", "vol-reg"} {
			must.StrNotContains(t, e.get("default", id).StatusDescription, secret)
		}
	}
}

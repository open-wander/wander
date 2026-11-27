// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"bytes"
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

// lockedBuffer is a log sink that is safe to read while the server writes.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// reconcile restarts the reconciler with short timings, so a test does not wait
// for the production interval. The first pass runs at once, so volumes are
// seeded before this call. It returns a counter of the passes made.
func (e *dhvEnv) reconcile(mutate func(*dynamicHostVolumeReconcileSettings)) *atomic.Int64 {
	e.t.Helper()
	e.s.stopDynamicHostVolumeReconciler()
	set := dynamicHostVolumeReconcileSettings{
		Interval: 15 * time.Millisecond, BackoffBase: 5 * time.Millisecond, BackoffCap: 20 * time.Millisecond,
		Bound: 10 * time.Minute, CallTimeout: 10 * time.Second,
	}
	if mutate != nil {
		mutate(&set)
	}
	passes := &atomic.Int64{}
	coord := e.s.dhvCoordinator
	coord.setReconcileSettings(set)
	coord.setHook(func(point string) {
		if point == "reconcile-pass" {
			passes.Add(1)
		}
	})
	stopCh := make(chan struct{})
	e.t.Cleanup(func() {
		close(stopCh)
		e.s.stopDynamicHostVolumeReconciler()
	})
	e.s.startDynamicHostVolumeReconciler(stopCh)
	return passes
}

// waitPasses waits for n more passes of the loop.
func waitPasses(t *testing.T, passes *atomic.Int64, n int64) {
	t.Helper()
	want := passes.Load() + n
	testutil.WaitForResult(func() (bool, error) {
		return passes.Load() >= want, errors.New("the reconciler made no further pass")
	}, func(err error) { must.NoError(t, err) })
}

func (e *dhvEnv) waitState(id string, st structs.DynamicHostVolumeState) *structs.DynamicHostVolume {
	e.t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		v := e.get("default", id)
		if v == nil || v.State != st {
			return false, errors.New("volume is not in the wanted state yet")
		}
		return true, nil
	}, func(err error) { must.NoError(e.t, err) })
	return e.get("default", id)
}

func TestHostVolumeReconcile_Pending(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name     string
		origin   structs.DynamicHostVolumeOrigin
		reject   bool
		wantPath string
	}{
		{"create ready", structs.DynamicHostVolumeOriginPluginCreated, false, "/synthetic/vol-1"},
		{"create failed", structs.DynamicHostVolumeOriginPluginCreated, true, ""},
		{"register ready", structs.DynamicHostVolumeOriginRegistered, false, "/synthetic/vol-1"},
		{"register failed", structs.DynamicHostVolumeOriginRegistered, true, "/synthetic/vol-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDHVEnv(t, false)
			seed := dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, tc.origin)
			e.seedVolumes(seed)
			if tc.reject {
				e.mock.set(func(m *mockClientHostVolume) {
					m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
						return cstructs.HostVolumeCreateResponse{Reason: "synthetic_reason"}
					}
					m.registerReply = func(*cstructs.HostVolumeRegisterRequest) cstructs.HostVolumeRegisterResponse {
						return cstructs.HostVolumeRegisterResponse{Reason: "synthetic_reason"}
					}
				})
			}
			e.reconcile(nil)

			if tc.reject {
				got := e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
				must.Eq(t, "synthetic_reason", got.StatusDescription)
			} else {
				got := e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
				must.Eq(t, tc.wantPath, got.HostPath)
			}

			nc, nr := e.mock.counts()
			if tc.origin == structs.DynamicHostVolumeOriginRegistered {
				must.Eq(t, 0, nc)
				must.Eq(t, 1, nr)
				must.Eq(t, "vol-1", e.mock.registry[0].VolumeID)
				must.Eq(t, "/synthetic/vol-1", e.mock.registry[0].HostPath)
			} else {
				must.Eq(t, 1, nc)
				must.Eq(t, 0, nr)
				req := e.mock.creates[0]
				must.Eq(t, "vol-1", req.VolumeID)
				must.Eq(t, e.nodeID, req.NodeID)
				must.Eq(t, seed.PluginID, req.PluginID)
				must.Eq(t, seed.Parameters, req.Parameters)
				must.Eq(t, seed.RequestedCapacityMaxBytes, req.CapacityMaxBytes)
			}
		})
	}
}

// A reply that is lost leaves the volume pending. The retry sends the same
// request, so the client's repeat-safe create can answer it from its record.
func TestHostVolumeReconcile_LostReply(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
	e.reconcile(nil)

	_, _, err := e.create(dhvReqVolume("data"))
	must.Error(t, err)
	rec := e.records()[0]
	must.Eq(t, structs.DynamicHostVolumeStatePending, rec.State)

	testutil.WaitForResult(func() (bool, error) {
		nc, _ := e.mock.counts()
		return nc >= 3, errors.New("the reconciler did not retry")
	}, func(err error) { must.NoError(t, err) })
	must.Eq(t, structs.DynamicHostVolumeStatePending, e.get("default", rec.ID).State)

	e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
	got := e.waitState(rec.ID, structs.DynamicHostVolumeStateReady)
	must.Eq(t, "/synthetic/"+rec.ID, got.HostPath)

	e.mock.mu.Lock()
	defer e.mock.mu.Unlock()
	for _, req := range e.mock.creates {
		must.Eq(t, e.mock.creates[0], req)
	}
	must.Eq(t, rec.ID, e.mock.creates[0].VolumeID)
}

func TestHostVolumeReconcile_Requested(t *testing.T) {
	ci.Parallel(t)

	t.Run("a node is eligible", func(t *testing.T) {
		for _, pool := range []string{"", structs.NodePoolAll} {
			t.Run("pool "+pool, func(t *testing.T) {
				e := newDHVEnv(t, false)
				seed := dhvSeedVolume("vol-1", "data", "", structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeOriginPluginCreated)
				seed.NodePool = pool
				e.seedVolumes(seed)
				e.reconcile(nil)

				got := e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
				must.Eq(t, e.nodeID, got.NodeID)
				node, err := e.s.State().NodeByID(nil, e.nodeID)
				must.NoError(t, err)
				must.Eq(t, node.NodePool, got.NodePool)
				must.Eq(t, "/synthetic/vol-1", got.HostPath)
				nc, _ := e.mock.counts()
				must.Eq(t, 1, nc)
			})
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(e *dhvEnv, v *structs.DynamicHostVolume)
		want   string
	}{
		{"plugin not advertised", func(_ *dhvEnv, v *structs.DynamicHostVolume) { v.PluginID = "absent" },
			"no_eligible_node: plugin_unhealthy=1"},
		{"only a name conflict", func(e *dhvEnv, _ *structs.DynamicHostVolume) {
			e.seedVolumes(dhvSeedVolume("held", "data", e.nodeID, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated))
		}, "name_conflict"},
		{"register origin", func(_ *dhvEnv, v *structs.DynamicHostVolume) {
			v.Origin, v.PluginID = structs.DynamicHostVolumeOriginRegistered, ""
		}, "invalid_requested_origin"},
	} {
		t.Run("no node: "+tc.name, func(t *testing.T) {
			e := newDHVEnv(t, false)
			seed := dhvSeedVolume("vol-1", "data", "", structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeOriginPluginCreated)
			tc.mutate(e, seed)
			e.seedVolumes(seed)
			e.reconcile(nil)

			got := e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
			must.Eq(t, tc.want, got.StatusDescription)
			must.Eq(t, "", got.NodeID)
			nc, _ := e.mock.counts()
			must.Eq(t, 0, nc)
		})
	}
}

func TestHostVolumeReconcile_InFlight(t *testing.T) {
	ci.Parallel(t)

	t.Run("the reconciler skips a volume the RPC has", func(t *testing.T) {
		e := newDHVEnv(t, false)
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		e.mock.set(func(m *mockClientHostVolume) {
			m.hook = func() {
				once.Do(func() { close(entered) })
				<-release
			}
		})
		passes := e.reconcile(nil)

		done := make(chan error, 1)
		go func() {
			_, _, err := e.create(dhvReqVolume("data"))
			done <- err
		}()
		<-entered
		waitPasses(t, passes, 5)
		nc, _ := e.mock.counts()
		must.Eq(t, 1, nc)

		close(release)
		must.NoError(t, <-done)
		nc, _ = e.mock.counts()
		must.Eq(t, 1, nc)
	})

	t.Run("the RPC refuses a volume the reconciler has", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		e.mock.set(func(m *mockClientHostVolume) {
			m.hook = func() {
				once.Do(func() { close(entered) })
				<-release
			}
		})
		e.reconcile(nil)
		<-entered

		index, recs := e.snapshotTable()
		for _, method := range []string{"HostVolume.Create", "HostVolume.Register"} {
			v := dhvReqVolume("data")
			v.ID = "vol-1"
			_, _, err := e.call(method, "", "default", v)
			mustCoded(t, err, 409, "operation in progress")
		}
		e.mustUnchanged(index, recs)
		must.True(t, e.s.dhvCoordinator.isInFlight("default/vol-1"))

		close(release)
		e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
	})
}

// A volume the RPC settled between the scan and the mark is not called again:
// the record is read once more after the mark.
func TestHostVolumeReconcile_StaleScan(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.s.stopDynamicHostVolumeReconciler()
	e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
	scanned := e.get("default", "vol-1").Copy()

	_, _, err := e.s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeOutcomeRequestType, &structs.DynamicHostVolumeOutcomeRequest{
		Key: structs.DynamicHostVolumeKey{Namespace: "default", ID: "vol-1"}, State: structs.DynamicHostVolumeStateReady,
		HostPath: "/synthetic/vol-1", ModifyTime: time.Now().UnixNano(), WriteRequest: structs.WriteRequest{Region: e.s.Region()},
	})
	must.NoError(t, err)

	r := e.s.newDynamicHostVolumeReconciler(defaultDynamicHostVolumeReconcileSettings())
	r.process(t.Context(), scanned)
	nc, nr := e.mock.counts()
	must.Eq(t, 0, nc)
	must.Eq(t, 0, nr)
	must.False(t, e.s.dhvCoordinator.isInFlight("default/vol-1"))
}

func TestHostVolumeReconcile_Backoff(t *testing.T) {
	ci.Parallel(t)

	t.Run("failed with a fixed code after the bound", func(t *testing.T) {
		e := newDHVEnv(t, false)
		seed := dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated)
		seed.ModifyTime = time.Now().UnixNano()
		e.seedVolumes(seed)
		e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure 10.0.0.1") })
		e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 300 * time.Millisecond })

		got := e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
		must.Eq(t, "retry_limit_reached", got.StatusDescription)
		nc, _ := e.mock.counts()
		must.Greater(t, 2, nc, must.Sprint("the volume was retried with backoff before the bound"))
	})

	t.Run("one attempt is made before an old record fails", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
		e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
		e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = time.Second })

		got := e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
		must.Eq(t, "retry_limit_reached", got.StatusDescription)
		nc, _ := e.mock.counts()
		must.Eq(t, 1, nc)
	})

	t.Run("a node without a connection waits for the bound", func(t *testing.T) {
		e := newDHVEnv(t, false)
		away := mockNodeInState(t, e)
		seed := dhvSeedVolume("vol-1", "data", away, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated)
		seed.ModifyTime = time.Now().UnixNano()
		e.seedVolumes(seed)
		passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 400 * time.Millisecond })

		waitPasses(t, passes, 3)
		must.Eq(t, structs.DynamicHostVolumeStatePending, e.get("default", "vol-1").State)
		got := e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
		must.Eq(t, "retry_limit_reached", got.StatusDescription)
		nc, _ := e.mock.counts()
		must.Eq(t, 0, nc)
	})

	t.Run("an unknown node fails at once", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.seedVolumes(dhvSeedVolume("vol-1", "data", uuid.Generate(), structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
		e.reconcile(nil)

		got := e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
		must.Eq(t, "node_not_found", got.StatusDescription)
	})

	// A call past its deadline may still create the volume on the node, so the
	// volume is never failed while the call runs, even past the bound, and the
	// answer of the call is recorded when it returns.
	t.Run("a client call past its deadline records its own answer", func(t *testing.T) {
		const timeout = 50 * time.Millisecond
		e := newDHVEnv(t, false)
		// The seeded modify time is far past the bound.
		e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		e.mock.set(func(m *mockClientHostVolume) {
			m.hook = func() {
				once.Do(func() { close(entered) })
				<-release
			}
		})
		passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) {
			s.CallTimeout, s.Bound = timeout, 30*time.Millisecond
		})

		<-entered
		time.Sleep(4 * timeout)
		waitPasses(t, passes, 3)
		got := e.get("default", "vol-1")
		must.Eq(t, structs.DynamicHostVolumeStatePending, got.State, must.Sprintf("diagnostic %q", got.StatusDescription))
		must.True(t, e.s.dhvCoordinator.isInFlight("default/vol-1"))

		close(release)
		got = e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
		must.Eq(t, "/synthetic/vol-1", got.HostPath)
		testutil.WaitForResult(func() (bool, error) {
			return !e.s.dhvCoordinator.isInFlight("default/vol-1"), errors.New("the volume is still in flight")
		}, func(err error) { must.NoError(t, err) })
		nc, _ := e.mock.counts()
		must.Eq(t, 1, nc)
	})

	t.Run("a client call past its deadline that fails transiently is retried", func(t *testing.T) {
		e := newDHVEnv(t, false)
		seed := dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated)
		seed.ModifyTime = time.Now().UnixNano()
		e.seedVolumes(seed)
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		e.mock.set(func(m *mockClientHostVolume) {
			m.err = errors.New("synthetic transport failure")
			m.hook = func() {
				once.Do(func() {
					close(entered)
					<-release
				})
			}
		})
		passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.CallTimeout = 50 * time.Millisecond })

		<-entered
		time.Sleep(200 * time.Millisecond)
		waitPasses(t, passes, 3)
		nc, _ := e.mock.counts()
		must.Eq(t, 1, nc)

		close(release)
		testutil.WaitForResult(func() (bool, error) {
			nc, _ := e.mock.counts()
			return nc >= 3, errors.New("the reconciler did not retry after the abandoned call returned")
		}, func(err error) { must.NoError(t, err) })
		must.Eq(t, structs.DynamicHostVolumeStatePending, e.get("default", "vol-1").State)
		e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
		e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
	})
}

// mockNodeInState adds a node that has no connection to any server.
func mockNodeInState(t *testing.T, e *dhvEnv) string {
	t.Helper()
	n := newDHVTestNode()
	must.NoError(t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, 3, n))
	return n.ID
}

func newDHVTestNode() *structs.Node {
	n := &structs.Node{
		ID: uuid.Generate(), SecretID: uuid.Generate(), Datacenter: "dc1", Name: "away", NodePool: "default",
		Status: structs.NodeStatusReady, SchedulingEligibility: structs.NodeSchedulingEligible,
		Attributes: map[string]string{},
	}
	n.ComputeClass()
	return n
}

// An outcome the state refuses, because the volume moved on during the call, is
// dropped and the volume is not called again.
func TestHostVolumeReconcile_OutcomeRefused(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
	var once sync.Once
	e.mock.set(func(m *mockClientHostVolume) {
		// The leader now drives a deleting volume to removal; a rejecting client
		// keeps the volume deleting so the test can still read it.
		m.deleteReply = func(*cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
			return cstructs.HostVolumeDeleteResponse{Reason: "nonzero_exit"}
		}
		m.hook = func() {
			once.Do(func() {
				_, _, err := e.s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeMarkDeletingRequestType,
					&structs.DynamicHostVolumeMarkDeletingRequest{
						Keys:         []structs.DynamicHostVolumeKey{{Namespace: "default", ID: "vol-1"}},
						ModifyTime:   time.Now().UnixNano(),
						WriteRequest: structs.WriteRequest{Region: e.s.Region()},
					})
				must.NoError(t, err)
			})
		}
	})
	passes := e.reconcile(nil)

	waitPasses(t, passes, 6)
	got := e.get("default", "vol-1")
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, got.State)
	must.Eq(t, "", got.HostPath)
	nc, _ := e.mock.counts()
	must.Eq(t, 1, nc)
}

// The version gate makes the reconciler wait: the volume is not failed, even
// when it is far past the bound, and it becomes ready once the gate lifts.
func TestHostVolumeReconcile_VersionGate(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
	current := e.s.serf.LocalMember().Tags[wanderVersionTag]
	advertiseWanderVersion(t, e.s, "0.1.1")
	passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 20 * time.Millisecond })

	testutil.WaitForResult(func() (bool, error) {
		nc, _ := e.mock.counts()
		return nc >= 2, errors.New("the reconciler did not try again while the gate was closed")
	}, func(err error) { must.NoError(t, err) })
	waitPasses(t, passes, 3)
	must.Eq(t, structs.DynamicHostVolumeStatePending, e.get("default", "vol-1").State)

	advertiseWanderVersion(t, e.s, current)
	got := e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
	must.Eq(t, "/synthetic/vol-1", got.HostPath)
}

// Time refused by the version gate does not count toward the bound: a volume
// the gate held for longer than the bound is not failed by the first transient
// error after the gate lifts.
func TestHostVolumeReconcile_VersionGateNotCounted(t *testing.T) {
	ci.Parallel(t)
	const bound = time.Second
	e := newDHVEnv(t, false)
	seed := dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated)
	seed.ModifyTime = time.Now().UnixNano()
	e.seedVolumes(seed)
	current := e.s.serf.LocalMember().Tags[wanderVersionTag]
	advertiseWanderVersion(t, e.s, "0.1.1")
	e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = bound })

	// The client answers, and the gate refuses the outcome until the record
	// is older than the bound.
	time.Sleep(bound + 200*time.Millisecond)
	nc, _ := e.mock.counts()
	must.Greater(t, 1, nc, must.Sprint("the reconciler did not try while the gate was closed"))
	must.Eq(t, structs.DynamicHostVolumeStatePending, e.get("default", "vol-1").State)

	e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
	advertiseWanderVersion(t, e.s, current)
	base, _ := e.mock.counts()
	testutil.WaitForResult(func() (bool, error) {
		nc, _ := e.mock.counts()
		if e.get("default", "vol-1").State != structs.DynamicHostVolumeStatePending {
			return true, nil
		}
		return nc >= base+3, errors.New("the reconciler did not retry after the gate lifted")
	}, func(err error) { must.NoError(t, err) })
	got := e.get("default", "vol-1")
	must.Eq(t, structs.DynamicHostVolumeStatePending, got.State, must.Sprintf("diagnostic %q", got.StatusDescription))

	e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
	got = e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
	must.Eq(t, "/synthetic/vol-1", got.HostPath)
}

// The loop is started by establishLeadership and stopped by revokeLeadership,
// including on the reassert path, where both run with one unclosed stop channel.
func TestHostVolumeReconcile_Leadership(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	run := &e.s.dhvCoordinator.reconcileRun
	doneOf := func() chan struct{} {
		run.mu.Lock()
		defer run.mu.Unlock()
		return run.done
	}

	testutil.WaitForResult(func() (bool, error) {
		return run.running.Load() == 1, errors.New("the loop did not start with the leadership")
	}, func(err error) { must.NoError(t, err) })
	first := doneOf()
	must.NotNil(t, first)

	must.NoError(t, e.s.revokeLeadership())
	select {
	case <-first:
	default:
		t.Fatal("the loop was still running after revokeLeadership")
	}
	must.Eq(t, int32(0), run.running.Load())

	// A pending volume is picked up after the leadership is established again.
	e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
	set := defaultDynamicHostVolumeReconcileSettings()
	set.Interval = 15 * time.Millisecond
	e.s.dhvCoordinator.setReconcileSettings(set)
	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	must.NoError(t, e.s.establishLeadership(stopCh))
	must.Eq(t, int32(1), run.running.Load())
	e.waitState("vol-1", structs.DynamicHostVolumeStateReady)

	// Repeated flaps, and a start while a loop runs, never leave two loops. The
	// inherited drainer races with itself when establishLeadership repeats in
	// quick succession, so these use the start and stop calls directly.
	for range 3 {
		prev := doneOf()
		e.s.stopDynamicHostVolumeReconciler()
		<-prev
		must.Eq(t, int32(0), run.running.Load())
		e.s.startDynamicHostVolumeReconciler(stopCh)
		must.Eq(t, int32(1), run.running.Load())
		e.s.startDynamicHostVolumeReconciler(stopCh)
		must.Eq(t, int32(1), run.running.Load())
	}
	must.NoError(t, e.s.revokeLeadership())
	must.NoError(t, e.s.revokeLeadership())
	must.Eq(t, int32(0), run.running.Load())
	select {
	case <-doneOf():
		t.Fatal("a stopped loop must leave no handle")
	default:
	}
}

// The loop leaves the volume in flight while a call is outstanding at stop,
// and writes nothing for it.
func TestHostVolumeReconcile_StopAbandonsCall(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.seedVolumes(dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated))
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() {
			once.Do(func() { close(entered) })
			<-release
		}
	})
	e.reconcile(nil)
	<-entered

	stopped := make(chan struct{})
	go func() {
		e.s.stopDynamicHostVolumeReconciler()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the stop waited for the client call")
	}
	must.True(t, e.s.dhvCoordinator.isInFlight("default/vol-1"))

	index := e.tableIndex()
	close(release)
	testutil.WaitForResult(func() (bool, error) {
		return !e.s.dhvCoordinator.isInFlight("default/vol-1"), errors.New("the abandoned call is still running")
	}, func(err error) { must.NoError(t, err) })
	must.Eq(t, index, e.tableIndex())
	must.Eq(t, structs.DynamicHostVolumeStatePending, e.get("default", "vol-1").State)
}

// No log line carries a host path, a parameter or client error text.
func TestHostVolumeReconcile_LogsNoRecordValues(t *testing.T) {
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

	reg := dhvSeedVolume("vol-reg", "ext", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginRegistered)
	reg.HostPath = "/synthetic/secret-path"
	reg.ModifyTime = time.Now().UnixNano()
	pend := dhvSeedVolume("vol-new", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated)
	pend.Parameters = map[string]string{"token": "hunter2"}
	pend.ModifyTime = time.Now().UnixNano()
	e.seedVolumes(reg, pend)
	e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic-client-secret") })
	e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 200 * time.Millisecond })

	e.waitState("vol-reg", structs.DynamicHostVolumeStateFailed)
	e.waitState("vol-new", structs.DynamicHostVolumeStateFailed)
	out := logs.String()
	must.StrContains(t, out, "retry limit reached")
	for _, secret := range []string{"secret-path", "hunter2", "synthetic-client-secret", "token"} {
		must.StrNotContains(t, out, secret)
	}
}

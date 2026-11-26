// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

const dhvTestPlugin = "plug"

// dhvEnv is a server with one client node that advertises the test plugin and
// answers the host volume handshake through the mock.
type dhvEnv struct {
	t      *testing.T
	s      *Server
	mock   *mockClientHostVolume
	nodeID string
	root   string // management secret, empty without ACLs
	seed   uint64
}

// rpc sends one request on its own connection: a connection serves one
// request at a time, and several tests need calls in flight together.
func (e *dhvEnv) rpc(method string, args, reply any) error {
	codec := rpcClient(e.t, e.s)
	defer codec.Close()
	return msgpackrpc.CallWithCodec(codec, method, args, reply)
}

// with returns the environment bound to the test that is running, so an
// assertion inside a subtest fails that subtest and not its parent.
func (e *dhvEnv) with(t *testing.T) *dhvEnv {
	c := *e
	c.t = t
	return &c
}

func newDHVEnv(t *testing.T, withACL bool) *dhvEnv {
	t.Helper()
	e := &dhvEnv{t: t, seed: 1}
	var cleanup func()
	if withACL {
		var tok *structs.ACLToken
		e.s, tok, cleanup = TestACLServer(t, nil)
		e.root = tok.SecretID
	} else {
		e.s, cleanup = TestServer(t, nil)
	}
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, e.s.RPC)
	e.mock = &mockClientHostVolume{}
	c := startMockHostVolumeClient(t, e.s, e.mock)
	e.nodeID = c.NodeID()
	e.advertisePlugin()
	return e
}

// advertisePlugin waits for the client node to be ready and then adds the
// plugin attribute to its stored record. The client's own fingerprinter owns
// that attribute and drops it, so the test sets it where selection reads it.
func (e *dhvEnv) advertisePlugin() {
	e.t.Helper()
	waitForNodeInState(e.t, e.s, e.nodeID)
	testutil.WaitForResult(func() (bool, error) {
		n, err := e.s.State().NodeByID(nil, e.nodeID)
		if err != nil {
			return false, err
		}
		if n.Status != structs.NodeStatusReady || n.SchedulingEligibility != structs.NodeSchedulingEligible {
			return false, fmt.Errorf("node is %s/%s", n.Status, n.SchedulingEligibility)
		}
		return true, nil
	}, func(err error) { must.NoError(e.t, err) })
	n, err := e.s.State().NodeByID(nil, e.nodeID)
	must.NoError(e.t, err)
	n = n.Copy()
	n.Attributes[dynamicHostVolumePluginAttributePrefix+dhvTestPlugin] = "1"
	must.NoError(e.t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, e.s.raft.LastIndex()+1, n))
}

// seedVolumes writes fixtures straight into the state store. The table index
// must only move forward, so the fixtures take the low Raft indexes and every
// seeding call comes before the first volume write through Raft.
func (e *dhvEnv) seedVolumes(vols ...*structs.DynamicHostVolume) {
	e.t.Helper()
	for _, v := range vols {
		e.seed++
		must.Less(e.t, e.s.raft.LastIndex(), e.seed, must.Sprint("too many seeds for the raft index"))
		must.NoError(e.t, e.s.State().UpsertDynamicHostVolumes(e.seed, []*structs.DynamicHostVolume{v}))
	}
}

// seedVolumesAt writes fixtures at indexes well above the Raft log, for tests
// that never write the table through Raft afterwards.
func (e *dhvEnv) seedVolumesAt(base uint64, vols ...*structs.DynamicHostVolume) {
	e.t.Helper()
	for i, v := range vols {
		must.NoError(e.t, e.s.State().UpsertDynamicHostVolumes(base+uint64(i), []*structs.DynamicHostVolume{v}))
	}
}

func dhvSeedVolume(id, name, node string, st structs.DynamicHostVolumeState, origin structs.DynamicHostVolumeOrigin) *structs.DynamicHostVolume {
	v := dhvReqVolume(name)
	v.ID, v.Namespace, v.Type, v.NodeID, v.NodePool = id, "default", structs.VolumeTypeHost, node, "default"
	v.State, v.Origin = st, origin
	v.CreateTime, v.ModifyTime = 1, 1
	if st == structs.DynamicHostVolumeStateReady || st == structs.DynamicHostVolumeStateUnavailable {
		v.HostPath, v.CapacityBytes = "/synthetic/"+id, 10
	}
	if origin == structs.DynamicHostVolumeOriginRegistered {
		v.PluginID = ""
		v.HostPath = "/synthetic/" + id
	}
	return v
}

// dhvReqVolume is a request body: only the fields a caller may set.
func dhvReqVolume(name string) *structs.DynamicHostVolume {
	return &structs.DynamicHostVolume{
		Name: name, PluginID: dhvTestPlugin,
		RequestedCapacityMinBytes: 1, RequestedCapacityMaxBytes: 100,
		RequestedCapabilities: []*structs.DynamicHostVolumeCapability{{
			AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.DynamicHostVolumeAccessModeSingleNodeWriter,
		}},
		Parameters: map[string]string{"k": "v"},
	}
}

func dhvRegVolume(name, node, path string) *structs.DynamicHostVolume {
	v := dhvReqVolume(name)
	v.PluginID, v.NodeID, v.HostPath = "", node, path
	return v
}

// call sends HostVolume.Create or HostVolume.Register. An empty secret means
// the management token when ACLs are on.
func (e *dhvEnv) call(method, secret, ns string, vol *structs.DynamicHostVolume) (*structs.DynamicHostVolume, uint64, error) {
	e.t.Helper()
	if secret == "" {
		secret = e.root
	}
	wr := structs.WriteRequest{Region: "global", Namespace: ns, AuthToken: secret}
	switch method {
	case "HostVolume.Create":
		var resp structs.HostVolumeCreateResponse
		err := e.rpc(method, &structs.HostVolumeCreateRequest{Volume: vol, WriteRequest: wr}, &resp)
		return resp.Volume, resp.Index, err
	case "HostVolume.Register":
		var resp structs.HostVolumeRegisterResponse
		err := e.rpc(method, &structs.HostVolumeRegisterRequest{Volume: vol, WriteRequest: wr}, &resp)
		return resp.Volume, resp.Index, err
	}
	e.t.Fatalf("unknown method %s", method)
	return nil, 0, nil
}

func (e *dhvEnv) create(vol *structs.DynamicHostVolume) (*structs.DynamicHostVolume, uint64, error) {
	return e.call("HostVolume.Create", "", "default", vol)
}

func (e *dhvEnv) register(vol *structs.DynamicHostVolume) (*structs.DynamicHostVolume, uint64, error) {
	return e.call("HostVolume.Register", "", "default", vol)
}

func (e *dhvEnv) tableIndex() uint64 {
	e.t.Helper()
	idx, err := e.s.State().Index(state.TableDynamicHostVolumes)
	must.NoError(e.t, err)
	return idx
}

func (e *dhvEnv) records() []*structs.DynamicHostVolume {
	e.t.Helper()
	iter, err := e.s.State().DynamicHostVolumes(nil)
	must.NoError(e.t, err)
	var out []*structs.DynamicHostVolume
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		out = append(out, raw.(*structs.DynamicHostVolume))
	}
	return out
}

func (e *dhvEnv) get(ns, id string) *structs.DynamicHostVolume {
	e.t.Helper()
	v, err := e.s.State().DynamicHostVolumeByID(nil, ns, id)
	must.NoError(e.t, err)
	return v
}

// snapshotTable is the whole table and its index, to prove that a refused
// call changed nothing.
func (e *dhvEnv) snapshotTable() (uint64, []*structs.DynamicHostVolume) {
	var copies []*structs.DynamicHostVolume
	for _, v := range e.records() {
		copies = append(copies, v.Copy())
	}
	return e.tableIndex(), copies
}

func (e *dhvEnv) mustUnchanged(index uint64, recs []*structs.DynamicHostVolume) {
	e.t.Helper()
	must.Eq(e.t, index, e.tableIndex())
	got := e.records()
	must.Len(e.t, len(recs), got)
	for _, want := range recs {
		must.True(e.t, want.Equal(e.get(want.Namespace, want.ID)))
	}
}

func mustCoded(t *testing.T, err error, code int, msg string) {
	t.Helper()
	must.Error(t, err)
	c, m, ok := structs.CodeFromRPCCodedErr(err)
	must.True(t, ok, must.Sprintf("not a coded error: %v", err))
	must.Eq(t, code, c)
	must.Eq(t, msg, m)
}

// usable accepts the states a freshly created volume may be read back in. The
// derivation from node updates can move a ready volume to unavailable.
func usable(st structs.DynamicHostVolumeState) bool {
	return st == structs.DynamicHostVolumeStateReady || st == structs.DynamicHostVolumeStateUnavailable
}

func TestHostVolumeCreate_New(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)

	// The client call must run with no lock held and with the volume marked
	// in flight.
	var lockFree, inFlight atomic.Bool
	var seenState atomic.Value
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() {
			if e.s.dhvCoordinator.placeMu.TryLock() {
				lockFree.Store(true)
				e.s.dhvCoordinator.placeMu.Unlock()
			}
			e.mock.mu.Lock()
			id := e.mock.creates[0].VolumeID
			e.mock.mu.Unlock()
			inFlight.Store(e.s.dhvCoordinator.isInFlight("default/" + id))
			if v := e.get("default", id); v != nil {
				seenState.Store(v.State)
			}
		}
	})

	req := dhvReqVolume("data")
	vol, index, err := e.create(req)
	must.NoError(t, err)
	must.NotNil(t, vol)

	must.True(t, lockFree.Load(), must.Sprint("placement lock held across the client call"))
	must.True(t, inFlight.Load())
	must.Eq(t, structs.DynamicHostVolumeStatePending, seenState.Load().(structs.DynamicHostVolumeState))

	must.Eq(t, 36, len(vol.ID))
	must.StrContains(t, vol.ID, "-")
	must.True(t, usable(vol.State))
	must.Eq(t, e.nodeID, vol.NodeID)
	must.Eq(t, "default", vol.NodePool)
	must.Eq(t, "default", vol.Namespace)
	must.Eq(t, structs.VolumeTypeHost, vol.Type)
	must.Eq(t, structs.DynamicHostVolumeOriginPluginCreated, vol.Origin)
	must.Eq(t, "/synthetic/"+vol.ID, vol.HostPath)
	must.Eq(t, int64(4096), vol.CapacityBytes)
	must.Eq(t, "", vol.StatusDescription)
	must.Positive(t, vol.CreateTime)

	// The reply is the stored record and the index of the last apply, which
	// is the second write (pending upsert, then the outcome).
	stored := e.get("default", vol.ID)
	must.True(t, stored.Equal(vol))
	must.Eq(t, stored.ModifyIndex, index)
	must.Eq(t, e.tableIndex(), index)
	must.Greater(t, stored.CreateIndex, stored.ModifyIndex)

	nc, nr := e.mock.counts()
	must.Eq(t, 1, nc)
	must.Eq(t, 0, nr)
	got := e.mock.creates[0]
	must.Eq(t, cstructs.HostVolumeCreateRequest{
		VolumeID: vol.ID, Namespace: "default", Name: "data", NodeID: e.nodeID, NodePool: "default",
		PluginID: dhvTestPlugin, CapacityMinBytes: 1, CapacityMaxBytes: 100, Parameters: map[string]string{"k": "v"},
	}, got)
}

func TestHostVolumeCreate_NoEligibleNode(t *testing.T) {
	ci.Parallel(t)
	t.Run("plugin is not advertised", func(t *testing.T) {
		e := newDHVEnv(t, false)
		// The client node has the test plugin, so ask for another one.
		req := dhvReqVolume("data")
		req.PluginID = "absent"
		vol, index, err := e.create(req)
		must.NoError(t, err)
		must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
		must.Eq(t, "no_eligible_node: plugin_unhealthy=1", vol.StatusDescription)
		must.Eq(t, "", vol.NodeID)
		// requested, then failed: two applies.
		stored := e.get("default", vol.ID)
		must.True(t, stored.Equal(vol))
		must.Eq(t, stored.CreateIndex+1, stored.ModifyIndex)
		must.Eq(t, stored.ModifyIndex, index)
		nc, _ := e.mock.counts()
		must.Eq(t, 0, nc)
	})

	t.Run("no node at all", func(t *testing.T) {
		s, cleanup := TestServer(t, nil)
		t.Cleanup(cleanup)
		testutil.WaitForLeader(t, s.RPC)
		e := &dhvEnv{t: t, s: s, mock: &mockClientHostVolume{}}
		vol, _, err := e.create(dhvReqVolume("data"))
		must.NoError(t, err)
		must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
		must.Eq(t, "no_eligible_node", vol.StatusDescription)
	})

	t.Run("mixed reasons carry codes and counts only", func(t *testing.T) {
		e := newDHVEnv(t, false)
		down := mock.Node()
		down.Status = structs.NodeStatusDown
		down.NodePool = "default"
		inel := mock.Node()
		inel.SchedulingEligibility = structs.NodeSchedulingIneligible
		must.NoError(t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, 5, down))
		must.NoError(t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, 6, inel))
		req := dhvReqVolume("data")
		req.PluginID = "absent"
		req.Constraints = []*structs.Constraint{{LTarget: "${attr.secret-token}", Operand: "=", RTarget: "hunter2"}}
		vol, _, err := e.create(req)
		must.NoError(t, err)
		must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
		must.Eq(t, "no_eligible_node: node_ineligible=1,node_not_ready=1,plugin_unhealthy=1", vol.StatusDescription)
		for _, secret := range []string{"hunter2", "secret-token", down.ID, inel.ID, e.nodeID} {
			must.StrNotContains(t, vol.StatusDescription, secret)
		}
	})
}

func TestHostVolumeCreate_NodeRejections(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	other := mock.Node()
	other.NodePool = "other-pool"
	must.NoError(t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, 5, other))

	cases := []struct {
		name string
		node string
		pool string
		msg  string
	}{
		{"missing node", uuid.Generate(), "", "node_not_found"},
		{"node in another pool", other.ID, "default", "node_pool_mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := e.with(t)
			index, recs := e.snapshotTable()
			req := dhvReqVolume("data")
			req.NodeID, req.NodePool = tc.node, tc.pool
			_, _, err := e.create(req)
			mustCoded(t, err, 400, tc.msg)
			e.mustUnchanged(index, recs)
			nc, _ := e.mock.counts()
			must.Eq(t, 0, nc)
		})
	}
}

func TestHostVolumeCreate_Validation(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)

	cases := []struct {
		name   string
		mutate func(v *structs.DynamicHostVolume)
		msg    string
	}{
		{"no plugin", func(v *structs.DynamicHostVolume) { v.PluginID = "" }, "invalid volume: PluginID: required"},
		{"no name", func(v *structs.DynamicHostVolume) { v.Name = "" }, "invalid volume: Name: required"},
		{"wrong type", func(v *structs.DynamicHostVolume) { v.Type = "csi" }, "invalid volume: Type: unsupported"},
		{"bad capability and constraint carry no values", func(v *structs.DynamicHostVolume) {
			v.RequestedCapabilities[0].AccessMode = "secret-mode-value"
			v.Constraints = []*structs.Constraint{{LTarget: "secret-ltarget", Operand: "bogus-operand", RTarget: "secret-rtarget"}}
		}, "invalid volume: Constraints: invalid, RequestedCapabilities: unsupported"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := e.with(t)
			index, recs := e.snapshotTable()
			req := dhvReqVolume("data")
			tc.mutate(req)
			_, _, err := e.create(req)
			mustCoded(t, err, 400, tc.msg)
			e.mustUnchanged(index, recs)
		})
	}

	t.Run("nil volume", func(t *testing.T) {
		e := e.with(t)
		_, _, err := e.create(nil)
		mustCoded(t, err, 400, "missing volume")
	})

	t.Run("register needs node and path", func(t *testing.T) {
		e := e.with(t)
		_, _, err := e.register(dhvReqVolume("data"))
		mustCoded(t, err, 400, "invalid volume: HostPath: required, NodeID: required")
	})

	t.Run("register path must be absolute", func(t *testing.T) {
		e := e.with(t)
		_, _, err := e.register(dhvRegVolume("data", e.nodeID, "relative/path"))
		mustCoded(t, err, 400, "invalid volume: HostPath: invalid")
	})
}

func TestHostVolumeCreate_Namespace(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, true)

	t.Run("body namespace differs from the request namespace", func(t *testing.T) {
		e := e.with(t)
		index, recs := e.snapshotTable()
		for _, method := range []string{"HostVolume.Create", "HostVolume.Register"} {
			v := dhvReqVolume("data")
			if method == "HostVolume.Register" {
				v = dhvRegVolume("data", e.nodeID, "/synthetic/x")
			}
			v.Namespace = "other"
			_, _, err := e.call(method, "", "default", v)
			mustCoded(t, err, 400, "volume namespace does not match the request namespace")
		}
		e.mustUnchanged(index, recs)
	})

	t.Run("body namespace equal to the request namespace is accepted", func(t *testing.T) {
		e := e.with(t)
		v := dhvReqVolume("same-ns")
		v.Namespace = "default"
		vol, _, err := e.create(v)
		must.NoError(t, err)
		must.Eq(t, "default", vol.Namespace)
	})

	t.Run("namespace that does not exist", func(t *testing.T) {
		e := e.with(t)
		index, recs := e.snapshotTable()
		_, _, err := e.call("HostVolume.Create", "", "ghost", dhvReqVolume("data"))
		mustCoded(t, err, 400, "namespace not found")
		e.mustUnchanged(index, recs)
	})

	t.Run("wildcard namespace", func(t *testing.T) {
		e := e.with(t)
		index, recs := e.snapshotTable()
		_, _, err := e.call("HostVolume.Create", "", structs.AllNamespacesSentinel, dhvReqVolume("data"))
		mustCoded(t, err, 400, "a volume write needs a single namespace")
		_, _, err = e.call("HostVolume.Register", "", structs.AllNamespacesSentinel, dhvRegVolume("data", e.nodeID, "/synthetic/x"))
		mustCoded(t, err, 400, "a volume write needs a single namespace")
		e.mustUnchanged(index, recs)
	})
}

// A request for a volume name the node already uses is refused, and many
// concurrent requests for one name leave exactly one record.
func TestHostVolumeCreate_NameConflict(t *testing.T) {
	ci.Parallel(t)

	t.Run("a pending volume holds the name while the client call runs", func(t *testing.T) {
		e := newDHVEnv(t, false)
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		e.mock.set(func(m *mockClientHostVolume) {
			m.hook = func() {
				once.Do(func() { close(entered) })
				<-release
			}
		})
		done := make(chan error, 1)
		go func() {
			_, _, err := e.create(dhvReqVolume("data"))
			done <- err
		}()
		<-entered

		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 400, "name_conflict")
		close(release)
		must.NoError(t, <-done)
		must.Len(t, 1, e.records())
		nc, _ := e.mock.counts()
		must.Eq(t, 1, nc)
	})

	t.Run("concurrent creates of one name", func(t *testing.T) {
		e := newDHVEnv(t, false)
		const n = 8
		start := make(chan struct{})
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, _, errs[i] = e.create(dhvReqVolume("data"))
			}()
		}
		close(start)
		wg.Wait()

		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			mustCoded(t, err, 400, "name_conflict")
		}
		must.Eq(t, 1, ok)
		must.Len(t, 1, e.records())
		nc, _ := e.mock.counts()
		must.Eq(t, 1, nc)
	})

	t.Run("concurrent registers of one name", func(t *testing.T) {
		e := newDHVEnv(t, false)
		const n = 8
		start := make(chan struct{})
		errs := make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, _, errs[i] = e.register(dhvRegVolume("data", e.nodeID, fmt.Sprintf("/synthetic/p%d", i)))
			}()
		}
		close(start)
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
				continue
			}
			mustCoded(t, err, 400, "name_conflict")
		}
		must.Eq(t, 1, ok)
		must.Len(t, 1, e.records())
	})

	t.Run("a failed or deleting record still holds the name", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.seedVolumes(
			dhvSeedVolume("seed-failed", "data", e.nodeID, structs.DynamicHostVolumeStateFailed, structs.DynamicHostVolumeOriginPluginCreated),
		)
		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 400, "name_conflict")
		must.Len(t, 1, e.records())
	})
}

func TestHostVolumeCreate_ClientOutcomes(t *testing.T) {
	ci.Parallel(t)

	t.Run("a rejection records failed with reason and message", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) {
			m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
				return cstructs.HostVolumeCreateResponse{Reason: "plugin_failed", Message: "synthetic message"}
			}
		})
		vol, index, err := e.create(dhvReqVolume("data"))
		must.NoError(t, err)
		must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
		must.Eq(t, "plugin_failed: synthetic message", vol.StatusDescription)
		must.Eq(t, e.nodeID, vol.NodeID)
		must.Eq(t, "", vol.HostPath)
		must.Eq(t, e.get("default", vol.ID).ModifyIndex, index)
	})

	t.Run("a rejection without a message records the reason", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) {
			m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
				return cstructs.HostVolumeCreateResponse{Reason: "plugin_failed"}
			}
		})
		vol, _, err := e.create(dhvReqVolume("data"))
		must.NoError(t, err)
		must.Eq(t, "plugin_failed", vol.StatusDescription)
	})

	t.Run("unavailable is a stored state, not a rejection", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) {
			m.createReply = func(r *cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
				return cstructs.HostVolumeCreateResponse{VolumeID: r.VolumeID, HostPath: "/synthetic/u", Status: "unavailable"}
			}
		})
		vol, _, err := e.create(dhvReqVolume("data"))
		must.NoError(t, err)
		must.True(t, usable(vol.State))
		must.Eq(t, "/synthetic/u", vol.HostPath)
	})

	t.Run("a transport error leaves the record pending", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport secret-detail") })
		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 502, `client call failed, volume state is "pending"`)
		recs := e.records()
		must.Len(t, 1, recs)
		must.Eq(t, structs.DynamicHostVolumeStatePending, recs[0].State)
		must.Eq(t, "", recs[0].StatusDescription)
		must.Eq(t, e.nodeID, recs[0].NodeID)
		must.False(t, e.s.dhvCoordinator.isInFlight("default/"+recs[0].ID))

		// A repeat by ID names the stored state: the in-flight mark is gone.
		again := dhvReqVolume("data")
		again.ID = recs[0].ID
		_, _, err = e.create(again)
		mustCoded(t, err, 409, `volume state is "pending"`)
	})

	t.Run("a reply with neither reason nor status leaves the record pending", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) {
			m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
				return cstructs.HostVolumeCreateResponse{}
			}
		})
		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 502, `client call failed, volume state is "pending"`)
		must.Eq(t, structs.DynamicHostVolumeStatePending, e.records()[0].State)
	})

	t.Run("a ready reply with a relative path is not recorded", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) {
			m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
				return cstructs.HostVolumeCreateResponse{HostPath: "relative", Status: "ready"}
			}
		})
		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 500, `outcome not recorded, volume state is "pending"`)
		must.Eq(t, structs.DynamicHostVolumeStatePending, e.records()[0].State)
	})

	t.Run("a delete that wins during the client call is reported by state", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) {
			m.hook = func() {
				rec := e.records()[0]
				_, _, err := e.s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeMarkDeletingRequestType,
					&structs.DynamicHostVolumeMarkDeletingRequest{
						Keys:         []structs.DynamicHostVolumeKey{{Namespace: rec.Namespace, ID: rec.ID}},
						ModifyTime:   2,
						WriteRequest: structs.WriteRequest{Region: "global"},
					})
				must.NoError(t, err)
			}
		})
		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 409, `volume state is "deleting"`)
		must.Eq(t, structs.DynamicHostVolumeStateDeleting, e.records()[0].State)
	})

	t.Run("the read back is checked against the state that was written", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.s.dhvCoordinator.setHook(func(point string) {
			if point != "outcome-applied" {
				return
			}
			rec := e.records()[0]
			_, _, err := e.s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeMarkDeletingRequestType,
				&structs.DynamicHostVolumeMarkDeletingRequest{
					Keys:         []structs.DynamicHostVolumeKey{{Namespace: rec.Namespace, ID: rec.ID}},
					ModifyTime:   2,
					WriteRequest: structs.WriteRequest{Region: "global"},
				})
			must.NoError(t, err)
		})
		_, _, err := e.create(dhvReqVolume("data"))
		mustCoded(t, err, 409, `volume state is "deleting"`)
	})

	t.Run("PolicyOverride changes nothing", func(t *testing.T) {
		e := newDHVEnv(t, false)
		var resp structs.HostVolumeCreateResponse
		err := e.rpc("HostVolume.Create", &structs.HostVolumeCreateRequest{
			Volume: dhvReqVolume("data"), PolicyOverride: true,
			WriteRequest: structs.WriteRequest{Region: "global", Namespace: "default"},
		}, &resp)
		must.NoError(t, err)
		must.True(t, usable(resp.Volume.State))
	})
}

func TestHostVolumeRegister_New(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)

	dir := t.TempDir()
	file := filepath.Join(dir, "keep.txt")
	must.NoError(t, os.WriteFile(file, []byte("existing"), 0o600))
	before, err := os.Stat(file)
	must.NoError(t, err)

	vol, index, err := e.register(dhvRegVolume("existing", e.nodeID, dir))
	must.NoError(t, err)
	must.True(t, usable(vol.State))
	must.Eq(t, structs.DynamicHostVolumeOriginRegistered, vol.Origin)
	must.Eq(t, dir, vol.HostPath)
	must.Eq(t, int64(0), vol.CapacityBytes)
	must.Eq(t, e.nodeID, vol.NodeID)
	must.Eq(t, "default", vol.NodePool)
	stored := e.get("default", vol.ID)
	must.True(t, stored.Equal(vol))
	must.Eq(t, stored.ModifyIndex, index)
	must.Eq(t, stored.CreateIndex+1, stored.ModifyIndex)

	_, nr := e.mock.counts()
	must.Eq(t, 1, nr)
	must.Eq(t, cstructs.HostVolumeRegisterRequest{
		VolumeID: vol.ID, Namespace: "default", Name: "existing", NodeID: e.nodeID, NodePool: "default", HostPath: dir,
	}, e.mock.registry[0])

	// The directory is untouched.
	content, err := os.ReadFile(file)
	must.NoError(t, err)
	must.Eq(t, "existing", string(content))
	after, err := os.Stat(file)
	must.NoError(t, err)
	must.Eq(t, before.ModTime(), after.ModTime())
	entries, err := os.ReadDir(dir)
	must.NoError(t, err)
	must.Len(t, 1, entries)
}

func TestHostVolumeRegister_NodeChecks(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)

	mk := func(mutate func(n *structs.Node)) *structs.Node {
		n := mock.Node()
		n.NodePool = "default"
		mutate(n)
		return n
	}
	down := mk(func(n *structs.Node) { n.Status = structs.NodeStatusDown })
	inel := mk(func(n *structs.Node) { n.SchedulingEligibility = structs.NodeSchedulingIneligible })
	otherPool := mk(func(n *structs.Node) { n.NodePool = "other-pool" })
	static := mk(func(n *structs.Node) {
		n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"taken": {Name: "taken", Path: "/synthetic/static"}}
	})
	for i, n := range []*structs.Node{down, inel, otherPool, static} {
		must.NoError(t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, uint64(10+i), n))
	}
	must.NoError(t, e.s.State().UpsertNamespaces(30, []*structs.Namespace{{Name: "other"}}))
	// A record in another namespace holds a name on the real node.
	_, _, err := e.s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeUpsertRequestType, &structs.DynamicHostVolumeUpsertRequest{
		WriteRequest: structs.WriteRequest{Region: "global"},
		Volumes: []*structs.DynamicHostVolume{func() *structs.DynamicHostVolume {
			v := dhvSeedVolume("held", "held-name", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginRegistered)
			v.Namespace, v.HostPath = "other", "/synthetic/held"
			return v
		}()},
	})
	must.NoError(t, err)

	cases := []struct {
		name string
		vol  *structs.DynamicHostVolume
		msg  string
	}{
		{"node does not exist", dhvRegVolume("n1", uuid.Generate(), "/synthetic/a"), "node_not_found"},
		{"node in another pool", func() *structs.DynamicHostVolume {
			v := dhvRegVolume("n2", otherPool.ID, "/synthetic/a")
			v.NodePool = "default"
			return v
		}(), "node_pool_mismatch"},
		{"node not ready", dhvRegVolume("n3", down.ID, "/synthetic/a"), "node_not_ready"},
		{"node ineligible", dhvRegVolume("n4", inel.ID, "/synthetic/a"), "node_ineligible"},
		{"name held by a static host volume", dhvRegVolume("taken", static.ID, "/synthetic/a"), "name_conflict"},
		{"name held by a record in another namespace", dhvRegVolume("held-name", e.nodeID, "/synthetic/a"), "name_conflict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := e.with(t)
			index, recs := e.snapshotTable()
			_, _, err := e.register(tc.vol)
			mustCoded(t, err, 400, tc.msg)
			e.mustUnchanged(index, recs)
			_, nr := e.mock.counts()
			must.Eq(t, 0, nr)
		})
	}
}

// The leader's node drainer finishes a drain with no allocations at once, so
// the draining rule is checked on the node check itself.
func TestRegisterHostVolumeNode_Draining(t *testing.T) {
	ci.Parallel(t)
	store := state.TestStateStore(t)
	node := mock.Node()
	node.NodePool = "default"
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 10, node))
	must.NoError(t, store.UpdateNodeDrain(structs.MsgTypeTestSetup, 11, node.ID,
		&structs.DrainStrategy{DrainSpec: structs.DrainSpec{Deadline: time.Hour}}, false, 0, nil, nil, ""))
	snap, err := store.Snapshot()
	must.NoError(t, err)

	_, err = registerHostVolumeNode(snap, dhvRegVolume("data", node.ID, "/synthetic/a"))
	mustCoded(t, err, 400, "node_draining")

	// Without the drain the same node is refused as ineligible, so the test
	// above proves the drain rule, not the eligibility rule.
	must.NoError(t, store.UpdateNodeDrain(structs.MsgTypeTestSetup, 12, node.ID, nil, false, 0, nil, nil, ""))
	snap, err = store.Snapshot()
	must.NoError(t, err)
	_, err = registerHostVolumeNode(snap, dhvRegVolume("data", node.ID, "/synthetic/a"))
	mustCoded(t, err, 400, "node_ineligible")
}

func TestHostVolumeRegister_ClientRejection(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.mock.set(func(m *mockClientHostVolume) {
		m.registerReply = func(*cstructs.HostVolumeRegisterRequest) cstructs.HostVolumeRegisterResponse {
			return cstructs.HostVolumeRegisterResponse{Reason: "path_missing", Message: "synthetic message"}
		}
	})
	vol, _, err := e.register(dhvRegVolume("existing", e.nodeID, "/synthetic/x"))
	must.NoError(t, err)
	must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
	must.Eq(t, "path_missing: synthetic message", vol.StatusDescription)

	t.Run("transport error", func(t *testing.T) {
		e := newDHVEnv(t, false)
		e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic") })
		_, _, err := e.register(dhvRegVolume("existing", e.nodeID, "/synthetic/x"))
		mustCoded(t, err, 502, `client call failed, volume state is "pending"`)
		must.Eq(t, structs.DynamicHostVolumeStatePending, e.records()[0].State)
	})
}

// updateCase is one request against a stored ready volume.
type updateCase struct {
	name   string
	mutate func(v *structs.DynamicHostVolume)
	code   int    // 0 for success
	msg    string // expected coded message
	noop   bool   // success with no write
}

func TestHostVolume_UpdateByID(t *testing.T) {
	ci.Parallel(t)

	for _, method := range []string{"HostVolume.Create", "HostVolume.Register"} {
		register := method == "HostVolume.Register"
		t.Run(method, func(t *testing.T) {
			e := newDHVEnv(t, false)
			// The stored volume sits on a node that no client update
			// touches, so a node write cannot move it to unavailable.
			ghost := uuid.Generate()
			origin := structs.DynamicHostVolumeOriginPluginCreated
			if register {
				origin = structs.DynamicHostVolumeOriginRegistered
			}
			first := dhvSeedVolume("orig", "data", ghost, structs.DynamicHostVolumeStateReady, origin)
			e.seedVolumes(first)

			// body builds the update request for the stored volume.
			body := func(mutate func(v *structs.DynamicHostVolume)) *structs.DynamicHostVolume {
				var v *structs.DynamicHostVolume
				if register {
					v = dhvRegVolume("data", "", "")
				} else {
					v = dhvReqVolume("data")
				}
				v.ID = first.ID
				mutate(v)
				return v
			}
			cases := []updateCase{
				{name: "identical", mutate: func(*structs.DynamicHostVolume) {}, noop: true},
				{name: "node id and pool omitted or all", mutate: func(v *structs.DynamicHostVolume) { v.NodeID, v.NodePool = "", "all" }, noop: true},
				{name: "node id and pool repeated", mutate: func(v *structs.DynamicHostVolume) { v.NodeID, v.NodePool = ghost, "default" }, noop: true},
				{name: "name", mutate: func(v *structs.DynamicHostVolume) { v.Name = "other" }, code: 400, msg: "immutable field Name"},
				{name: "type", mutate: func(v *structs.DynamicHostVolume) { v.Type = "csi" }, code: 400, msg: "immutable field Type"},
				{name: "node", mutate: func(v *structs.DynamicHostVolume) { v.NodeID = uuid.Generate() }, code: 400, msg: "immutable field NodeID"},
				{name: "node pool", mutate: func(v *structs.DynamicHostVolume) { v.NodePool = "elsewhere" }, code: 400, msg: "immutable field NodePool"},
				{name: "plugin", mutate: func(v *structs.DynamicHostVolume) { v.PluginID = "other-plugin" }, code: 400, msg: "immutable field PluginID"},
				{name: "invalid mutable field", mutate: func(v *structs.DynamicHostVolume) { v.RequestedCapabilities = nil },
					code: 400, msg: "invalid volume: RequestedCapabilities: required"},
			}
			if register {
				cases = append(cases,
					updateCase{name: "host path differs", mutate: func(v *structs.DynamicHostVolume) { v.HostPath = "/synthetic/else" }, code: 400, msg: "immutable field HostPath"},
					updateCase{name: "host path repeated", mutate: func(v *structs.DynamicHostVolume) { v.HostPath = "/synthetic/orig" }, noop: true},
				)
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					e := e.with(t)
					index, recs := e.snapshotTable()
					nc, nr := e.mock.counts()
					vol, replyIndex, err := e.call(method, "", "default", body(tc.mutate))
					switch {
					case tc.code != 0:
						mustCoded(t, err, tc.code, tc.msg)
						e.mustUnchanged(index, recs)
					case tc.noop:
						must.NoError(t, err)
						must.Eq(t, index, e.tableIndex())
						must.Eq(t, e.get("default", first.ID).ModifyIndex, replyIndex)
						must.True(t, e.get("default", first.ID).Equal(vol))
						e.mustUnchanged(index, recs)
					}
					nc2, nr2 := e.mock.counts()
					must.Eq(t, nc, nc2, must.Sprint("an update must not reach the plugin"))
					must.Eq(t, nr, nr2, must.Sprint("an update must not reach the plugin"))
				})
			}

			t.Run("mutable fields change and server-owned fields stay", func(t *testing.T) {
				e := e.with(t)
				before := e.get("default", first.ID).Copy()
				nc, nr := e.mock.counts()
				vol, replyIndex, err := e.call(method, "", "default", body(func(v *structs.DynamicHostVolume) {
					v.RequestedCapacityMaxBytes = 500
					v.Parameters = map[string]string{"k": "changed"}
					v.Constraints = []*structs.Constraint{{LTarget: "${attr.kernel.name}", Operand: "=", RTarget: "linux"}}
					v.RequestedCapabilities = append(v.RequestedCapabilities, &structs.DynamicHostVolumeCapability{
						AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem,
						AccessMode:     structs.DynamicHostVolumeAccessModeSingleNodeReader,
					})
				}))
				must.NoError(t, err)
				stored := e.get("default", first.ID)
				must.True(t, stored.Equal(vol))
				must.Greater(t, before.ModifyIndex, stored.ModifyIndex)
				must.Eq(t, stored.ModifyIndex, replyIndex)
				must.Eq(t, e.tableIndex(), replyIndex)
				must.Eq(t, int64(500), stored.RequestedCapacityMaxBytes)
				must.Eq(t, map[string]string{"k": "changed"}, stored.Parameters)
				must.Len(t, 2, stored.RequestedCapabilities)
				must.Len(t, 1, stored.Constraints)
				// Server-owned fields come from the stored record.
				must.Eq(t, before.HostPath, stored.HostPath)
				must.Eq(t, before.CapacityBytes, stored.CapacityBytes)
				must.Eq(t, before.NodeID, stored.NodeID)
				must.Eq(t, before.NodePool, stored.NodePool)
				must.Eq(t, before.Origin, stored.Origin)
				must.Eq(t, before.CreateIndex, stored.CreateIndex)
				must.Eq(t, before.CreateTime, stored.CreateTime)
				must.Eq(t, before.State, stored.State)
				nc2, nr2 := e.mock.counts()
				must.Eq(t, nc, nc2)
				must.Eq(t, nr, nr2)
			})

			t.Run("unknown ID", func(t *testing.T) {
				e := e.with(t)
				index, recs := e.snapshotTable()
				v := body(func(*structs.DynamicHostVolume) {})
				v.ID = uuid.Generate()
				_, _, err := e.call(method, "", "default", v)
				mustCoded(t, err, 404, "volume not found")
				e.mustUnchanged(index, recs)
			})

			t.Run("ID in another namespace is unknown", func(t *testing.T) {
				e := e.with(t)
				must.NoError(t, e.s.State().UpsertNamespaces(40, []*structs.Namespace{{Name: "other"}}))
				v := body(func(*structs.DynamicHostVolume) {})
				_, _, err := e.call(method, "", "other", v)
				mustCoded(t, err, 404, "volume not found")
			})

			t.Run("body namespace is refused", func(t *testing.T) {
				e := e.with(t)
				v := body(func(v *structs.DynamicHostVolume) { v.Namespace = "other" })
				_, _, err := e.call(method, "", "default", v)
				mustCoded(t, err, 400, "volume namespace does not match the request namespace")
			})

			t.Run("version of the other origin is refused", func(t *testing.T) {
				e := e.with(t)
				index, recs := e.snapshotTable()
				other := "HostVolume.Register"
				v := dhvRegVolume("data", "", "")
				if register {
					other = "HostVolume.Create"
					v = dhvReqVolume("data")
				}
				v.ID = first.ID
				_, _, err := e.call(other, "", "default", v)
				mustCoded(t, err, 400, "volume origin mismatch")
				e.mustUnchanged(index, recs)
			})
		})
	}
}

func TestHostVolume_UpdateRefusedByState(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)

	states := []structs.DynamicHostVolumeState{
		structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending,
		structs.DynamicHostVolumeStateFailed, structs.DynamicHostVolumeStateDeleting,
		structs.DynamicHostVolumeStateUnavailable,
	}
	ghost := uuid.Generate() // a node that no client update can touch
	var seeds []*structs.DynamicHostVolume
	for _, st := range states {
		for _, origin := range []structs.DynamicHostVolumeOrigin{
			structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeOriginRegistered} {
			node := ghost
			if st == structs.DynamicHostVolumeStateRequested {
				node = ""
			}
			seeds = append(seeds, dhvSeedVolume(fmt.Sprintf("v-%s-%s", st, origin), fmt.Sprintf("n-%s-%s", st, origin),
				node, st, origin))
		}
	}
	e.seedVolumesAt(1000, seeds...)

	for _, seed := range seeds {
		method := "HostVolume.Create"
		body := dhvReqVolume(seed.Name)
		if seed.Origin == structs.DynamicHostVolumeOriginRegistered {
			method = "HostVolume.Register"
			body = dhvRegVolume(seed.Name, "", "")
		}
		t.Run(seed.ID, func(t *testing.T) {
			e := e.with(t)
			index, recs := e.snapshotTable()
			body.ID = seed.ID
			_, _, err := e.call(method, "", "default", body)
			mustCoded(t, err, 409, fmt.Sprintf("volume state is %q", seed.State))
			e.mustUnchanged(index, recs)
		})
	}
}

func TestHostVolume_UpdateConflict(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	first, _, err := e.create(dhvReqVolume("data"))
	must.NoError(t, err)

	// Another writer changes the record between the read and the apply.
	e.s.dhvCoordinator.setHook(func(point string) {
		if point != "update-apply" {
			return
		}
		cur := e.get("default", first.ID).Copy()
		cur.Parameters = map[string]string{"other": "writer"}
		cur.ModifyTime++
		_, _, err := e.s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeUpdateRequestType,
			&structs.DynamicHostVolumeUpdateRequest{Volume: cur, WriteRequest: structs.WriteRequest{Region: "global"}})
		must.NoError(t, err)
	})
	upd := dhvReqVolume("data")
	upd.ID = first.ID
	upd.Parameters = map[string]string{"k": "mine"}
	_, _, err = e.create(upd)
	mustCoded(t, err, 409, "volume changed concurrently, retry")
	must.Eq(t, map[string]string{"other": "writer"}, e.get("default", first.ID).Parameters)
}

func TestHostVolume_OperationInProgress(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() {
			once.Do(func() { close(entered) })
			<-release
		}
	})
	done := make(chan error, 1)
	go func() {
		_, _, err := e.create(dhvReqVolume("data"))
		done <- err
	}()
	<-entered

	rec := e.records()[0]
	index, recs := e.snapshotTable()
	for _, method := range []string{"HostVolume.Create", "HostVolume.Register"} {
		v := dhvReqVolume("data")
		v.ID = rec.ID
		_, _, err := e.call(method, "", "default", v)
		mustCoded(t, err, 409, "operation in progress")
	}
	e.mustUnchanged(index, recs)
	close(release)
	must.NoError(t, <-done)

	// The mark is released: the same ID now reaches the update path.
	v := dhvReqVolume("data")
	v.ID = rec.ID
	_, _, err := e.create(v)
	must.NoError(t, err)
	must.False(t, e.s.dhvCoordinator.isInFlight("default/"+rec.ID))
}

func TestHostVolume_VersionGate(t *testing.T) {
	ci.Parallel(t)

	s1, cleanup1 := TestServer(t, nil)
	defer cleanup1()
	s2, cleanup2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.WanderVersion = "" // a peer that does not advertise a Wander version
	})
	defer cleanup2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForResult(func() (bool, error) {
		return len(s1.serf.Members()) == 2, fmt.Errorf("members: %d", len(s1.serf.Members()))
	}, func(err error) { must.NoError(t, err) })

	e := &dhvEnv{t: t, s: s1, mock: &mockClientHostVolume{}, seed: 1}
	c := startMockHostVolumeClient(t, s1, e.mock)
	e.nodeID = c.NodeID()
	e.advertisePlugin()

	stored := dhvSeedVolume("stored", "stored-name", uuid.Generate(), structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	e.seedVolumes(stored)

	const gate = "dynamic host volumes need every server to run Wander 0.1.2 or later"
	index, recs := e.snapshotTable()
	_, _, err := e.create(dhvReqVolume("data"))
	mustCoded(t, err, 503, gate)
	_, _, err = e.register(dhvRegVolume("ext", e.nodeID, "/synthetic/x"))
	mustCoded(t, err, 503, gate)
	upd := dhvReqVolume("stored-name")
	upd.ID = "stored"
	upd.Parameters = map[string]string{"k": "new"}
	_, _, err = e.create(upd)
	mustCoded(t, err, 503, gate)

	e.mustUnchanged(index, recs)
	nc, nr := e.mock.counts()
	must.Eq(t, 0, nc)
	must.Eq(t, 0, nr)
	must.False(t, e.s.dhvCoordinator.isInFlight("default/stored"))
}

// advertiseWanderVersion replaces the wander_version serf tag of the server.
// The local member takes the new tags at once, so the next submission through
// dynamicHostVolumeRaftApply sees them.
func advertiseWanderVersion(t *testing.T, s *Server, v string) {
	t.Helper()
	tags := maps.Clone(s.serf.LocalMember().Tags)
	tags[wanderVersionTag] = v
	must.NoError(t, s.serf.SetTags(tags))
}

// TestHostVolume_VersionGateAfterPending covers a gate that closes between the
// pending upsert and the outcome write. The refusal is reported like any other
// outcome the state refuses, and the record is left pending and untouched.
func TestHostVolume_VersionGateAfterPending(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() { advertiseWanderVersion(t, e.s, "0.1.1") }
	})

	_, _, err := e.create(dhvReqVolume("data"))
	mustCoded(t, err, 500, `outcome not recorded, volume state is "pending"`)

	recs := e.records()
	must.Len(t, 1, recs)
	rec := recs[0]
	must.Eq(t, structs.DynamicHostVolumeStatePending, rec.State)
	must.Eq(t, e.nodeID, rec.NodeID)
	must.Eq(t, "", rec.StatusDescription)
	must.Eq(t, "", rec.HostPath)
	must.Eq(t, int64(0), rec.CapacityBytes)
	// Only the pending upsert reached the log.
	must.Eq(t, rec.CreateIndex, rec.ModifyIndex)
	must.Eq(t, rec.ModifyIndex, e.tableIndex())
	must.False(t, e.s.dhvCoordinator.isInFlight("default/"+rec.ID))
	nc, nr := e.mock.counts()
	must.Eq(t, 1, nc)
	must.Eq(t, 0, nr)
}

func TestHostVolumeCreate_ACL(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, true)
	must.NoError(t, e.s.State().UpsertNamespaces(50, []*structs.Namespace{{Name: "other"}}))

	// Fixtures for the update cases, one per origin, in each namespace.
	// They sit on a node that no client update touches: a node write would
	// otherwise move volumes it does not advertise to unavailable and bump
	// the table index under a denied call.
	ghost := uuid.Generate()
	mkSeed := func(id, ns string, origin structs.DynamicHostVolumeOrigin) *structs.DynamicHostVolume {
		v := dhvSeedVolume(id, "n-"+id, ghost, structs.DynamicHostVolumeStateReady, origin)
		v.Namespace = ns
		return v
	}
	e.seedVolumes(
		mkSeed("created-default", "default", structs.DynamicHostVolumeOriginPluginCreated),
		mkSeed("registered-default", "default", structs.DynamicHostVolumeOriginRegistered),
		mkSeed("created-other", "other", structs.DynamicHostVolumeOriginPluginCreated),
		mkSeed("registered-other", "other", structs.DynamicHostVolumeOriginRegistered),
	)

	pol := func(ns, policy string, caps ...string) string { return mock.NamespacePolicy(ns, policy, caps) }
	const (
		c  = acl.NamespaceCapabilityHostVolumeCreate
		rg = acl.NamespaceCapabilityHostVolumeRegister
		w  = acl.NamespaceCapabilityHostVolumeWrite
	)
	cases := []struct {
		name     string
		policy   string
		token    string // "none", "bogus" or "root" for special cases
		create   bool
		register bool
	}{
		{name: "no token", token: "none"},
		{name: "unknown token", token: "bogus"},
		{name: "read capability", policy: pol("default", "", acl.NamespaceCapabilityHostVolumeRead)},
		{name: "delete capability", policy: pol("default", "", acl.NamespaceCapabilityHostVolumeDelete)},
		{name: "unrelated capability", policy: pol("default", "", acl.NamespaceCapabilityReadJob)},
		{name: "policy read", policy: pol("default", acl.PolicyRead)},
		{name: "policy deny", policy: pol("default", acl.PolicyDeny)},
		{name: "create only", policy: pol("default", "", c), create: true},
		{name: "register implies create", policy: pol("default", "", rg), create: true, register: true},
		{name: "write capability", policy: pol("default", "", w), create: true, register: true},
		{name: "policy write", policy: pol("default", acl.PolicyWrite), create: true, register: true},
		{name: "capability in another namespace only", policy: pol("other", "", w, c, rg)},
		{name: "deny overrides an allow", policy: pol("default", acl.PolicyDeny, w)},
		{name: "management", token: "root", create: true, register: true},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := e.with(t)
			var secret string
			switch tc.token {
			case "none":
				secret = "-"
			case "bogus":
				secret = "00000000-0000-0000-0000-000000000000"
			case "root":
				secret = e.root
			default:
				secret = mock.CreatePolicyAndToken(t, e.s.State(), uint64(1000+i*2), fmt.Sprintf("p%d", i), tc.policy).SecretID
			}
			// call treats an empty secret as management; "-" is no token.
			send := func(method, ns string, v *structs.DynamicHostVolume) error {
				tok := secret
				if tok == "-" {
					tok = ""
					wr := structs.WriteRequest{Region: "global", Namespace: ns}
					var err error
					if method == "HostVolume.Create" {
						err = e.rpc(method, &structs.HostVolumeCreateRequest{Volume: v, WriteRequest: wr}, &structs.HostVolumeCreateResponse{})
					} else {
						err = e.rpc(method, &structs.HostVolumeRegisterRequest{Volume: v, WriteRequest: wr}, &structs.HostVolumeRegisterResponse{})
					}
					return err
				}
				_, _, err := e.call(method, tok, ns, v)
				return err
			}

			check := func(label string, allow bool, method, ns string, mk func() *structs.DynamicHostVolume) {
				t.Run(label, func(t *testing.T) {
					e := e.with(t)
					index, recs := e.snapshotTable()
					nc, nr := e.mock.counts()
					err := send(method, ns, mk())
					if allow {
						must.NoError(t, err)
						return
					}
					must.EqError(t, err, structs.ErrPermissionDenied.Error())
					nc2, nr2 := e.mock.counts()
					must.Eq(t, nc, nc2, must.Sprint("a denied call reached the client"))
					must.Eq(t, nr, nr2, must.Sprint("a denied call reached the client"))
					e.mustUnchanged(index, recs)
				})
			}
			newCreate := func() *structs.DynamicHostVolume { return dhvReqVolume("c-" + uuid.Short()) }
			newRegister := func() *structs.DynamicHostVolume {
				return dhvRegVolume("r-"+uuid.Short(), e.nodeID, "/synthetic/"+uuid.Short())
			}
			updateOf := func(id, name string, register bool) func() *structs.DynamicHostVolume {
				return func() *structs.DynamicHostVolume {
					v := dhvReqVolume(name)
					if register {
						v = dhvRegVolume(name, "", "")
					}
					v.ID = id
					v.Parameters = map[string]string{"k": uuid.Short()}
					return v
				}
			}
			check("create", tc.create, "HostVolume.Create", "default", newCreate)
			check("register", tc.register, "HostVolume.Register", "default", newRegister)
			check("update through create", tc.create, "HostVolume.Create", "default",
				updateOf("created-default", "n-created-default", false))
			check("update through register", tc.register, "HostVolume.Register", "default",
				updateOf("registered-default", "n-registered-default", true))
			// Nothing in the other namespace is granted to these policies
			// except where the policy names it.
			otherAllowed := strings.Contains(tc.name, "another namespace")
			if tc.token == "root" {
				otherAllowed = true
			}
			check("create in other namespace", otherAllowed, "HostVolume.Create", "other", newCreate)
			check("register in other namespace", otherAllowed, "HostVolume.Register", "other", newRegister)
		})
	}

	t.Run("wildcard namespace with a management token", func(t *testing.T) {
		e := e.with(t)
		_, _, err := e.call("HostVolume.Create", e.root, structs.AllNamespacesSentinel, dhvReqVolume("w"))
		mustCoded(t, err, 400, "a volume write needs a single namespace")
	})

	t.Run("wildcard namespace without the capability anywhere is denied", func(t *testing.T) {
		e := e.with(t)
		tok := mock.CreatePolicyAndToken(t, e.s.State(), 3000, "wild", pol("default", "", acl.NamespaceCapabilityHostVolumeRead))
		_, _, err := e.call("HostVolume.Create", tok.SecretID, structs.AllNamespacesSentinel, dhvReqVolume("w"))
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})

	t.Run("wildcard namespace with the capability somewhere is refused as a bad request", func(t *testing.T) {
		e := e.with(t)
		tok := mock.CreatePolicyAndToken(t, e.s.State(), 3002, "wild2", pol("default", "", w))
		_, _, err := e.call("HostVolume.Create", tok.SecretID, structs.AllNamespacesSentinel, dhvReqVolume("w"))
		mustCoded(t, err, 400, "a volume write needs a single namespace")
	})
}

// The server stamps the record with its own clock.
func TestHostVolumeCreate_Timestamps(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	start := time.Now().UnixNano()
	vol, _, err := e.create(dhvReqVolume("data"))
	must.NoError(t, err)
	must.GreaterEq(t, start, vol.CreateTime)
	must.GreaterEq(t, vol.CreateTime, vol.ModifyTime)
}

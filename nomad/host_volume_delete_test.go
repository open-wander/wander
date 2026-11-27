// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

const (
	dhvMethodDelete     = "HostVolume.Delete"
	dhvMethodDeregister = "HostVolume.Deregister"

	dhvInUse = "volume in use"
	dhvGate  = "dynamic host volumes need every server to run Wander 0.1.2 or later"
)

// dhvUseIndex hands out state indexes for allocation and node fixtures. They
// sit far above the Raft log, and none of those tables is written through Raft
// by these tests.
var dhvUseIndex atomic.Uint64

func init() { dhvUseIndex.Store(1_000_000) }

func nextDHVIndex() uint64 { return dhvUseIndex.Add(10) }

// remove sends HostVolume.Delete or HostVolume.Deregister. An empty secret
// means the management token when ACLs are on and "-" means no token.
func (e *dhvEnv) remove(method, secret, ns, id string, force bool) (uint64, error) {
	e.t.Helper()
	if secret == "" {
		secret = e.root
	}
	if secret == "-" {
		secret = ""
	}
	wr := structs.WriteRequest{Region: "global", Namespace: ns, AuthToken: secret}
	switch method {
	case dhvMethodDelete:
		var resp structs.HostVolumeDeleteResponse
		err := e.rpc(method, &structs.HostVolumeDeleteRequest{VolumeID: id, Force: force, WriteRequest: wr}, &resp)
		return resp.Index, err
	case dhvMethodDeregister:
		var resp structs.HostVolumeDeregisterResponse
		err := e.rpc(method, &structs.HostVolumeDeregisterRequest{VolumeID: id, Force: force, WriteRequest: wr}, &resp)
		return resp.Index, err
	}
	e.t.Fatalf("unknown method %s", method)
	return 0, nil
}

// methodFor returns the call that matches the origin of a volume.
func methodFor(origin structs.DynamicHostVolumeOrigin) string {
	if origin == structs.DynamicHostVolumeOriginRegistered {
		return dhvMethodDeregister
	}
	return dhvMethodDelete
}

// stateNode adds a node with the given status to the state store. It must run
// before the volumes are seeded, so that the node write does not move them.
func (e *dhvEnv) stateNode(status string) string {
	e.t.Helper()
	n := mock.Node()
	n.Status = status
	must.NoError(e.t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, nextDHVIndex(), n))
	return n.ID
}

// seedAll writes the fixtures in one batch at the lowest index, below the Raft
// log, so that later writes through Raft move the table index forward.
func (e *dhvEnv) seedAll(vols ...*structs.DynamicHostVolume) {
	e.t.Helper()
	e.seed++
	must.Less(e.t, e.s.raft.LastIndex(), e.seed, must.Sprint("too many seeds for the raft index"))
	must.NoError(e.t, e.s.State().UpsertDynamicHostVolumes(e.seed, vols))
}

// use adds an allocation on the node that mounts the volume named source.
func (e *dhvEnv) use(nodeID, source, clientStatus string) {
	e.t.Helper()
	job := mock.Job()
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"data": {Name: "data", Type: structs.VolumeTypeHost, Source: source}}
	alloc := mock.Alloc()
	alloc.Job, alloc.JobID, alloc.Namespace = job, job.ID, job.Namespace
	alloc.TaskGroup = job.TaskGroups[0].Name
	alloc.NodeID = nodeID
	alloc.Name = job.ID + "." + alloc.TaskGroup + "[0]"
	alloc.ClientStatus, alloc.DesiredStatus = structs.AllocClientStatusPending, structs.AllocDesiredStatusRun
	i := nextDHVIndex()
	must.NoError(e.t, e.s.State().UpsertJob(structs.MsgTypeTestSetup, i, nil, job))
	must.NoError(e.t, e.s.State().UpsertAllocs(structs.MsgTypeTestSetup, i+1, []*structs.Allocation{alloc}))
	if clientStatus != structs.AllocClientStatusPending {
		update := alloc.Copy()
		update.ClientStatus = clientStatus
		must.NoError(e.t, e.s.State().UpdateAllocsFromClient(structs.MsgTypeTestSetup, i+2, []*structs.Allocation{update}))
	}
}

// onClient builds a volume on the client node in the given state, for seedAll.
// The fixtures are written to state, not made through HostVolume.Create or
// HostVolume.Register: those pick the node by the plugin attribute, which the
// client's fingerprinter can drop, and a create that finds no node leaves the
// volume nodeless, so a later delete would never call the client.
//
// A node update from the client moves a ready volume the node does not
// advertise to unavailable. The removal still calls the client for it, since
// the node is live and unavailable may move to deleting.
func (e *dhvEnv) onClient(origin structs.DynamicHostVolumeOrigin, ns, name string, st structs.DynamicHostVolumeState) *structs.DynamicHostVolume {
	v := dhvSeedVolume(uuid.Generate(), name, e.nodeID, st, origin)
	v.Namespace = ns
	return v
}

// created seeds a ready plugin-created volume on the client node. Every seed
// must come before the first volume write through Raft.
func (e *dhvEnv) created(name string) *structs.DynamicHostVolume {
	e.t.Helper()
	v := e.onClient(structs.DynamicHostVolumeOriginPluginCreated, "default", name, structs.DynamicHostVolumeStateReady)
	e.seedAll(v)
	return v
}

// registered seeds a ready registered volume on the client node. Every seed
// must come before the first volume write through Raft.
func (e *dhvEnv) registered(name string) *structs.DynamicHostVolume {
	e.t.Helper()
	v := e.onClient(structs.DynamicHostVolumeOriginRegistered, "default", name, structs.DynamicHostVolumeStateReady)
	e.seedAll(v)
	return v
}

// mustKept asserts that a refused call left the volume as it was. A node
// update may move it between ready and unavailable meanwhile, once or more,
// which changes only the state and the modify index, so those two may differ
// within the usable states and nothing else may.
func (e *dhvEnv) mustKept(before *structs.DynamicHostVolume) {
	e.t.Helper()
	after := e.get(before.Namespace, before.ID)
	must.NotNil(e.t, after, must.Sprint("the volume was removed"))
	must.True(e.t, usable(after.State), must.Sprintf("the volume is %s", after.State))
	want := before.Copy()
	want.State, want.ModifyIndex = after.State, after.ModifyIndex
	must.True(e.t, want.Equal(after))
}

func (e *dhvEnv) mustCounts(deletes, deregs int) {
	e.t.Helper()
	d, r := e.mock.removalCounts()
	must.Eq(e.t, deletes, d, must.Sprint("delete calls"))
	must.Eq(e.t, deregs, r, must.Sprint("deregister calls"))
}

func TestHostVolumeDelete_Created(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := e.created("data")
	createIndex := e.tableIndex()

	// The client call must run with the volume marked deleting, marked in
	// flight and with no placement lock held.
	var lockFree, inFlight atomic.Bool
	var seenState atomic.Value
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() {
			if e.s.dhvCoordinator.placeMu.TryLock() {
				lockFree.Store(true)
				e.s.dhvCoordinator.placeMu.Unlock()
			}
			inFlight.Store(e.s.dhvCoordinator.isInFlight("default/" + vol.ID))
			if v := e.get("default", vol.ID); v != nil {
				seenState.Store(v.State)
			}
		}
	})

	index, err := e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	must.NoError(t, err)

	must.True(t, lockFree.Load(), must.Sprint("placement lock held across the client call"))
	must.True(t, inFlight.Load())
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, seenState.Load().(structs.DynamicHostVolumeState))
	must.False(t, e.s.dhvCoordinator.isInFlight("default/"+vol.ID))

	must.Nil(t, e.get("default", vol.ID))
	must.Greater(t, createIndex, index)
	must.Eq(t, e.tableIndex(), index)

	e.mustCounts(1, 0)
	firstDelete, _ := e.mock.firstRemovals()
	must.Eq(t, cstructs.HostVolumeDeleteRequest{VolumeID: vol.ID, Namespace: "default", NodeID: e.nodeID}, firstDelete)

	// A repeat after the removal is not found.
	_, err = e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	mustCoded(t, err, 404, "volume not found")
	e.mustCounts(1, 0)
}

func TestHostVolumeDeregister_Registered(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := e.registered("ext")
	registerIndex := e.tableIndex()

	var lockFree, inFlight atomic.Bool
	var seenState atomic.Value
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() {
			if e.s.dhvCoordinator.placeMu.TryLock() {
				lockFree.Store(true)
				e.s.dhvCoordinator.placeMu.Unlock()
			}
			inFlight.Store(e.s.dhvCoordinator.isInFlight("default/" + vol.ID))
			if v := e.get("default", vol.ID); v != nil {
				seenState.Store(v.State)
			}
		}
	})

	index, err := e.remove(dhvMethodDeregister, "", "default", vol.ID, false)
	must.NoError(t, err)

	must.True(t, lockFree.Load())
	must.True(t, inFlight.Load())
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, seenState.Load().(structs.DynamicHostVolumeState))
	must.Nil(t, e.get("default", vol.ID))
	must.Greater(t, registerIndex, index)
	must.Eq(t, e.tableIndex(), index)

	// No plugin runs: the only client call is the deregistration.
	e.mustCounts(0, 1)
	_, firstDereg := e.mock.firstRemovals()
	must.Eq(t, cstructs.HostVolumeDeregisterRequest{VolumeID: vol.ID, Namespace: "default", NodeID: e.nodeID}, firstDereg)

	_, err = e.remove(dhvMethodDeregister, "", "default", vol.ID, false)
	mustCoded(t, err, 404, "volume not found")
	e.mustCounts(0, 1)
}

func TestHostVolumeRemove_CrossKindRefused(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	ghost := uuid.Generate()
	made := dhvSeedVolume("made", "n-made", ghost, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	reg := dhvSeedVolume("reg", "n-reg", ghost, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginRegistered)
	odd := dhvSeedVolume("odd", "n-odd", ghost, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	odd.Origin = ""
	e.seedAll(made, reg, odd)

	cases := []struct {
		name, method, id, msg string
	}{
		{"deregister of a plugin volume", dhvMethodDeregister, "made", "volume was created by a plugin, use delete"},
		{"delete of a registered volume", dhvMethodDelete, "reg", "volume was registered, use deregister"},
		{"delete of a volume with no origin", dhvMethodDelete, "odd", "volume origin is unknown"},
		{"deregister of a volume with no origin", dhvMethodDeregister, "odd", "volume origin is unknown"},
	}
	for _, tc := range cases {
		for _, force := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s force=%v", tc.name, force), func(t *testing.T) {
				e := e.with(t)
				index, recs := e.snapshotTable()
				_, err := e.remove(tc.method, "", "default", tc.id, force)
				mustCoded(t, err, 400, tc.msg)
				e.mustUnchanged(index, recs)
				e.mustCounts(0, 0)
			})
		}
	}
}

func TestHostVolumeRemove_InUseRefused(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	down := e.stateNode(structs.NodeStatusDown)
	disconnected := e.stateNode(structs.NodeStatusDisconnected)
	unknown := uuid.Generate()

	type fixture struct {
		node, origin, clientStatus string
	}
	var fixtures []*structs.DynamicHostVolume
	byName := map[string]fixture{}
	for _, n := range []struct{ kind, id, status string }{
		{"unknown", unknown, structs.AllocClientStatusRunning},
		{"down", down, structs.AllocClientStatusRunning},
		// The allocations of a disconnected node are unknown, which is not
		// terminal, so force does not free the volume.
		{"disconnected", disconnected, structs.AllocClientStatusUnknown},
	} {
		for _, origin := range []structs.DynamicHostVolumeOrigin{
			structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeOriginRegistered} {
			name := fmt.Sprintf("%s-%s", n.kind, origin)
			v := dhvSeedVolume("id-"+name, name, n.id, structs.DynamicHostVolumeStateReady, origin)
			fixtures = append(fixtures, v)
			byName[name] = fixture{node: n.id, origin: string(origin), clientStatus: n.status}
		}
	}
	// A volume that is already deleting is not marked again, so only the
	// check before the first write keeps the client from deleting it.
	repeat := dhvSeedVolume("id-deleting", "deleting", unknown, structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated)
	fixtures = append(fixtures, repeat)
	byName["deleting"] = fixture{node: unknown, clientStatus: structs.AllocClientStatusRunning}
	e.seedAll(fixtures...)
	for _, v := range fixtures {
		e.use(v.NodeID, v.Name, byName[v.Name].clientStatus)
	}

	for _, v := range fixtures {
		for _, force := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s force=%v", v.Name, force), func(t *testing.T) {
				e := e.with(t)
				index, recs := e.snapshotTable()
				_, err := e.remove(methodFor(v.Origin), "", "default", v.ID, force)
				mustCoded(t, err, 409, dhvInUse)
				e.mustUnchanged(index, recs)
				e.mustCounts(0, 0)
			})
		}
	}
}

// A volume on a live node is in use too, with force and without it.
func TestHostVolumeRemove_InUseOnReadyNode(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := e.created("data")
	e.use(e.nodeID, "data", structs.AllocClientStatusRunning)

	for _, force := range []bool{false, true} {
		_, err := e.remove(dhvMethodDelete, "", "default", vol.ID, force)
		mustCoded(t, err, 409, dhvInUse)
		must.True(t, usable(e.get("default", vol.ID).State))
	}
	e.mustCounts(0, 0)
}

// An allocation that the plan applier commits after the server looked at the
// volume is refused by the apply, before the client can delete anything.
func TestHostVolumeRemove_InUseAfterTheCheck(t *testing.T) {
	ci.Parallel(t)
	for _, method := range []string{dhvMethodDelete, dhvMethodDeregister} {
		t.Run(method, func(t *testing.T) {
			e := newDHVEnv(t, false)
			var vol *structs.DynamicHostVolume
			if method == dhvMethodDelete {
				vol = e.created("data")
			} else {
				vol = e.registered("data")
			}
			e.s.dhvCoordinator.setHook(func(point string) {
				if point == "delete-checked" {
					e.use(e.nodeID, vol.Name, structs.AllocClientStatusPending)
				}
			})

			_, err := e.remove(method, "", "default", vol.ID, false)
			mustCoded(t, err, 409, dhvInUse)
			must.True(t, usable(e.get("default", vol.ID).State), must.Sprint("the volume was marked deleting"))
			e.mustCounts(0, 0)
		})
	}
}

func TestHostVolumeRemove_Force(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	down := e.stateNode(structs.NodeStatusDown)
	disconnected := e.stateNode(structs.NodeStatusDisconnected)
	initializing := e.stateNode(structs.NodeStatusInit)
	unknown := uuid.Generate()

	nodes := []struct{ kind, id string }{{"unknown", unknown}, {"down", down}, {"disconnected", disconnected}}
	var fixtures []*structs.DynamicHostVolume
	for _, n := range nodes {
		for _, origin := range []structs.DynamicHostVolumeOrigin{
			structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeOriginRegistered} {
			name := fmt.Sprintf("%s-%s", n.kind, origin)
			fixtures = append(fixtures,
				dhvSeedVolume("force-"+name, name, n.id, structs.DynamicHostVolumeStateReady, origin),
				dhvSeedVolume("plain-"+name, name+"-plain", n.id, structs.DynamicHostVolumeStateReady, origin))
		}
	}
	init := dhvSeedVolume("init", "init", initializing, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	fixtures = append(fixtures, init)
	e.seedAll(fixtures...)

	t.Run("force removes the volume without a client call", func(t *testing.T) {
		e := e.with(t)
		for _, v := range fixtures {
			if !strings.HasPrefix(v.ID, "force-") {
				continue
			}
			index, err := e.remove(methodFor(v.Origin), "", "default", v.ID, true)
			must.NoError(t, err, must.Sprint(v.Name))
			must.Nil(t, e.get("default", v.ID))
			must.Eq(t, e.tableIndex(), index)
		}
		e.mustCounts(0, 0)
	})

	t.Run("without force a node that is gone refuses before any write", func(t *testing.T) {
		e := e.with(t)
		for _, v := range fixtures {
			if !strings.HasPrefix(v.ID, "plain-") {
				continue
			}
			index, recs := e.snapshotTable()
			_, err := e.remove(methodFor(v.Origin), "", "default", v.ID, false)
			want := "node is " + strings.SplitN(v.Name, "-", 2)[0] + ", force is needed to remove the volume"
			mustCoded(t, err, 400, want)
			e.mustUnchanged(index, recs)
		}
		e.mustCounts(0, 0)
	})

	t.Run("force on a node that is not gone is refused naming the status", func(t *testing.T) {
		e := e.with(t)
		index, recs := e.snapshotTable()
		_, err := e.remove(dhvMethodDelete, "", "default", "init", true)
		mustCoded(t, err, 400, `force is only for a down, disconnected or unknown node, node status is "initializing"`)
		e.mustUnchanged(index, recs)
		e.mustCounts(0, 0)
	})
}

func TestHostVolumeRemove_ForceOnReadyNode(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	made := e.onClient(structs.DynamicHostVolumeOriginPluginCreated, "default", "data", structs.DynamicHostVolumeStateReady)
	reg := e.onClient(structs.DynamicHostVolumeOriginRegistered, "default", "ext", structs.DynamicHostVolumeStateReady)
	e.seedAll(made, reg)
	for _, vol := range []*structs.DynamicHostVolume{made, reg} {
		before := e.get("default", vol.ID)
		_, err := e.remove(methodFor(vol.Origin), "", "default", vol.ID, true)
		mustCoded(t, err, 400, `force is only for a down, disconnected or unknown node, node status is "ready"`)
		e.mustKept(before)
	}
	e.mustCounts(0, 0)
}

func TestHostVolumeRemove_NoNode(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	requested := dhvSeedVolume("requested", "n-requested", "", structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeOriginPluginCreated)
	failed := dhvSeedVolume("failed", "n-failed", "", structs.DynamicHostVolumeStateFailed, structs.DynamicHostVolumeOriginPluginCreated)
	failed.StatusDescription = "no_eligible_node: node_pool=1"
	e.seedAll(requested, failed)

	// Nothing is on disk, so there is nobody to call and force is not needed.
	for _, id := range []string{"requested", "failed"} {
		index, err := e.remove(dhvMethodDelete, "", "default", id, false)
		must.NoError(t, err)
		must.Nil(t, e.get("default", id))
		must.Eq(t, e.tableIndex(), index)
	}
	e.mustCounts(0, 0)
}

// A failed volume that has a node gets the normal call: a client that no
// longer holds the volume answers success.
func TestHostVolumeDelete_FailedWithNode(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := e.onClient(structs.DynamicHostVolumeOriginPluginCreated, "default", "data", structs.DynamicHostVolumeStateFailed)
	vol.StatusDescription = "nonzero_exit"
	e.seedAll(vol)

	_, err := e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	must.NoError(t, err)
	must.Nil(t, e.get("default", vol.ID))
	e.mustCounts(1, 0)
}

func TestHostVolumeRemove_ClientRejection(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name, reason, want string
	}{
		{"delete mismatch", "delete_mismatch", "delete_mismatch"},
		{"kind conflict", "volume_kind_conflict", "volume_kind_conflict"},
		{"plugin failure", "nonzero_exit", "nonzero_exit"},
		{"unlisted reason", "/synthetic/secret/path reason", "client_rejected"},
	}
	for _, tc := range cases {
		for _, method := range []string{dhvMethodDelete, dhvMethodDeregister} {
			t.Run(tc.name+" "+method, func(t *testing.T) {
				e := newDHVEnv(t, false)
				var vol *structs.DynamicHostVolume
				if method == dhvMethodDelete {
					vol = e.created("data")
				} else {
					vol = e.registered("ext")
				}
				e.mock.set(func(m *mockClientHostVolume) {
					m.deleteReply = func(req *cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
						return cstructs.HostVolumeDeleteResponse{VolumeID: req.VolumeID, Reason: tc.reason, Message: "/synthetic/secret/path " + e.nodeID}
					}
					m.deregReply = func(req *cstructs.HostVolumeDeregisterRequest) cstructs.HostVolumeDeregisterResponse {
						return cstructs.HostVolumeDeregisterResponse{VolumeID: req.VolumeID, Reason: tc.reason, Message: "/synthetic/secret/path " + e.nodeID}
					}
				})

				_, err := e.remove(method, "", "default", vol.ID, false)
				// The text is fixed: it names the code and the state, and never
				// the client message, the node or the volume.
				mustCoded(t, err, 502, `client rejected: `+tc.want+`, volume state is "deleting"`)
				must.Eq(t, structs.DynamicHostVolumeStateDeleting, e.get("default", vol.ID).State)

				// A repeat is accepted and drives the removal again.
				e.mock.set(func(m *mockClientHostVolume) { m.deleteReply, m.deregReply = nil, nil })
				_, err = e.remove(method, "", "default", vol.ID, false)
				must.NoError(t, err)
				must.Nil(t, e.get("default", vol.ID))
				d, r := e.mock.removalCounts()
				must.Eq(t, 2, d+r)
			})
		}
	}
}

func TestHostVolumeRemove_TransportFailure(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := e.created("data")
	e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("dial tcp 192.0.2.1: /synthetic/secret/path") })

	_, err := e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	mustCoded(t, err, 502, `client call failed, volume state is "deleting"`)
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, e.get("default", vol.ID).State)
	must.False(t, e.s.dhvCoordinator.isInFlight("default/"+vol.ID))

	// The client is back: the repeat removes it.
	e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
	_, err = e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	must.NoError(t, err)
	must.Nil(t, e.get("default", vol.ID))
}

func TestHostVolumeRemove_RepeatOfDeletingVolume(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	// A deleting volume with no node has nothing to call.
	v := dhvSeedVolume("gone", "n-gone", "", structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated)
	e.seedAll(v)
	_, err := e.remove(dhvMethodDelete, "", "default", "gone", false)
	must.NoError(t, err)
	must.Nil(t, e.get("default", "gone"))
}

func TestHostVolumeRemove_Errors(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	data := e.onClient(structs.DynamicHostVolumeOriginPluginCreated, "default", "data", structs.DynamicHostVolumeStateReady)
	held := e.onClient(structs.DynamicHostVolumeOriginPluginCreated, "default", "held", structs.DynamicHostVolumeStateReady)
	e.seedAll(data, held)

	t.Run("unknown volume", func(t *testing.T) {
		e := e.with(t)
		for _, m := range []string{dhvMethodDelete, dhvMethodDeregister} {
			_, err := e.remove(m, "", "default", uuid.Generate(), false)
			mustCoded(t, err, 404, "volume not found")
		}
	})
	t.Run("missing ID", func(t *testing.T) {
		e := e.with(t)
		for _, m := range []string{dhvMethodDelete, dhvMethodDeregister} {
			_, err := e.remove(m, "", "default", "", false)
			mustCoded(t, err, 400, "missing volume ID")
		}
	})
	t.Run("wildcard namespace", func(t *testing.T) {
		e := e.with(t)
		for _, m := range []string{dhvMethodDelete, dhvMethodDeregister} {
			_, err := e.remove(m, "", structs.AllNamespacesSentinel, "x", false)
			mustCoded(t, err, 400, "a volume write needs a single namespace")
		}
	})
	t.Run("volume in another namespace is not found", func(t *testing.T) {
		e := e.with(t)
		before := e.get("default", data.ID)
		_, err := e.remove(dhvMethodDelete, "", "other", data.ID, false)
		mustCoded(t, err, 404, "volume not found")
		e.mustKept(before)
	})
	t.Run("volume a create holds is in progress", func(t *testing.T) {
		e := e.with(t)
		must.True(t, e.s.dhvCoordinator.begin("default/"+held.ID))
		defer e.s.dhvCoordinator.end("default/" + held.ID)
		before := e.get("default", held.ID)
		_, err := e.remove(dhvMethodDelete, "", "default", held.ID, false)
		mustCoded(t, err, 409, "operation in progress")
		e.mustKept(before)
	})
}

func TestHostVolumeDelete_Concurrent(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	vol := e.created("data")

	reached := make(chan struct{})
	release := make(chan struct{})
	e.s.dhvCoordinator.setHook(func(point string) {
		if point == "delete-marked" {
			close(reached)
			<-release
		}
	})

	first := make(chan error, 1)
	go func() {
		_, err := e.remove(dhvMethodDelete, "", "default", vol.ID, false)
		first <- err
	}()
	select {
	case <-reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the first delete did not reach the marked state")
	}

	// The second call is refused while the first holds the volume, and it
	// does not touch the record.
	_, err := e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	mustCoded(t, err, 409, "operation in progress")
	_, err = e.remove(dhvMethodDeregister, "", "default", vol.ID, false)
	mustCoded(t, err, 409, "operation in progress")
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, e.get("default", vol.ID).State)

	close(release)
	must.NoError(t, <-first)
	e.mustCounts(1, 0)

	_, err = e.remove(dhvMethodDelete, "", "default", vol.ID, false)
	mustCoded(t, err, 404, "volume not found")
	e.mustCounts(1, 0)
}

func TestHostVolumeRemove_VersionGate(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	// The gate closes after the fixtures exist: the version tag of the server
	// is what the gate reads.
	ghost := uuid.Generate()
	made := dhvSeedVolume("made", "n-made", ghost, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	reg := dhvSeedVolume("reg", "n-reg", ghost, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginRegistered)
	nodeless := dhvSeedVolume("nodeless", "n-nodeless", "", structs.DynamicHostVolumeStateFailed, structs.DynamicHostVolumeOriginPluginCreated)
	e.seedAll(made, reg, nodeless)
	advertiseWanderVersion(t, e.s, "0.1.1")

	index, recs := e.snapshotTable()
	for _, tc := range []struct {
		method, id string
		force      bool
	}{
		{dhvMethodDelete, "made", true},
		{dhvMethodDeregister, "reg", true},
		{dhvMethodDelete, "nodeless", false},
	} {
		_, err := e.remove(tc.method, "", "default", tc.id, tc.force)
		mustCoded(t, err, 503, dhvGate)
	}
	e.mustUnchanged(index, recs)
	e.mustCounts(0, 0)
	must.False(t, e.s.dhvCoordinator.isInFlight("default/made"))
}

// A gate that closes between the mark and the removal leaves the volume
// deleting, and the error says so.
func TestHostVolumeRemove_VersionGateAfterMark(t *testing.T) {
	ci.Parallel(t)
	for _, method := range []string{dhvMethodDelete, dhvMethodDeregister} {
		t.Run(method, func(t *testing.T) {
			e := newDHVEnv(t, false)
			var vol *structs.DynamicHostVolume
			if method == dhvMethodDelete {
				vol = e.created("data")
			} else {
				vol = e.registered("ext")
			}
			e.mock.set(func(m *mockClientHostVolume) {
				m.hook = func() { advertiseWanderVersion(t, e.s, "0.1.1") }
			})

			_, err := e.remove(method, "", "default", vol.ID, false)
			mustCoded(t, err, 503, dhvGate+`, volume state is "deleting"`)
			rec := e.get("default", vol.ID)
			must.Eq(t, structs.DynamicHostVolumeStateDeleting, rec.State)
			// Only the mark reached the log after the fixture write.
			must.Eq(t, rec.ModifyIndex, e.tableIndex())
			must.False(t, e.s.dhvCoordinator.isInFlight("default/"+vol.ID))
		})
	}
}

func TestHostVolumeRemove_ACL(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, true)
	must.NoError(t, e.s.State().UpsertNamespaces(50, []*structs.Namespace{{Name: "other"}}))

	pol := func(ns, policy string, caps ...string) string { return mock.NamespacePolicy(ns, policy, caps) }
	const (
		del = acl.NamespaceCapabilityHostVolumeDelete
		w   = acl.NamespaceCapabilityHostVolumeWrite
	)
	cases := []struct {
		name         string
		token        string // "none", "bogus", "root" or a policy
		policy       string
		def, otherNS bool
	}{
		{name: "no token", token: "none"},
		{name: "unknown token", token: "bogus"},
		{name: "management token", token: "root", def: true, otherNS: true},
		{name: "delete capability", policy: pol("default", "", del), def: true},
		{name: "write capability", policy: pol("default", "", w), def: true},
		{name: "policy write", policy: pol("default", acl.PolicyWrite), def: true},
		{name: "read capability", policy: pol("default", "", acl.NamespaceCapabilityHostVolumeRead)},
		{name: "create and register capabilities", policy: pol("default", "", acl.NamespaceCapabilityHostVolumeCreate, acl.NamespaceCapabilityHostVolumeRegister)},
		{name: "policy read", policy: pol("default", acl.PolicyRead)},
		{name: "deny overrides an allow", policy: pol("default", acl.PolicyDeny, del)},
		{name: "unrelated capability", policy: pol("default", "", acl.NamespaceCapabilityReadJob)},
		{name: "delete capability in another namespace", policy: pol("other", "", del), otherNS: true},
	}
	// Every case gets its own volume for each namespace and call, seeded in
	// one batch before the first write through Raft.
	type fixtureKey struct {
		c          int
		ns, method string
	}
	vols := map[fixtureKey]*structs.DynamicHostVolume{}
	var all []*structs.DynamicHostVolume
	for i := range cases {
		for _, ns := range []string{"default", "other"} {
			for _, method := range []string{dhvMethodDelete, dhvMethodDeregister} {
				origin := structs.DynamicHostVolumeOriginPluginCreated
				if method == dhvMethodDeregister {
					origin = structs.DynamicHostVolumeOriginRegistered
				}
				v := e.onClient(origin, ns, fmt.Sprintf("v%d-%s", i, uuid.Short()), structs.DynamicHostVolumeStateReady)
				vols[fixtureKey{i, ns, method}] = v
				all = append(all, v)
			}
		}
	}
	e.seedAll(all...)

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

			for _, ns := range []string{"default", "other"} {
				allow := tc.def
				if ns == "other" {
					allow = tc.otherNS
				}
				for _, method := range []string{dhvMethodDelete, dhvMethodDeregister} {
					t.Run(ns+" "+method, func(t *testing.T) {
						e := e.with(t)
						vol := vols[fixtureKey{i, ns, method}]
						d0, r0 := e.mock.removalCounts()

						_, err := e.remove(method, secret, ns, vol.ID, false)
						d1, r1 := e.mock.removalCounts()
						if allow {
							must.NoError(t, err)
							must.Nil(t, e.get(ns, vol.ID))
							must.Eq(t, d0+r0+1, d1+r1)
							return
						}
						must.EqError(t, err, structs.ErrPermissionDenied.Error())
						must.Eq(t, d0+r0, d1+r1, must.Sprint("a denied call reached the client"))
						rec := e.get(ns, vol.ID)
						must.NotNil(t, rec)
						must.NotEq(t, structs.DynamicHostVolumeStateDeleting, rec.State)
					})
				}
			}
		})
	}

	t.Run("wildcard namespace with a management token", func(t *testing.T) {
		e := e.with(t)
		_, err := e.remove(dhvMethodDelete, e.root, structs.AllNamespacesSentinel, "x", false)
		mustCoded(t, err, 400, "a volume write needs a single namespace")
	})
	t.Run("wildcard namespace without the capability anywhere is denied", func(t *testing.T) {
		e := e.with(t)
		tok := mock.CreatePolicyAndToken(t, e.s.State(), 3000, "wild", pol("default", "", acl.NamespaceCapabilityHostVolumeRead))
		_, err := e.remove(dhvMethodDeregister, tok.SecretID, structs.AllNamespacesSentinel, "x", false)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})
	t.Run("wildcard namespace with the capability somewhere is a bad request", func(t *testing.T) {
		e := e.with(t)
		tok := mock.CreatePolicyAndToken(t, e.s.State(), 3002, "wild2", pol("default", "", del))
		_, err := e.remove(dhvMethodDeregister, tok.SecretID, structs.AllNamespacesSentinel, "x", false)
		mustCoded(t, err, 400, "a volume write needs a single namespace")
	})
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package scheduler

import (
	"errors"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// stickyFixture is a store with two ready nodes, each with a dynamic host
// volume record named data-a, and a job whose group requests data-a sticky.
type stickyFixture struct {
	t     *testing.T
	store *state.StateStore
	ctx   *EvalContext
	nodes [2]*structs.Node
	vols  [2]*structs.DynamicHostVolume
	job   *structs.Job
	index uint64
}

func newStickyFixture(t *testing.T) *stickyFixture {
	f := &stickyFixture{t: t, index: 1000}
	f.store, f.ctx = testContext(t)
	for i := range f.nodes {
		id := uuid.Generate()
		f.nodes[i] = dynamicHostVolumeNode(id, "data-a", "/synthetic/data-a")
		f.vols[i] = dynamicHostVolumeRecord(f.nodes[i], id, structs.DefaultNamespace, "data-a")
		must.NoError(t, f.store.UpsertNode(structs.MsgTypeTestSetup, f.next(), f.nodes[i]))
		must.NoError(t, f.store.UpsertDynamicHostVolumes(f.next(), []*structs.DynamicHostVolume{f.vols[i]}))
	}
	f.job = mock.Job()
	f.job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{"data": {
		Name: "data", Type: structs.VolumeTypeHost, Source: "data-a", Sticky: true,
	}}
	other := f.job.TaskGroups[0].Copy()
	other.Name = "other-group"
	f.job.TaskGroups = append(f.job.TaskGroups, other)
	must.NoError(t, f.store.UpsertJob(structs.MsgTypeTestSetup, f.next(), nil, f.job))
	f.ctx.Plan().Job = f.job
	return f
}

func (f *stickyFixture) next() uint64 {
	f.index++
	return f.index
}

func (f *stickyFixture) group() *structs.TaskGroup { return f.job.TaskGroups[0] }

// claim records a claim of the job's group on volume i.
func (f *stickyFixture) claim(i int) {
	f.t.Helper()
	must.NoError(f.t, f.store.UpsertDynamicHostVolumeClaims(f.next(), []*structs.DynamicHostVolumeClaim{{
		ID: uuid.Generate(), Namespace: f.job.Namespace, JobID: f.job.ID, TaskGroupName: f.group().Name,
		VolumeName: "data-a", VolumeID: f.vols[i].ID,
	}}))
}

// alloc builds an allocation of the group on node i.
func (f *stickyFixture) alloc(i int, desired, client string) *structs.Allocation {
	a := mock.Alloc()
	a.Job = f.job
	a.JobID = f.job.ID
	a.Namespace = f.job.Namespace
	a.TaskGroup = f.group().Name
	a.NodeID = f.nodes[i].ID
	a.Name = structs.AllocName(f.job.ID, f.group().Name, uint(i))
	a.DesiredStatus = desired
	a.ClientStatus = client
	return a
}

// store1 stores the allocations. A new allocation is always stored as desired
// run and client pending, so the wanted statuses are applied afterwards the way
// the server and the client report them.
func (f *stickyFixture) store1(allocs ...*structs.Allocation) {
	f.t.Helper()
	storeStickyAllocs(f.t, f.store, f.next, allocs)
}

func storeStickyAllocs(t *testing.T, store *state.StateStore, next func() uint64, allocs []*structs.Allocation) {
	t.Helper()
	for _, a := range allocs {
		desired, client := a.DesiredStatus, a.ClientStatus
		initial := a.Copy()
		initial.DesiredStatus, initial.ClientStatus = structs.AllocDesiredStatusRun, structs.AllocClientStatusPending
		must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, next(), []*structs.Allocation{initial}))
		server := initial.Copy()
		server.DesiredStatus = desired
		must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, next(), []*structs.Allocation{server}))
		report := initial.Copy()
		report.ClientStatus = client
		must.NoError(t, store.UpdateAllocsFromClient(structs.MsgTypeTestSetup, next(), []*structs.Allocation{report}))
	}
}

func (f *stickyFixture) checker() *HostVolumeChecker {
	c := NewHostVolumeChecker(f.ctx)
	c.SetNamespace(f.job.Namespace)
	c.SetJobID(f.job.ID)
	c.SetTaskGroup(f.group().Name)
	c.SetVolumes(structs.AllocName(f.job.ID, f.group().Name, 0), f.group().Volumes)
	return c
}

// feasible reports the verdict of a fresh checker for both nodes.
func (f *stickyFixture) feasible() [2]bool {
	c := f.checker()
	return [2]bool{c.Feasible(f.nodes[0]), c.Feasible(f.nodes[1])}
}

func TestHostVolumeChecker_Sticky(t *testing.T) {
	ci.Parallel(t)

	run := structs.AllocDesiredStatusRun
	stop := structs.AllocDesiredStatusStop
	running := structs.AllocClientStatusRunning

	cases := []struct {
		name  string
		setup func(f *stickyFixture)
		want  [2]bool
	}{
		{"no claims allows both volumes", func(f *stickyFixture) {}, [2]bool{true, true}},
		{"a free claim forces its volume", func(f *stickyFixture) { f.claim(0) }, [2]bool{true, false}},
		{"a claim used by a live allocation allows both", func(f *stickyFixture) {
			f.claim(0)
			f.store1(f.alloc(0, run, running))
		}, [2]bool{true, true}},
		{"a claim whose allocation the plan stops is free", func(f *stickyFixture) {
			f.claim(0)
			a := f.alloc(0, run, running)
			f.store1(a)
			f.ctx.Plan().AppendStoppedAlloc(a, "stop", "", "")
		}, [2]bool{true, false}},
		{"a claim whose allocation the plan preempts is free", func(f *stickyFixture) {
			f.claim(0)
			a := f.alloc(0, run, running)
			f.store1(a)
			f.ctx.Plan().AppendPreemptedAlloc(a, uuid.Generate())
		}, [2]bool{true, false}},
		// Wander policy: a server-terminal allocation does not use its volume,
		// even while the client still runs it, so a re-run goes back to the
		// claimed volume and waits there if the writer accounting says so.
		{"desired stop but still running does not use the claim", func(f *stickyFixture) {
			f.claim(0)
			f.store1(f.alloc(0, stop, running))
		}, [2]bool{true, false}},
		{"client failed does not use the claim", func(f *stickyFixture) {
			f.claim(0)
			f.store1(f.alloc(0, run, structs.AllocClientStatusFailed))
		}, [2]bool{true, false}},
		{"client complete does not use the claim", func(f *stickyFixture) {
			f.claim(0)
			f.store1(f.alloc(0, stop, structs.AllocClientStatusComplete))
		}, [2]bool{true, false}},
		{"in plan: no claims, the first placement does not constrain the second", func(f *stickyFixture) {
			f.ctx.Plan().AppendAlloc(f.alloc(0, run, structs.AllocClientStatusPending), nil)
		}, [2]bool{true, true}},
		{"in plan: a claim used by a live allocation, the second may go to either", func(f *stickyFixture) {
			f.claim(0)
			f.store1(f.alloc(0, run, running))
			f.ctx.Plan().AppendAlloc(f.alloc(1, run, structs.AllocClientStatusPending), nil)
		}, [2]bool{true, true}},
		{"in plan: a placement on the claimed volume uses the claim", func(f *stickyFixture) {
			f.claim(0)
			f.ctx.Plan().AppendAlloc(f.alloc(0, run, structs.AllocClientStatusPending), nil)
		}, [2]bool{true, true}},
		{"both claims used leaves any volume allowed", func(f *stickyFixture) {
			f.claim(0)
			f.claim(1)
			f.store1(f.alloc(0, run, running), f.alloc(1, run, running))
		}, [2]bool{true, true}},
		{"one of two claims free forces that one", func(f *stickyFixture) {
			f.claim(0)
			f.claim(1)
			f.store1(f.alloc(1, run, running))
		}, [2]bool{true, false}},
		{"an allocation of another group does not use the claim", func(f *stickyFixture) {
			f.claim(0)
			other := f.alloc(0, run, running)
			other.TaskGroup = "other-group"
			f.store1(other)
		}, [2]bool{true, false}},
		{"a claim of another group is ignored", func(f *stickyFixture) {
			must.NoError(f.t, f.store.UpsertDynamicHostVolumeClaims(f.next(), []*structs.DynamicHostVolumeClaim{{
				ID: uuid.Generate(), Namespace: f.job.Namespace, JobID: f.job.ID, TaskGroupName: "other-group",
				VolumeName: "data-a", VolumeID: f.vols[0].ID,
			}}))
		}, [2]bool{true, true}},
		{"a request that is not sticky follows no rule", func(f *stickyFixture) {
			f.claim(0)
			f.group().Volumes["data"].Sticky = false
		}, [2]bool{true, true}},
		{"a claim whose volume record is gone is ignored", func(f *stickyFixture) {
			restore, err := f.store.Restore()
			must.NoError(f.t, err)
			must.NoError(f.t, restore.DynamicHostVolumeClaimRestore(&structs.DynamicHostVolumeClaim{
				ID: uuid.Generate(), Namespace: f.job.Namespace, JobID: f.job.ID, TaskGroupName: f.group().Name,
				VolumeName: "data-a", VolumeID: uuid.Generate(), CreateIndex: 5, ModifyIndex: 5,
			}))
			must.NoError(f.t, restore.Commit())
		}, [2]bool{true, true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStickyFixture(t)
			tc.setup(f)
			must.Eq(t, tc.want, f.feasible())

			// A node filtered by the rule carries the ordinary reason, which
			// names no volume.
			filtered := f.ctx.Metrics().ConstraintFiltered
			if tc.want[0] && tc.want[1] {
				must.MapLen(t, 0, filtered)
				return
			}
			must.MapLen(t, 1, filtered)
			must.MapContainsKey(t, filtered, FilterConstraintHostVolumes)
		})
	}
}

// The free claims depend on the plan, which grows between placements, so the
// verdict must be recomputed by every SetVolumes.
func TestHostVolumeChecker_StickyRecomputedPerPlacement(t *testing.T) {
	ci.Parallel(t)

	f := newStickyFixture(t)
	f.claim(0)
	a := f.alloc(0, structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning)
	f.store1(a)

	c := f.checker()
	must.True(t, c.Feasible(f.nodes[1]))

	f.ctx.Plan().AppendStoppedAlloc(a, "stop", "", "")
	c.SetVolumes(structs.AllocName(f.job.ID, f.group().Name, 0), f.group().Volumes)
	must.False(t, c.Feasible(f.nodes[1]))
	must.True(t, c.Feasible(f.nodes[0]))
}

func TestHostVolumeChecker_StickyStaticVolume(t *testing.T) {
	ci.Parallel(t)

	f := newStickyFixture(t)
	f.claim(0)
	static := mock.Node()
	static.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"data-a": {Name: "data-a", Path: "/synthetic/static"}}
	must.True(t, f.checker().Feasible(static))
}

// stickyFailingState fails the claims read.
type stickyFailingState struct{ State }

func (stickyFailingState) DynamicHostVolumeClaimsByJobID(memdb.WatchSet, string, string) (memdb.ResultIterator, error) {
	return nil, errors.New("synthetic failure naming /synthetic/path")
}

func TestHostVolumeChecker_StickyFailsClosed(t *testing.T) {
	ci.Parallel(t)

	f := newStickyFixture(t)
	ctx := NewEvalContext(nil, stickyFailingState{f.store}, f.ctx.Plan(), testlog.HCLogger(t))
	c := NewHostVolumeChecker(ctx)
	c.SetNamespace(f.job.Namespace)
	c.SetJobID(f.job.ID)
	c.SetTaskGroup(f.group().Name)
	c.SetVolumes(structs.AllocName(f.job.ID, f.group().Name, 0), f.group().Volumes)
	must.False(t, c.Feasible(f.nodes[0]))
	must.False(t, c.Feasible(f.nodes[1]))
	for reason := range ctx.Metrics().ConstraintFiltered {
		must.Eq(t, FilterConstraintHostVolumes, reason)
	}
}

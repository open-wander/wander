// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"testing"

	"github.com/hashicorp/raft"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

const stickyReason = "dynamic host volume sticky claim"

// stickyPlanFixture is a snapshot source with two ready nodes, each advertising
// its own dynamic volume named data-a, and a job whose group requests it
// sticky.
type stickyPlanFixture struct {
	t     *testing.T
	state *state.StateStore
	nodes [2]*structs.Node
	vols  [2]*structs.DynamicHostVolume
	job   *structs.Job
	index uint64
}

func newStickyPlanFixture(t *testing.T) *stickyPlanFixture {
	return newStickyPlanFixtureOn(t, testStateStore(t))
}

func newStickyPlanFixtureOn(t *testing.T, store *state.StateStore) *stickyPlanFixture {
	f := &stickyPlanFixture{t: t, state: store, index: 1000}
	for i := range f.nodes {
		id := uuid.Generate()
		n := mock.Node()
		n.ReservedResources = nil
		n.Reserved = nil
		n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
			"data-a": {Name: "data-a", Path: "/synthetic/host-path-marker", ID: id},
		}
		f.nodes[i] = n
		f.vols[i] = &structs.DynamicHostVolume{
			ID: id, Name: "data-a", Namespace: structs.DefaultNamespace, Type: structs.VolumeTypeHost,
			NodeID: n.ID, PluginID: "mkdir", State: structs.DynamicHostVolumeStateReady,
			RequestedCapabilities: []*structs.DynamicHostVolumeCapability{{
				AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem,
				AccessMode:     structs.DynamicHostVolumeAccessModeSingleNodeWriter,
			}},
		}
		f.write(func(i uint64) error { return f.state.UpsertNode(structs.MsgTypeTestSetup, i, n) })
		f.write(func(i uint64) error {
			return f.state.UpsertDynamicHostVolumes(i, []*structs.DynamicHostVolume{f.vols[nodeIndex(f, n)]})
		})
	}
	f.job = mock.Job()
	f.job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{"data": {
		Name: "data", Type: structs.VolumeTypeHost, Source: "data-a", Sticky: true,
		AccessMode: structs.CSIVolumeAccessMode(structs.DynamicHostVolumeAccessModeSingleNodeWriter),
	}}
	f.write(func(i uint64) error { return f.state.UpsertJob(structs.MsgTypeTestSetup, i, nil, f.job) })
	return f
}

// nodeIndex finds the position of the node.
func nodeIndex(f *stickyPlanFixture, n *structs.Node) int {
	for i := range f.nodes {
		if f.nodes[i] == n {
			return i
		}
	}
	panic("unknown node")
}

func (f *stickyPlanFixture) write(fn func(index uint64) error) {
	f.index++
	must.NoError(f.t, fn(f.index))
}

func (f *stickyPlanFixture) group() string { return f.job.TaskGroups[0].Name }

func (f *stickyPlanFixture) claim(volume int) *structs.DynamicHostVolumeClaim {
	c := &structs.DynamicHostVolumeClaim{
		ID: uuid.Generate(), Namespace: f.job.Namespace, JobID: f.job.ID, TaskGroupName: f.group(),
		VolumeName: "data-a", VolumeID: f.vols[volume].ID,
	}
	f.write(func(i uint64) error {
		return f.state.UpsertDynamicHostVolumeClaims(i, []*structs.DynamicHostVolumeClaim{c})
	})
	return c
}

// alloc builds an allocation of the job's group on node i with a small
// footprint, as stored (the job is attached).
func (f *stickyPlanFixture) alloc(node, index int) *structs.Allocation {
	a := mock.Alloc()
	a.Job = f.job
	a.JobID = f.job.ID
	a.Namespace = f.job.Namespace
	a.NodeID = f.nodes[node].ID
	a.TaskGroup = f.group()
	a.Name = structs.AllocName(f.job.ID, f.group(), uint(index))
	a.AllocatedResources = &structs.AllocatedResources{
		Tasks: map[string]*structs.AllocatedTaskResources{
			"web": {Cpu: structs.AllocatedCpuResources{CpuShares: 100}, Memory: structs.AllocatedMemoryResources{MemoryMB: 64}},
		},
		Shared: structs.AllocatedSharedResources{DiskMB: 10},
	}
	return a
}

// planAlloc is shaped like a scheduler placement: the job rides on the plan.
func (f *stickyPlanFixture) planAlloc(node, index int) *structs.Allocation {
	a := f.alloc(node, index)
	a.Job = nil
	return a
}

// store writes allocations with the given statuses, applying the desired
// status as the server and the client status as the client would.
func (f *stickyPlanFixture) store(desired, client string, allocs ...*structs.Allocation) {
	for _, a := range allocs {
		initial := a.Copy()
		initial.DesiredStatus, initial.ClientStatus = structs.AllocDesiredStatusRun, structs.AllocClientStatusPending
		f.write(func(i uint64) error {
			return f.state.UpsertAllocs(structs.MsgTypeTestSetup, i, []*structs.Allocation{initial})
		})
		server := initial.Copy()
		server.DesiredStatus = desired
		f.write(func(i uint64) error {
			return f.state.UpsertAllocs(structs.MsgTypeTestSetup, i, []*structs.Allocation{server})
		})
		report := initial.Copy()
		report.ClientStatus = client
		f.write(func(i uint64) error {
			return f.state.UpdateAllocsFromClient(structs.MsgTypeTestSetup, i, []*structs.Allocation{report})
		})
	}
}

func (f *stickyPlanFixture) evaluate(plan *structs.Plan, node int) (bool, string) {
	snap, err := f.state.Snapshot()
	must.NoError(f.t, err)
	fit, reason, err := evaluateNodePlan(snap, plan, f.nodes[node].ID)
	must.NoError(f.t, err)
	return fit, reason
}

func (f *stickyPlanFixture) placing(allocs ...*structs.Allocation) *structs.Plan {
	plan := &structs.Plan{Job: f.job, NodeAllocation: map[string][]*structs.Allocation{}}
	for _, a := range allocs {
		plan.NodeAllocation[a.NodeID] = append(plan.NodeAllocation[a.NodeID], a)
	}
	return plan
}

func TestPlanApply_EvalNodePlan_DynamicHostVolumeSticky(t *testing.T) {
	ci.Parallel(t)

	run := structs.AllocDesiredStatusRun
	stop := structs.AllocDesiredStatusStop
	running := structs.AllocClientStatusRunning

	cases := []struct {
		name string
		// run builds the snapshot and returns the plan and the node that is
		// evaluated.
		run    func(f *stickyPlanFixture) (*structs.Plan, int)
		fit    bool
		reason string
	}{
		{
			name: "no claims: any volume",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				return f.placing(f.planAlloc(1, 0)), 1
			},
			fit: true,
		},
		{
			name: "a free claim: the claimed volume",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				return f.placing(f.planAlloc(0, 0)), 0
			},
			fit: true,
		},
		{
			// The group holds a claim that no live allocation uses, so a
			// placement on a volume it has no claim on is refused.
			name: "a free claim: another volume is refused",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				return f.placing(f.planAlloc(1, 0)), 1
			},
			reason: stickyReason,
		},
		{
			// The scheduler saw the holder live and placed on the other
			// volume; the holder has since been stopped.
			name: "a stale view: the holder was stopped after the plan was made",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				f.store(stop, structs.AllocClientStatusComplete, f.alloc(0, 0))
				return f.placing(f.planAlloc(1, 0)), 1
			},
			reason: stickyReason,
		},
		{
			name: "a stale view: the holder became client-terminal",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				f.store(run, structs.AllocClientStatusFailed, f.alloc(0, 0))
				return f.placing(f.planAlloc(1, 0)), 1
			},
			reason: stickyReason,
		},
		{
			name: "the holder is live: the other volume is allowed",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				f.store(run, running, f.alloc(0, 0))
				return f.placing(f.planAlloc(1, 1)), 1
			},
			fit: true,
		},
		{
			name: "the plan stops the holder: the other volume is refused",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				holder := f.alloc(0, 0)
				f.store(run, running, holder)
				stopped := holder.Copy()
				stopped.DesiredStatus = stop
				plan := f.placing(f.planAlloc(1, 0))
				plan.NodeUpdate = map[string][]*structs.Allocation{holder.NodeID: {stopped}}
				return plan, 1
			},
			reason: stickyReason,
		},
		{
			name: "the plan stops the holder: the claimed volume is allowed",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				holder := f.alloc(0, 0)
				f.store(run, running, holder)
				stopped := holder.Copy()
				stopped.DesiredStatus = stop
				plan := f.placing(f.planAlloc(0, 0))
				plan.NodeUpdate = map[string][]*structs.Allocation{holder.NodeID: {stopped}}
				return plan, 0
			},
			fit: true,
		},
		{
			name: "the plan places the group on the claimed volume too: the other volume is allowed",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				return f.placing(f.planAlloc(0, 0), f.planAlloc(1, 1)), 1
			},
			fit: true,
		},
		{
			name: "an allocation already stored is not checked again",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				existing := f.alloc(1, 0)
				f.store(run, running, existing)
				updated := existing.Copy()
				updated.Job = nil
				return f.placing(updated), 1
			},
			fit: true,
		},
		{
			name: "a claim whose volume record is gone is ignored",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				restore, err := f.state.Restore()
				must.NoError(f.t, err)
				must.NoError(f.t, restore.DynamicHostVolumeClaimRestore(&structs.DynamicHostVolumeClaim{
					ID: uuid.Generate(), Namespace: f.job.Namespace, JobID: f.job.ID, TaskGroupName: f.group(),
					VolumeName: "data-a", VolumeID: uuid.Generate(), CreateIndex: 5, ModifyIndex: 5,
				}))
				must.NoError(f.t, restore.Commit())
				return f.placing(f.planAlloc(1, 0)), 1
			},
			fit: true,
		},
		{
			name: "a request that is not sticky follows no rule",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				f.job.TaskGroups[0].Volumes["data"].Sticky = false
				return f.placing(f.planAlloc(1, 0)), 1
			},
			fit: true,
		},
		{
			name: "a static volume follows no rule",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				static := f.nodes[1].Copy()
				static.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"data-a": {Name: "data-a", Path: "/synthetic/static"}}
				f.write(func(i uint64) error { return f.state.UpsertNode(structs.MsgTypeTestSetup, i, static) })
				return f.placing(f.planAlloc(1, 0)), 1
			},
			fit: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStickyPlanFixture(t)
			plan, node := tc.run(f)
			fit, reason := f.evaluate(plan, node)
			must.Eq(t, tc.fit, fit)
			must.Eq(t, tc.reason, reason)
			must.StrNotContains(t, reason, f.vols[0].ID)
			must.StrNotContains(t, reason, f.vols[1].ID)
			must.StrNotContains(t, reason, "synthetic")
		})
	}
}

func TestPlanApply_EvalNodePlan_DynamicHostVolumeStickyReadOnly(t *testing.T) {
	ci.Parallel(t)

	f := newStickyPlanFixture(t)
	f.claim(0)
	plan := f.placing(f.planAlloc(1, 0), f.planAlloc(0, 1))
	snap, err := f.state.Snapshot()
	must.NoError(t, err)
	for range 4 {
		_, _, err := evaluateNodePlan(snap, plan, f.nodes[1].ID)
		must.NoError(t, err)
	}
	must.Eq(t, 2, len(plan.NodeAllocation))
	must.Nil(t, plan.NodeAllocation[f.nodes[1].ID][0].Job)
}

// stickyApply runs a plan through applyPlan and the Raft log of a test server.
type stickyApply struct {
	t *testing.T
	s *Server
	f *stickyPlanFixture
}

func newStickyApply(t *testing.T) *stickyApply {
	s, cleanup := TestServer(t, nil)
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	f := newStickyPlanFixtureOn(t, s.State())
	f.index = 5000
	for _, n := range f.nodes {
		testRegisterNode(t, s, n)
	}
	return &stickyApply{t: t, s: s, f: f}
}

func (a *stickyApply) apply(allocs ...*structs.Allocation) (*state.StateSnapshot, uint64) {
	a.t.Helper()
	snap, err := a.s.State().Snapshot()
	must.NoError(a.t, err)
	plan := a.f.placing(allocs...)
	result := &structs.PlanResult{NodeAllocation: plan.NodeAllocation}
	future, err := a.s.applyPlan(plan, result, snap)
	must.NoError(a.t, err)
	index, err := planWaitFuture(future)
	must.NoError(a.t, err)
	return snap, index
}

func (a *stickyApply) claims() []*structs.DynamicHostVolumeClaim {
	a.t.Helper()
	iter, err := a.s.fsm.State().DynamicHostVolumeClaimsByJobID(nil, a.f.job.Namespace, a.f.job.ID)
	must.NoError(a.t, err)
	var out []*structs.DynamicHostVolumeClaim
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		out = append(out, raw.(*structs.DynamicHostVolumeClaim))
	}
	return out
}

func TestPlanApply_applyPlan_StickyClaims(t *testing.T) {
	ci.Parallel(t)

	t.Run("a placement on an unclaimed volume creates a claim in the same apply", func(t *testing.T) {
		a := newStickyApply(t)
		placed := a.f.planAlloc(0, 0)
		snap, index := a.apply(placed)

		claims := a.claims()
		must.Len(t, 1, claims)
		claim := claims[0]
		must.Eq(t, a.f.job.Namespace, claim.Namespace)
		must.Eq(t, a.f.job.ID, claim.JobID)
		must.Eq(t, a.f.group(), claim.TaskGroupName)
		must.Eq(t, "data-a", claim.VolumeName)
		must.Eq(t, a.f.vols[0].ID, claim.VolumeID)
		must.Eq(t, placed.ID, claim.AllocationID)
		must.Eq(t, uint64(0), claim.AllocationIndex)
		must.Eq(t, index, claim.CreateIndex)
		must.NotEq(t, "", claim.ID)

		alloc, err := a.s.fsm.State().AllocByID(nil, placed.ID)
		must.NoError(t, err)
		must.Eq(t, claim.CreateIndex, alloc.CreateIndex, must.Sprint("claim and allocation share one apply"))
		claimsIndex, err := a.s.fsm.State().Index(state.TableDynamicHostVolumeClaims)
		must.NoError(t, err)
		must.Eq(t, index, claimsIndex)

		// The optimistic snapshot carries the claim too, so the next queued
		// plan sees it.
		optimistic, err := snap.DynamicHostVolumeClaimsByJobID(nil, a.f.job.Namespace, a.f.job.ID)
		must.NoError(t, err)
		must.NotNil(t, optimistic.Next())
	})

	t.Run("two placements on one new volume create one claim", func(t *testing.T) {
		a := newStickyApply(t)
		first, second := a.f.planAlloc(0, 0), a.f.planAlloc(0, 1)
		a.apply(first, second)
		claims := a.claims()
		must.Len(t, 1, claims)
		must.Eq(t, a.f.vols[0].ID, claims[0].VolumeID)
	})

	t.Run("placements on two new volumes create two claims", func(t *testing.T) {
		a := newStickyApply(t)
		a.apply(a.f.planAlloc(0, 0), a.f.planAlloc(1, 1))
		must.Len(t, 2, a.claims())
	})

	t.Run("a placement on an already claimed volume leaves the claim as it is", func(t *testing.T) {
		a := newStickyApply(t)
		first := a.f.planAlloc(0, 0)
		a.apply(first)
		before := a.claims()
		must.Len(t, 1, before)

		a.apply(a.f.planAlloc(0, 1))
		after := a.claims()
		must.Len(t, 1, after)
		must.Eq(t, before[0], after[0])
		must.Eq(t, first.ID, after[0].AllocationID)
	})

	t.Run("two queued plans on one new volume create one claim", func(t *testing.T) {
		a := newStickyApply(t)

		// The second plan is evaluated against the snapshot the first was
		// applied to optimistically, before the first has committed. That view
		// must already hold the first claim.
		snap, err := a.s.State().Snapshot()
		must.NoError(t, err)
		var futures []raft.ApplyFuture
		for _, alloc := range []*structs.Allocation{a.f.planAlloc(0, 0), a.f.planAlloc(0, 1)} {
			plan := a.f.placing(alloc)
			future, err := a.s.applyPlan(plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation}, snap)
			must.NoError(t, err)
			futures = append(futures, future)
		}
		for _, future := range futures {
			_, err := planWaitFuture(future)
			must.NoError(t, err)
		}
		must.Len(t, 1, a.claims())
	})
}

func TestPlanApply_StickyClaimsForPlan(t *testing.T) {
	ci.Parallel(t)

	t.Run("only new allocations of sticky requests on dynamic volumes", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		existing := f.alloc(1, 0)
		f.store(structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning, existing)
		updated := existing.Copy()
		updated.Job = nil

		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		plan := f.placing(updated)
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 0, claims, must.Sprint("an update of a stored allocation creates no claim"))

		f.job.TaskGroups[0].Volumes["data"].Sticky = false
		plan = f.placing(f.planAlloc(0, 1))
		claims, err = stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 0, claims, must.Sprint("not sticky"))
	})

	t.Run("a static volume creates no claim", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		static := f.nodes[0].Copy()
		static.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"data-a": {Name: "data-a", Path: "/synthetic/static"}}
		f.write(func(i uint64) error { return f.state.UpsertNode(structs.MsgTypeTestSetup, i, static) })
		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		plan := f.placing(f.planAlloc(0, 0))
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 0, claims)
	})

	t.Run("two placements on one volume give one claim", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		first, second := f.planAlloc(0, 0), f.planAlloc(0, 1)
		plan := f.placing(first, second)
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 1, claims)
		must.Eq(t, first.ID, claims[0].AllocationID)
		must.Eq(t, f.vols[0].ID, claims[0].VolumeID)
		must.NotEq(t, "", claims[0].ID)
	})

	t.Run("an existing claim of the group on the volume suppresses a new one", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		f.claim(0)
		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		plan := f.placing(f.planAlloc(0, 1))
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 0, claims)
	})

	t.Run("a claim of another group on the volume does not suppress a new one", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		other := f.job.TaskGroups[0].Copy()
		other.Name = "group-other"
		f.job.TaskGroups = append(f.job.TaskGroups, other)
		f.write(func(i uint64) error { return f.state.UpsertJob(structs.MsgTypeTestSetup, i, nil, f.job) })
		held := &structs.DynamicHostVolumeClaim{
			ID: uuid.Generate(), Namespace: f.job.Namespace, JobID: f.job.ID, TaskGroupName: other.Name,
			VolumeName: "data-a", VolumeID: f.vols[0].ID,
		}
		f.write(func(i uint64) error {
			return f.state.UpsertDynamicHostVolumeClaims(i, []*structs.DynamicHostVolumeClaim{held})
		})
		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		plan := f.placing(f.planAlloc(0, 0))
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 1, claims)
		must.Eq(t, f.group(), claims[0].TaskGroupName)
	})

	t.Run("a per-alloc source names the suffixed volume", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		f.job.TaskGroups[0].Volumes["data"].PerAlloc = true
		n := f.nodes[1].Copy()
		n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"data-a[1]": {Name: "data-a[1]", Path: "/synthetic/x", ID: f.vols[1].ID}}
		f.write(func(idx uint64) error { return f.state.UpsertNode(structs.MsgTypeTestSetup, idx, n) })
		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		plan := f.placing(f.planAlloc(1, 1))
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 1, claims)
		must.Eq(t, "data-a[1]", claims[0].VolumeName)
		must.Eq(t, uint64(1), claims[0].AllocationIndex)
	})
}

// The claim of a sticky group stays with its volume while the client is
// disconnected: an allocation in client status unknown does not hold it, so a
// replacement is refused on any other volume of the name.
func TestPlanApply_EvalNodePlan_DynamicHostVolumeStickyDisconnected(t *testing.T) {
	ci.Parallel(t)

	run := structs.AllocDesiredStatusRun
	running := structs.AllocClientStatusRunning
	unknown := structs.AllocClientStatusUnknown

	// unknownCopy is the plan's update of the holder, as the reconciler
	// writes it when the client disconnects.
	unknownCopy := func(holder *structs.Allocation) *structs.Allocation {
		c := holder.Copy()
		c.Job = nil
		c.ClientStatus = unknown
		return c
	}

	cases := []struct {
		name   string
		run    func(f *stickyPlanFixture) (*structs.Plan, int)
		fit    bool
		reason string
	}{
		{
			name: "the plan marks the holder unknown and places the replacement on the other volume",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				holder := f.alloc(0, 0)
				f.store(run, running, holder)
				plan := f.placing(unknownCopy(holder), f.planAlloc(1, 0))
				return plan, 1
			},
			reason: stickyReason,
		},
		{
			name: "the holder is already unknown in the snapshot",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				holder := f.alloc(0, 0)
				f.store(run, unknown, holder)
				return f.placing(f.planAlloc(1, 0)), 1
			},
			reason: stickyReason,
		},
		{
			name: "the holder is already unknown: the claimed volume is allowed",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				holder := f.alloc(0, 0)
				f.store(run, unknown, holder)
				return f.placing(f.planAlloc(0, 1)), 0
			},
			fit: true,
		},
		{
			name: "a scale-up on the other volume is refused while the holder is unknown",
			run: func(f *stickyPlanFixture) (*structs.Plan, int) {
				f.claim(0)
				holder := f.alloc(0, 0)
				f.store(run, unknown, holder)
				return f.placing(f.planAlloc(1, 1)), 1
			},
			reason: stickyReason,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newStickyPlanFixture(t)
			plan, node := tc.run(f)
			fit, reason := f.evaluate(plan, node)
			must.Eq(t, tc.fit, fit)
			must.Eq(t, tc.reason, reason)
		})
	}

	// The original keeps its claim through a reconnect: the update of the
	// stored allocation creates no claim.
	t.Run("a reconnect creates no claim", func(t *testing.T) {
		f := newStickyPlanFixture(t)
		f.claim(0)
		holder := f.alloc(0, 0)
		f.store(run, unknown, holder)
		reconnected := holder.Copy()
		reconnected.Job = nil
		reconnected.ClientStatus = running

		snap, err := f.state.Snapshot()
		must.NoError(t, err)
		plan := f.placing(reconnected)
		claims, err := stickyClaimsForPlan(snap, plan, &structs.PlanResult{NodeAllocation: plan.NodeAllocation})
		must.NoError(t, err)
		must.Len(t, 0, claims)
	})
}

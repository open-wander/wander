// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package scheduler

import (
	"fmt"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// stickySched is a harness with two nodes, each with a dynamic volume named
// data-a, and a sticky job.
type stickySched struct {
	t     *testing.T
	h     *Harness
	nodes [2]*structs.Node
	vols  [2]*structs.DynamicHostVolume
	job   *structs.Job
}

func newStickySched(t *testing.T, access structs.DynamicHostVolumeAccessMode, count int) *stickySched {
	s := &stickySched{t: t, h: NewHarness(t)}
	for i := range s.nodes {
		id := uuid.Generate()
		s.nodes[i] = dynamicHostVolumeNode(id, "data-a", "/synthetic/data-a")
		s.vols[i] = dynamicHostVolumeRecord(s.nodes[i], id, structs.DefaultNamespace, "data-a")
		s.vols[i].RequestedCapabilities = []*structs.DynamicHostVolumeCapability{{
			AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem, AccessMode: access,
		}}
		must.NoError(t, s.h.State.UpsertNode(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[i]))
		must.NoError(t, s.h.State.UpsertDynamicHostVolumes(s.h.NextIndex(), []*structs.DynamicHostVolume{s.vols[i]}))
	}
	s.job = mock.Job()
	s.job.Update = structs.UpdateStrategy{}
	s.job.TaskGroups[0].Count = count
	s.job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{"data": {
		Name: "data", Type: structs.VolumeTypeHost, Source: "data-a", Sticky: true,
		AccessMode: structs.CSIVolumeAccessMode(access),
	}}
	must.NoError(t, s.h.State.UpsertJob(structs.MsgTypeTestSetup, s.h.NextIndex(), nil, s.job))
	return s
}

func (s *stickySched) claim(i int) {
	must.NoError(s.t, s.h.State.UpsertDynamicHostVolumeClaims(s.h.NextIndex(), []*structs.DynamicHostVolumeClaim{{
		ID: uuid.Generate(), Namespace: s.job.Namespace, JobID: s.job.ID, TaskGroupName: s.job.TaskGroups[0].Name,
		VolumeName: "data-a", VolumeID: s.vols[i].ID,
	}}))
}

func (s *stickySched) alloc(i, index int, desired, client string) *structs.Allocation {
	a := mock.Alloc()
	a.Job = s.job
	a.JobID = s.job.ID
	a.TaskGroup = s.job.TaskGroups[0].Name
	a.NodeID = s.nodes[i].ID
	a.Name = structs.AllocName(s.job.ID, a.TaskGroup, uint(index))
	a.DesiredStatus = desired
	a.ClientStatus = client
	storeStickyAllocs(s.t, s.h.State, s.h.NextIndex, []*structs.Allocation{a})
	return a
}

func (s *stickySched) process() {
	s.t.Helper()
	eval := &structs.Evaluation{
		Namespace: s.job.Namespace, ID: uuid.Generate(), Priority: s.job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister, JobID: s.job.ID, Status: structs.EvalStatusPending,
	}
	must.NoError(s.t, s.h.State.UpsertEvals(structs.MsgTypeTestSetup, s.h.NextIndex(), []*structs.Evaluation{eval}))
	must.NoError(s.t, s.h.Process(NewServiceScheduler, eval))
}

// placedOn returns the nodes of the allocations of the last plan.
func (s *stickySched) placedOn() map[string]int {
	s.t.Helper()
	placed := map[string]int{}
	if len(s.h.Plans) == 0 {
		return placed
	}
	for node, allocs := range s.h.Plans[len(s.h.Plans)-1].NodeAllocation {
		placed[node] += len(allocs)
	}
	return placed
}

func (s *stickySched) failedReasons() map[string]int {
	s.t.Helper()
	must.NotEq(s.t, 0, len(s.h.Evals))
	metrics := s.h.Evals[len(s.h.Evals)-1].FailedTGAllocs[s.job.TaskGroups[0].Name]
	must.NotNil(s.t, metrics)
	return metrics.ConstraintFiltered
}

const stickyRepeats = 25

// Replacements and destructive updates go back to the claimed volume: the
// stopped allocation is released by the plan itself.
func TestServiceSched_DynamicHostVolume_StickyDestructiveUpdate(t *testing.T) {
	ci.Parallel(t)

	for range stickyRepeats {
		s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
		s.claim(1)
		s.alloc(1, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning)

		updated := s.job.Copy()
		updated.TaskGroups[0].Tasks[0].Config["command"] = "/bin/other"
		must.NoError(t, s.h.State.UpsertJob(structs.MsgTypeTestSetup, s.h.NextIndex(), nil, updated))
		s.process()

		must.Len(t, 1, s.h.Plans)
		must.Eq(t, map[string]int{s.nodes[1].ID: 1}, s.placedOn())
	}
}

// A re-run after a stop goes back to the claimed volume, whether or not the
// client has reported the stopped allocation terminal yet.
func TestServiceSched_DynamicHostVolume_StickyRerunAfterStop(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name   string
		client string
	}{
		{"stopped and complete", structs.AllocClientStatusComplete},
		{"stopped but still running", structs.AllocClientStatusRunning},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for range stickyRepeats {
				s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
				s.claim(1)
				s.alloc(1, 0, structs.AllocDesiredStatusStop, tc.client)
				s.process()
				must.Eq(t, map[string]int{s.nodes[1].ID: 1}, s.placedOn())
			}
		})
	}

	// With an exclusive writer the re-run waits for the old allocation to end
	// rather than taking the other volume.
	t.Run("exclusive writer waits for the claimed volume", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeSingleWriter, 1)
			s.claim(1)
			s.alloc(1, 0, structs.AllocDesiredStatusStop, structs.AllocClientStatusRunning)
			s.process()
			must.Len(t, 0, s.h.Plans)
			must.MapContainsKey(t, s.failedReasons(), fmt.Sprintf(FilterConstraintDynamicHostVolumeWriterTemplate, "data-a"))
		}
	})
}

// The node of the claimed volume is ineligible: the group is not placed on
// the other volume of the same name, and is placed on the claimed one later.
func TestServiceSched_DynamicHostVolume_StickyClaimedNodeIneligible(t *testing.T) {
	ci.Parallel(t)

	for range stickyRepeats {
		s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
		s.claim(0)
		s.alloc(0, 0, structs.AllocDesiredStatusStop, structs.AllocClientStatusComplete)
		must.NoError(t, s.h.State.UpdateNodeEligibility(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeSchedulingIneligible, 0, nil))

		s.process()
		must.Len(t, 0, s.h.Plans)
		must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
		for reason := range s.failedReasons() {
			must.StrNotContains(t, reason, s.vols[0].ID)
			must.StrNotContains(t, reason, s.vols[1].ID)
		}

		must.NoError(t, s.h.State.UpdateNodeEligibility(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeSchedulingEligible, 0, nil))
		s.h.Plans, s.h.Evals = nil, nil
		s.process()
		must.Eq(t, map[string]int{s.nodes[0].ID: 1}, s.placedOn())
	}
}

// The node of the claimed volume is down or gone from state: the volume
// record and the claim stay, so the group is not placed on the other volume of
// the same name, and it returns to the claimed volume when the node is back.
// The ineligible case above covers a node that is still known and ready to
// run what it has; these cover one that cannot be placed on at all.
func TestServiceSched_DynamicHostVolume_StickyClaimedNodeDownOrMissing(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name string
		take func(s *stickySched)
		back func(s *stickySched)
	}{
		{
			"down",
			func(s *stickySched) {
				must.NoError(t, s.h.State.UpdateNodeStatus(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeStatusDown, 0, nil))
			},
			func(s *stickySched) {
				must.NoError(t, s.h.State.UpdateNodeStatus(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeStatusReady, 0, nil))
			},
		},
		{
			"missing",
			func(s *stickySched) {
				must.NoError(t, s.h.State.DeleteNode(structs.MsgTypeTestSetup, s.h.NextIndex(), []string{s.nodes[0].ID}))
			},
			func(s *stickySched) {
				must.NoError(t, s.h.State.UpsertNode(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0]))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for range stickyRepeats {
				s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
				s.claim(0)
				s.alloc(0, 0, structs.AllocDesiredStatusStop, structs.AllocClientStatusLost)
				tc.take(s)

				s.process()
				must.Len(t, 0, s.h.Plans)
				must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
				for reason := range s.failedReasons() {
					must.StrNotContains(t, reason, s.vols[0].ID)
					must.StrNotContains(t, reason, s.vols[1].ID)
				}

				tc.back(s)
				s.h.Plans, s.h.Evals = nil, nil
				s.process()
				must.Eq(t, map[string]int{s.nodes[0].ID: 1}, s.placedOn())
			}
		})
	}
}

// A group that is not sticky and asks for a name that exists only on a down
// node is not placed, and is placed once the node is back.
func TestServiceSched_DynamicHostVolume_NonStickyNameOnlyOnDownNode(t *testing.T) {
	ci.Parallel(t)

	for range stickyRepeats {
		s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
		// The second node no longer has the volume.
		bare := s.nodes[1].Copy()
		bare.HostVolumes = nil
		must.NoError(t, s.h.State.UpsertNode(structs.MsgTypeTestSetup, s.h.NextIndex(), bare))
		stored, err := s.h.State.DynamicHostVolumeByID(nil, s.vols[1].Namespace, s.vols[1].ID)
		must.NoError(t, err)
		must.NoError(t, s.h.State.DeleteDynamicHostVolumes(s.h.NextIndex(), []state.DynamicHostVolumeDelete{
			{Namespace: stored.Namespace, ID: stored.ID, ExpectedModifyIndex: stored.ModifyIndex}}))
		plain := s.job.Copy()
		plain.TaskGroups[0].Volumes["data"].Sticky = false
		must.NoError(t, s.h.State.UpsertJob(structs.MsgTypeTestSetup, s.h.NextIndex(), nil, plain))
		must.NoError(t, s.h.State.UpdateNodeStatus(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeStatusDown, 0, nil))

		s.process()
		must.Len(t, 0, s.h.Plans)
		must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)

		must.NoError(t, s.h.State.UpdateNodeStatus(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeStatusReady, 0, nil))
		s.h.Plans, s.h.Evals = nil, nil
		s.process()
		must.Eq(t, map[string]int{s.nodes[0].ID: 1}, s.placedOn())
	}
}

// Count 2 on two claimed volumes, one node ineligible and its allocation
// stopped: the replacement is not placed on the other claimed volume.
func TestServiceSched_DynamicHostVolume_StickyReplacementNotOnOtherClaimedVolume(t *testing.T) {
	ci.Parallel(t)

	for range stickyRepeats {
		s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 2)
		s.claim(0)
		s.claim(1)
		s.alloc(0, 0, structs.AllocDesiredStatusStop, structs.AllocClientStatusLost)
		s.alloc(1, 1, structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning)
		must.NoError(t, s.h.State.UpdateNodeEligibility(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[0].ID, structs.NodeSchedulingIneligible, 0, nil))

		s.process()
		must.Len(t, 0, s.h.Plans)
		must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
	}
}

// Scaling up with no free claim places on any volume, and a fresh job with
// two volumes may place both allocations anywhere.
func TestServiceSched_DynamicHostVolume_StickyScaleAndFresh(t *testing.T) {
	ci.Parallel(t)

	t.Run("scale 1 to 3", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 3)
			s.claim(0)
			s.alloc(0, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning)
			s.process()
			must.Len(t, 1, s.h.Plans)
			total := 0
			for _, n := range s.placedOn() {
				total += n
			}
			must.Eq(t, 2, total)
			must.MapLen(t, 0, s.h.Evals[0].FailedTGAllocs)
		}
	})

	t.Run("fresh count 2 with no claims", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 2)
			s.process()
			total := 0
			for _, n := range s.placedOn() {
				total += n
			}
			must.Eq(t, 2, total)
			must.MapLen(t, 0, s.h.Evals[0].FailedTGAllocs)
		}
	})
}

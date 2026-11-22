// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package scheduler

import (
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// withMaxClientDisconnect stores a version of the job that tolerates a
// disconnected client and makes it the job the harness allocations run.
func (s *stickySched) withMaxClientDisconnect() {
	s.t.Helper()
	d := 10 * time.Minute
	updated := s.job.Copy()
	updated.TaskGroups[0].MaxClientDisconnect = &d
	// No delay, so the replacement of a disconnecting allocation is made in
	// the evaluation that sees the disconnect.
	updated.TaskGroups[0].ReschedulePolicy = &structs.ReschedulePolicy{
		Unlimited: true, Delay: 0, DelayFunction: "constant",
	}
	must.NoError(s.t, s.h.State.UpsertJob(structs.MsgTypeTestSetup, s.h.NextIndex(), nil, updated))
	stored, err := s.h.State.JobByID(nil, updated.Namespace, updated.ID)
	must.NoError(s.t, err)
	s.job = stored
}

func (s *stickySched) setNodeStatus(i int, status string) {
	s.t.Helper()
	must.NoError(s.t, s.h.State.UpdateNodeStatus(structs.MsgTypeTestSetup, s.h.NextIndex(), s.nodes[i].ID, status, 0, nil))
}

// scaleTo stores a version of the job with the given count.
func (s *stickySched) scaleTo(count int) {
	s.t.Helper()
	updated := s.job.Copy()
	updated.TaskGroups[0].Count = count
	must.NoError(s.t, s.h.State.UpsertJob(structs.MsgTypeTestSetup, s.h.NextIndex(), nil, updated))
	stored, err := s.h.State.JobByID(nil, updated.Namespace, updated.ID)
	must.NoError(s.t, err)
	s.job = stored
}

func (s *stickySched) processNodeUpdate(i int) {
	s.t.Helper()
	s.processEval(&structs.Evaluation{
		Namespace: s.job.Namespace, ID: uuid.Generate(), Priority: s.job.Priority,
		TriggeredBy: structs.EvalTriggerNodeUpdate, JobID: s.job.ID, NodeID: s.nodes[i].ID,
		Status: structs.EvalStatusPending,
	})
}

func (s *stickySched) processEval(eval *structs.Evaluation) {
	s.t.Helper()
	must.NoError(s.t, s.h.State.UpsertEvals(structs.MsgTypeTestSetup, s.h.NextIndex(), []*structs.Evaluation{eval}))
	must.NoError(s.t, s.h.Process(NewServiceScheduler, eval))
}

// unknownUpdates returns the allocations of the last plan that the plan
// sends to the node in client status unknown.
func (s *stickySched) unknownUpdates(i int) []*structs.Allocation {
	s.t.Helper()
	var out []*structs.Allocation
	if len(s.h.Plans) == 0 {
		return out
	}
	for _, a := range s.h.Plans[len(s.h.Plans)-1].NodeAllocation[s.nodes[i].ID] {
		if a.ClientStatus == structs.AllocClientStatusUnknown {
			out = append(out, a)
		}
	}
	return out
}

func (s *stickySched) blocked() bool {
	for _, e := range s.h.CreateEvals {
		if e.Status == structs.EvalStatusBlocked {
			return true
		}
	}
	return false
}

// The claim of a sticky group stays with its volume while the client is
// disconnected: the replacement of the unknown allocation waits for the
// volume and is not placed on the other volume of the name.
func TestServiceSched_DynamicHostVolume_StickyDisconnectedClientReplacementWaits(t *testing.T) {
	ci.Parallel(t)

	t.Run("the evaluation that disconnects the client", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
			s.withMaxClientDisconnect()
			s.claim(0)
			a := s.alloc(0, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning)
			s.setNodeStatus(0, structs.NodeStatusDisconnected)

			s.processNodeUpdate(0)
			must.Len(t, 1, s.h.Plans)
			must.Eq(t, 0, s.placedOn()[s.nodes[1].ID])
			updates := s.unknownUpdates(0)
			must.Len(t, 1, updates)
			must.Eq(t, a.ID, updates[0].ID)
			must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
			// The evaluation creates the follow-up evaluation for the end of
			// max_client_disconnect, so it delays instead of blocking.
			must.False(t, s.blocked())
		}
	})

	// The default reschedule delay makes the replacement wait for the
	// follow-up evaluation, which finds the allocation stored as unknown.
	t.Run("the follow-up evaluation with the allocation stored as unknown", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
			s.withMaxClientDisconnect()
			s.claim(0)
			eval := &structs.Evaluation{
				Namespace: s.job.Namespace, ID: uuid.Generate(), Priority: s.job.Priority,
				TriggeredBy: structs.EvalTriggerRetryFailedAlloc, JobID: s.job.ID, Status: structs.EvalStatusPending,
			}
			a := s.alloc(0, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusUnknown)
			a.FollowupEvalID = eval.ID
			a.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
			must.NoError(t, s.h.State.UpsertAllocs(structs.MsgTypeTestSetup, s.h.NextIndex(), []*structs.Allocation{a}))
			s.setNodeStatus(0, structs.NodeStatusDisconnected)

			s.processEval(eval)
			must.Eq(t, 0, s.placedOn()[s.nodes[1].ID])
			must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
			must.True(t, s.blocked())
		}
	})

	// Behaviour change pinned on purpose: while the claim is free, every
	// placement of the group is forced onto its volume, so a scale-up waits
	// as well.
	t.Run("a scale-up waits for the volume of the disconnected client", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
			s.withMaxClientDisconnect()
			s.claim(0)
			s.alloc(0, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusUnknown)
			s.setNodeStatus(0, structs.NodeStatusDisconnected)
			s.scaleTo(2)

			s.process()
			must.Eq(t, 0, s.placedOn()[s.nodes[1].ID])
			must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
			must.True(t, s.blocked())
		}
	})

	// Without max_client_disconnect the allocation is lost, which is
	// client-terminal, so the claim was already free.
	t.Run("without max_client_disconnect the replacement waits", func(t *testing.T) {
		for range stickyRepeats {
			s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
			s.claim(0)
			s.alloc(0, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusRunning)
			s.setNodeStatus(0, structs.NodeStatusDisconnected)

			s.processNodeUpdate(0)
			must.Eq(t, 0, s.placedOn()[s.nodes[1].ID])
			must.MapContainsKey(t, s.failedReasons(), FilterConstraintHostVolumes)
			must.True(t, s.blocked())
		}
	})
}

// When the client is back and has not yet reported, the allocation is still
// unknown. The evaluation keeps the original and places nothing: its claim
// exists, so no second claim is made either.
func TestServiceSched_DynamicHostVolume_StickyReconnectKeepsOriginal(t *testing.T) {
	ci.Parallel(t)

	for range stickyRepeats {
		s := newStickySched(t, structs.DynamicHostVolumeAccessModeSingleNodeWriter, 1)
		s.withMaxClientDisconnect()
		s.claim(0)
		a := s.alloc(0, 0, structs.AllocDesiredStatusRun, structs.AllocClientStatusUnknown)
		a.AppendState(structs.AllocStateFieldClientStatus, structs.AllocClientStatusUnknown)
		storeStickyAllocs(t, s.h.State, s.h.NextIndex, []*structs.Allocation{a})

		s.processNodeUpdate(0)
		must.Eq(t, 0, s.placedOn()[s.nodes[1].ID])
		must.False(t, s.blocked())
		// The group needs no placement: the original counts towards its count.
		must.NotEq(t, 0, len(s.h.Evals))
		must.MapEmpty(t, s.h.Evals[len(s.h.Evals)-1].FailedTGAllocs)
		for _, p := range s.h.Plans {
			for _, placed := range p.NodeAllocation {
				for _, pa := range placed {
					must.Eq(t, a.ID, pa.ID)
				}
			}
			for _, stopped := range p.NodeUpdate {
				for _, sa := range stopped {
					must.NotEq(t, a.ID, sa.ID)
				}
			}
		}
	}
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package scheduler

import (
	"fmt"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// preemptionWriterNode is a node with a dynamic host volume that admits a
// single writer. The sizes in these tests leave room for one large allocation,
// so that a second large placement has to preempt the first.
func preemptionWriterNode(t *testing.T, h *Harness) *structs.Node {
	t.Helper()

	id := uuid.Generate()
	node := dynamicHostVolumeNode(id, "data-a", "/synthetic/data-a")
	must.NoError(t, h.State.UpsertNode(structs.MsgTypeTestSetup, h.NextIndex(), node))
	vol := dynamicHostVolumeRecord(node, id, structs.DefaultNamespace, "data-a")
	vol.RequestedCapabilities = []*structs.DynamicHostVolumeCapability{{
		AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem,
		AccessMode:     structs.DynamicHostVolumeAccessModeSingleNodeSingleWriter,
	}}
	must.NoError(t, h.State.UpsertDynamicHostVolumes(h.NextIndex(), []*structs.DynamicHostVolume{vol}))
	return node
}

func enableServicePreemption(t *testing.T, h *Harness) {
	t.Helper()
	must.NoError(t, h.State.SchedulerSetConfig(h.NextIndex(), &structs.SchedulerConfiguration{
		PreemptionConfig: structs.PreemptionConfig{ServiceSchedulerEnabled: true},
	}))
}

func sizeGroup(tg *structs.TaskGroup, cpu, mem int) {
	tg.Networks = nil
	tg.Tasks[0].Resources.Networks = nil
	tg.Tasks[0].Resources.CPU = cpu
	tg.Tasks[0].Resources.MemoryMB = mem
}

func processRegister(t *testing.T, h *Harness, job *structs.Job) {
	t.Helper()
	must.NoError(t, h.State.UpsertJob(structs.MsgTypeTestSetup, h.NextIndex(), nil, job))
	eval := &structs.Evaluation{
		Namespace: job.Namespace, ID: uuid.Generate(), Priority: job.Priority,
		TriggeredBy: structs.EvalTriggerJobRegister, JobID: job.ID, Status: structs.EvalStatusPending,
	}
	must.NoError(t, h.State.UpsertEvals(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Evaluation{eval}))
	must.NoError(t, h.Process(NewServiceScheduler, eval))
}

// placeHolder runs a low-priority writer through the scheduler and marks its
// allocation running, so that it holds the volume.
func placeHolder(t *testing.T, h *Harness, node *structs.Node) *structs.Allocation {
	t.Helper()

	job := dynamicWriterJob(1)
	job.Priority = 30
	sizeGroup(job.TaskGroups[0], 2500, 4000)
	processRegister(t, h, job)

	must.Len(t, 1, h.Plans)
	must.Len(t, 1, h.Plans[0].NodeAllocation[node.ID])
	holder := h.Plans[0].NodeAllocation[node.ID][0].Copy()
	holder.ClientStatus = structs.AllocClientStatusRunning
	must.NoError(t, h.State.UpsertAllocs(structs.MsgTypeTestSetup, h.NextIndex(), []*structs.Allocation{holder}))
	return holder
}

// A plan that preempts the holder for a placement that does not use the volume
// must not then place a writer beside the holder, which has not stopped yet.
func TestServiceSched_DynamicHostVolume_WriterNotPlacedBesidePreemptedHolder(t *testing.T) {
	ci.Parallel(t)

	// The scheduler places the groups in no fixed order. Only the order in
	// which the group that preempts goes first used to let the writer through,
	// so the scenario runs repeatedly.
	for i := range 50 {
		t.Run(fmt.Sprintf("run-%d", i), func(t *testing.T) {
			h := NewHarness(t)
			node := preemptionWriterNode(t, h)
			enableServicePreemption(t, h)
			holder := placeHolder(t, h, node)

			// One group needs the room the holder uses and does not touch the
			// volume. The other asks for the volume and fits beside it.
			job := dynamicWriterJob(1)
			job.Priority = 100
			job.TaskGroups[0].Name = "writer"
			sizeGroup(job.TaskGroups[0], 100, 100)
			big := job.TaskGroups[0].Copy()
			big.Name = "big"
			big.Volumes = nil
			sizeGroup(big, 2000, 5000)
			job.TaskGroups = append(job.TaskGroups, big)
			processRegister(t, h, job)

			must.Len(t, 2, h.Plans)
			plan := h.Plans[1]
			for _, alloc := range plan.NodeAllocation[node.ID] {
				must.NotEq(t, "writer", alloc.TaskGroup, must.Sprint("a writer was placed while the holder still runs"))
			}

			// The writer waits.
			last := h.Evals[len(h.Evals)-1]
			must.NotNil(t, last.FailedTGAllocs["writer"])

			// The holder is only ever preempted for the group that does not
			// use the volume.
			must.Len(t, 1, plan.NodePreemptions[node.ID])
			must.Eq(t, holder.ID, plan.NodePreemptions[node.ID][0].ID)
		})
	}
}

// A writer that would need the holder preempted for its resources is not
// placed, and the holder is left alone.
func TestServiceSched_DynamicHostVolume_HolderNotPreemptedForWriter(t *testing.T) {
	ci.Parallel(t)

	h := NewHarness(t)
	node := preemptionWriterNode(t, h)
	enableServicePreemption(t, h)
	placeHolder(t, h, node)

	job := dynamicWriterJob(1)
	job.Priority = 100
	sizeGroup(job.TaskGroups[0], 2000, 5000)
	processRegister(t, h, job)

	// A placement that finds no node submits no plan, so the only plan is the
	// holder's own.
	must.Len(t, 1, h.Plans)
	must.MapLen(t, 0, h.Plans[0].NodePreemptions)
	last := h.Evals[len(h.Evals)-1]
	must.NotNil(t, last.FailedTGAllocs[job.TaskGroups[0].Name])
}

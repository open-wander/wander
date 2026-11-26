// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// applyDynamicHostVolume encodes the request and applies it as the log entry at
// the index, as Raft would.
func applyDynamicHostVolume(t *testing.T, fsm *nomadFSM, index uint64, msgType structs.MessageType, req any) any {
	t.Helper()
	buf, err := structs.Encode(msgType, req)
	must.NoError(t, err)
	return fsm.Apply(&raft.Log{Index: index, Term: 1, Type: raft.LogCommand, Data: buf})
}

func dhvFSMVolume(id string) *structs.DynamicHostVolume {
	return &structs.DynamicHostVolume{
		ID: id, Name: "data-" + id, Namespace: "default", Type: structs.VolumeTypeHost,
		PluginID: "plugin-a", State: structs.DynamicHostVolumeStateRequested,
		Origin:                    structs.DynamicHostVolumeOriginPluginCreated,
		RequestedCapacityMinBytes: 1, RequestedCapacityMaxBytes: 100,
		RequestedCapabilities: []*structs.DynamicHostVolumeCapability{{
			AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem,
			AccessMode:     structs.DynamicHostVolumeAccessModeSingleNodeWriter}},
		CreateTime: 100, ModifyTime: 100,
	}
}

func dhvFSMKey(id string) structs.DynamicHostVolumeKey {
	return structs.DynamicHostVolumeKey{Namespace: "default", ID: id}
}

func dhvFSMGet(t *testing.T, fsm *nomadFSM, id string) *structs.DynamicHostVolume {
	t.Helper()
	v, err := fsm.State().DynamicHostVolumeByID(nil, "default", id)
	must.NoError(t, err)
	return v
}

func TestFSM_DynamicHostVolume_Lifecycle(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)

	// Upsert creates the volume as requested.
	resp := applyDynamicHostVolume(t, fsm, 10, structs.DynamicHostVolumeUpsertRequestType,
		&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{dhvFSMVolume("volume-a")}})
	must.Nil(t, resp)
	got := dhvFSMGet(t, fsm, "volume-a")
	must.Eq(t, structs.DynamicHostVolumeStateRequested, got.State)
	must.Eq(t, uint64(10), got.CreateIndex)

	// A second upsert gives it a node: requested to pending.
	pending := dhvFSMVolume("volume-a")
	pending.NodeID, pending.State = "node-a", structs.DynamicHostVolumeStatePending
	pending.ModifyIndex, pending.CreateIndex = got.ModifyIndex, got.CreateIndex
	must.Nil(t, applyDynamicHostVolume(t, fsm, 11, structs.DynamicHostVolumeUpsertRequestType,
		&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{pending}}))
	must.Eq(t, structs.DynamicHostVolumeStatePending, dhvFSMGet(t, fsm, "volume-a").State)

	// The client outcome makes it ready.
	must.Nil(t, applyDynamicHostVolume(t, fsm, 12, structs.DynamicHostVolumeOutcomeRequestType,
		&structs.DynamicHostVolumeOutcomeRequest{Key: dhvFSMKey("volume-a"), State: structs.DynamicHostVolumeStateReady,
			HostPath: "/srv/wander-synthetic/a", CapacityBytes: 64, ModifyTime: 200}))
	got = dhvFSMGet(t, fsm, "volume-a")
	must.Eq(t, structs.DynamicHostVolumeStateReady, got.State)
	must.Eq(t, "/srv/wander-synthetic/a", got.HostPath)
	must.Eq(t, int64(64), got.CapacityBytes)

	// Update with CAS on the modify index.
	next := got.Copy()
	next.Parameters = map[string]string{"synthetic-key": "changed"}
	must.Nil(t, applyDynamicHostVolume(t, fsm, 13, structs.DynamicHostVolumeUpdateRequestType,
		&structs.DynamicHostVolumeUpdateRequest{Volume: next}))
	must.Eq(t, "changed", dhvFSMGet(t, fsm, "volume-a").Parameters["synthetic-key"])
	// The same update again carries a stale index.
	stale := applyDynamicHostVolume(t, fsm, 14, structs.DynamicHostVolumeUpdateRequestType,
		&structs.DynamicHostVolumeUpdateRequest{Volume: next})
	must.ErrorIs(t, stale.(error), state.ErrDynamicHostVolumeStateCAS)

	// A sticky claim goes with the volume.
	must.NoError(t, fsm.State().UpsertDynamicHostVolumeClaims(15, []*structs.DynamicHostVolumeClaim{
		{ID: "claim-a", Namespace: "default", JobID: "job-a", TaskGroupName: "group-a", VolumeName: "data-volume-a", VolumeID: "volume-a"}}))

	// Removal before marking is refused and names the state.
	refused := applyDynamicHostVolume(t, fsm, 16, structs.DynamicHostVolumeRemoveRequestType,
		&structs.DynamicHostVolumeRemoveRequest{Keys: []structs.DynamicHostVolumeKey{dhvFSMKey("volume-a")}})
	must.ErrorIs(t, refused.(error), state.ErrDynamicHostVolumeStateTransition)
	must.StrContains(t, refused.(error).Error(), `current state "ready"`)

	must.Nil(t, applyDynamicHostVolume(t, fsm, 17, structs.DynamicHostVolumeMarkDeletingRequestType,
		&structs.DynamicHostVolumeMarkDeletingRequest{Keys: []structs.DynamicHostVolumeKey{dhvFSMKey("volume-a")}, ModifyTime: 300}))
	must.Eq(t, structs.DynamicHostVolumeStateDeleting, dhvFSMGet(t, fsm, "volume-a").State)

	must.Nil(t, applyDynamicHostVolume(t, fsm, 18, structs.DynamicHostVolumeRemoveRequestType,
		&structs.DynamicHostVolumeRemoveRequest{Keys: []structs.DynamicHostVolumeKey{dhvFSMKey("volume-a")}}))
	must.Nil(t, dhvFSMGet(t, fsm, "volume-a"))
	claim, err := fsm.State().DynamicHostVolumeClaimByID(nil, "default", "claim-a")
	must.NoError(t, err)
	must.Nil(t, claim)
	claimIndex, err := fsm.State().Index(state.TableDynamicHostVolumeClaims)
	must.NoError(t, err)
	must.Eq(t, uint64(18), claimIndex)
}

func TestFSM_DynamicHostVolume_RefusedTransitions(t *testing.T) {
	ci.Parallel(t)
	cases := []struct {
		name    string
		msgType structs.MessageType
		req     any
		state   string
	}{
		{"outcome ready of a requested volume", structs.DynamicHostVolumeOutcomeRequestType,
			&structs.DynamicHostVolumeOutcomeRequest{Key: dhvFSMKey("volume-a"), State: structs.DynamicHostVolumeStateReady,
				HostPath: "/srv/wander-synthetic/a", CapacityBytes: 1}, "requested"},
		{"update of a requested volume", structs.DynamicHostVolumeUpdateRequestType,
			&structs.DynamicHostVolumeUpdateRequest{Volume: func() *structs.DynamicHostVolume {
				v := dhvFSMVolume("volume-a")
				v.ModifyIndex = 10
				return v
			}()}, "requested"},
		{"remove of a requested volume", structs.DynamicHostVolumeRemoveRequestType,
			&structs.DynamicHostVolumeRemoveRequest{Keys: []structs.DynamicHostVolumeKey{dhvFSMKey("volume-a")}}, "requested"},
		{"changed repeat of a create", structs.DynamicHostVolumeUpsertRequestType,
			&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{func() *structs.DynamicHostVolume {
				v := dhvFSMVolume("volume-a")
				v.Parameters = map[string]string{"synthetic-key": "other"}
				return v
			}()}}, "requested"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsm := testFSM(t)
			must.Nil(t, applyDynamicHostVolume(t, fsm, 10, structs.DynamicHostVolumeUpsertRequestType,
				&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{dhvFSMVolume("volume-a")}}))
			before := dhvFSMGet(t, fsm, "volume-a")
			resp := applyDynamicHostVolume(t, fsm, 11, tc.msgType, tc.req)
			err, ok := resp.(error)
			must.True(t, ok, must.Sprint("the apply must return the refusal"))
			must.ErrorIs(t, err, state.ErrDynamicHostVolumeStateTransition)
			must.StrContains(t, err.Error(), fmt.Sprintf("current state %q", tc.state))
			must.True(t, before.Equal(dhvFSMGet(t, fsm, "volume-a")))
		})
	}
}

func TestFSM_DynamicHostVolume_IdempotentRepeat(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	create := &structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{dhvFSMVolume("volume-a")}}
	must.Nil(t, applyDynamicHostVolume(t, fsm, 10, structs.DynamicHostVolumeUpsertRequestType, create))
	before := dhvFSMGet(t, fsm, "volume-a")

	// A duplicate of the same entry arrives later with new timestamps.
	repeat := dhvFSMVolume("volume-a")
	repeat.CreateTime, repeat.ModifyTime = 900, 900
	must.Nil(t, applyDynamicHostVolume(t, fsm, 11, structs.DynamicHostVolumeUpsertRequestType,
		&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{repeat}}))
	must.True(t, before.Equal(dhvFSMGet(t, fsm, "volume-a")))
}

func TestFSM_SnapshotRestore_DynamicHostVolumeStatusDescription(t *testing.T) {
	ci.Parallel(t)
	fsm := testFSM(t)
	must.Nil(t, applyDynamicHostVolume(t, fsm, 10, structs.DynamicHostVolumeUpsertRequestType,
		&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{dhvFSMVolume("volume-a")}}))
	must.Nil(t, applyDynamicHostVolume(t, fsm, 11, structs.DynamicHostVolumeOutcomeRequestType,
		&structs.DynamicHostVolumeOutcomeRequest{Key: dhvFSMKey("volume-a"), State: structs.DynamicHostVolumeStateFailed,
			Diagnostic: "no node satisfied the constraints", ModifyTime: 200}))
	want := dhvFSMGet(t, fsm, "volume-a")
	must.Eq(t, "no node satisfied the constraints", want.StatusDescription)

	restored := testSnapshotRestore(t, fsm)
	got, err := restored.State().DynamicHostVolumeByID(nil, "default", "volume-a")
	must.NoError(t, err)
	must.Eq(t, want, got)
	must.Eq(t, "no node satisfied the constraints", got.StatusDescription)
	must.Eq(t, structs.DynamicHostVolumeStateFailed, got.State)
}

func TestFSM_DynamicHostVolume_UpdateKeepsServerOwnedFields(t *testing.T) {
	ci.Parallel(t)
	for name, mutate := range map[string]func(*structs.DynamicHostVolume){
		"host path":          func(v *structs.DynamicHostVolume) { v.HostPath = "/srv/wander-synthetic/other" },
		"capacity":           func(v *structs.DynamicHostVolume) { v.CapacityBytes = 999 },
		"status description": func(v *structs.DynamicHostVolume) { v.StatusDescription = "forged" },
		"attachments": func(v *structs.DynamicHostVolume) {
			v.Attachments = []*structs.DynamicHostVolumeAttachment{{AllocationID: "alloc-a", NodeID: "node-a"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			fsm := testFSM(t)
			pending := dhvFSMVolume("volume-a")
			pending.NodeID, pending.State = "node-a", structs.DynamicHostVolumeStatePending
			must.Nil(t, applyDynamicHostVolume(t, fsm, 10, structs.DynamicHostVolumeUpsertRequestType,
				&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{pending}}))
			must.Nil(t, applyDynamicHostVolume(t, fsm, 11, structs.DynamicHostVolumeOutcomeRequestType,
				&structs.DynamicHostVolumeOutcomeRequest{Key: dhvFSMKey("volume-a"), State: structs.DynamicHostVolumeStateReady,
					HostPath: "/srv/wander-synthetic/a", CapacityBytes: 64, ModifyTime: 200}))
			before := dhvFSMGet(t, fsm, "volume-a")
			next := before.Copy()
			mutate(next)
			resp := applyDynamicHostVolume(t, fsm, 12, structs.DynamicHostVolumeUpdateRequestType,
				&structs.DynamicHostVolumeUpdateRequest{Volume: next})
			err, isErr := resp.(error)
			must.True(t, isErr)
			must.ErrorIs(t, err, structs.ErrDynamicHostVolumeImmutable)
			must.True(t, before.Equal(dhvFSMGet(t, fsm, "volume-a")))
		})
	}
}

// dhvNodeFSMSequence applies node updates that move volumes between ready and
// unavailable, after seeding the volumes through the Raft commands, and returns
// the resulting records.
func dhvNodeFSMSequence(t *testing.T, fsm *nomadFSM) map[string]*structs.DynamicHostVolume {
	t.Helper()
	nodeA, nodeB := "aaaaaaaa-0000-0000-0000-000000000001", "bbbbbbbb-0000-0000-0000-000000000002"
	register := func(index uint64, id string, status string, advertised ...string) {
		node := mock.Node()
		node.ID, node.Status = id, status
		node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{}
		for _, vol := range advertised {
			node.HostVolumes["data-"+vol] = &structs.ClientHostVolumeConfig{Name: "data-" + vol, Path: "/srv/wander-synthetic/" + vol, ID: vol}
		}
		must.Nil(t, applyDynamicHostVolume(t, fsm, index, structs.NodeRegisterRequestType, &structs.NodeRegisterRequest{Node: node}))
	}
	register(5, nodeA, structs.NodeStatusReady, "volume-a", "volume-b")
	register(6, nodeB, structs.NodeStatusReady, "volume-c")

	// Seed each volume through the Raft commands, as the real flow does.
	index := uint64(10)
	for _, spec := range []struct{ id, node string }{{"volume-a", nodeA}, {"volume-b", nodeA}, {"volume-c", nodeB}} {
		vol := dhvFSMVolume(spec.id)
		vol.NodeID, vol.State = spec.node, structs.DynamicHostVolumeStatePending
		must.Nil(t, applyDynamicHostVolume(t, fsm, index, structs.DynamicHostVolumeUpsertRequestType,
			&structs.DynamicHostVolumeUpsertRequest{Volumes: []*structs.DynamicHostVolume{vol}}))
		must.Nil(t, applyDynamicHostVolume(t, fsm, index+1, structs.DynamicHostVolumeOutcomeRequestType,
			&structs.DynamicHostVolumeOutcomeRequest{Key: dhvFSMKey(spec.id), State: structs.DynamicHostVolumeStateReady,
				HostPath: "/srv/wander-synthetic/" + spec.id, CapacityBytes: 64, ModifyTime: 200}))
		index += 2
	}

	// volume-a is no longer listed: unavailable at 30.
	register(30, nodeA, structs.NodeStatusReady, "volume-b")
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, dhvFSMGet(t, fsm, "volume-a").State)
	must.Eq(t, uint64(30), dhvFSMGet(t, fsm, "volume-a").ModifyIndex)
	// Listed again: ready at 31.
	register(31, nodeA, structs.NodeStatusReady, "volume-a", "volume-b")
	must.Eq(t, structs.DynamicHostVolumeStateReady, dhvFSMGet(t, fsm, "volume-a").State)
	// Down at 32: both volumes of node A become unavailable.
	must.Nil(t, applyDynamicHostVolume(t, fsm, 32, structs.NodeUpdateStatusRequestType,
		&structs.NodeUpdateStatusRequest{NodeID: nodeA, Status: structs.NodeStatusDown, UpdatedAt: 1}))
	must.Eq(t, uint64(32), dhvFSMGet(t, fsm, "volume-b").ModifyIndex)
	// Node B is deleted at 33.
	must.Nil(t, applyDynamicHostVolume(t, fsm, 33, structs.NodeDeregisterRequestType,
		&structs.NodeDeregisterRequest{NodeID: nodeB}))
	got := dhvFSMGet(t, fsm, "volume-c")
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, got.State)
	must.Eq(t, uint64(33), got.ModifyIndex)

	// A batch deregister reaches the same path.
	register(34, nodeB, structs.NodeStatusReady, "volume-c")
	must.Eq(t, structs.DynamicHostVolumeStateReady, dhvFSMGet(t, fsm, "volume-c").State)
	must.Nil(t, applyDynamicHostVolume(t, fsm, 35, structs.NodeBatchDeregisterRequestType,
		&structs.NodeBatchDeregisterRequest{NodeIDs: []string{nodeB}}))
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, dhvFSMGet(t, fsm, "volume-c").State)
	tableIndex, err := fsm.State().Index(state.TableDynamicHostVolumes)
	must.NoError(t, err)
	must.Eq(t, uint64(35), tableIndex)

	out := map[string]*structs.DynamicHostVolume{}
	for _, id := range []string{"volume-a", "volume-b", "volume-c"} {
		out[id] = dhvFSMGet(t, fsm, id)
	}
	return out
}

func TestFSM_DynamicHostVolume_NodeUpdatesAreDeterministic(t *testing.T) {
	ci.Parallel(t)
	first := dhvNodeFSMSequence(t, testFSM(t))
	second := dhvNodeFSMSequence(t, testFSM(t))
	must.Eq(t, first, second)
	for _, vol := range first {
		must.Eq(t, structs.DynamicHostVolumeStateUnavailable, vol.State)
	}
}

// dhvClaimFSMSetup stores a volume, a claim on it and, when live is set, a
// running allocation of the claim's group, through the FSM state.
func dhvClaimFSMSetup(t *testing.T, fsm *nomadFSM, live bool) *structs.DynamicHostVolumeClaim {
	t.Helper()
	st := fsm.State()
	must.NoError(t, st.UpsertNamespaces(5, []*structs.Namespace{{Name: "default"}}))
	vol := dhvFSMVolume("volume-a")
	vol.NodeID = "00000000-0000-0000-0000-00000000000a"
	vol.State = structs.DynamicHostVolumeStateReady
	must.NoError(t, st.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{vol}))

	// A fixed job ID keeps the claim rows equal across separately seeded FSMs.
	job := mock.Job()
	job.ID = "job-a"
	job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"data": {Name: "data", Type: structs.VolumeTypeHost, Source: vol.Name}}
	must.NoError(t, st.UpsertJob(structs.MsgTypeTestSetup, 11, nil, job))
	claim := &structs.DynamicHostVolumeClaim{ID: "claim-a", Namespace: "default", JobID: job.ID,
		TaskGroupName: job.TaskGroups[0].Name, VolumeName: vol.Name, VolumeID: vol.ID,
		AllocationID: "00000000-0000-0000-0000-0000000000c1", AllocationIndex: 1}
	must.NoError(t, st.UpsertDynamicHostVolumeClaims(12, []*structs.DynamicHostVolumeClaim{claim}))
	if live {
		alloc := mock.Alloc()
		alloc.Job, alloc.JobID, alloc.Namespace = job, job.ID, job.Namespace
		alloc.TaskGroup = job.TaskGroups[0].Name
		alloc.NodeID = vol.NodeID
		alloc.Name = job.ID + "." + alloc.TaskGroup + "[0]"
		alloc.ClientStatus, alloc.DesiredStatus = structs.AllocClientStatusPending, structs.AllocDesiredStatusRun
		must.NoError(t, st.UpsertAllocs(structs.MsgTypeTestSetup, 13, []*structs.Allocation{alloc}))
	}
	got, err := st.DynamicHostVolumeClaimByID(nil, "default", "claim-a")
	must.NoError(t, err)
	return got
}

func dhvClaimsOf(t *testing.T, fsm *nomadFSM) []*structs.DynamicHostVolumeClaim {
	t.Helper()
	iter, err := fsm.State().DynamicHostVolumeClaims(nil)
	must.NoError(t, err)
	var out []*structs.DynamicHostVolumeClaim
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		out = append(out, raw.(*structs.DynamicHostVolumeClaim))
	}
	return out
}

func TestFSM_DynamicHostVolumeClaimDelete(t *testing.T) {
	ci.Parallel(t)
	delReq := func(c *structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaimDeleteRequest {
		return &structs.DynamicHostVolumeClaimDeleteRequest{Claims: []structs.DynamicHostVolumeClaimKey{
			{Namespace: c.Namespace, ID: c.ID, ExpectedModifyIndex: c.ModifyIndex}}}
	}

	t.Run("the same log entry gives identical tables on two fresh FSMs", func(t *testing.T) {
		// The entry is encoded once and the same bytes reach both FSMs, as
		// one Raft log entry reaches every server. Three claims are seeded
		// and the middle one is deleted, so the rows that stay are compared
		// and not two empty tables.
		buf, err := structs.Encode(structs.DynamicHostVolumeClaimDeleteRequestType,
			&structs.DynamicHostVolumeClaimDeleteRequest{Claims: []structs.DynamicHostVolumeClaimKey{
				{Namespace: "default", ID: "claim-b", ExpectedModifyIndex: 14}}})
		must.NoError(t, err)

		var tables [][]*structs.DynamicHostVolumeClaim
		var indexes []uint64
		for range 2 {
			fsm := testFSM(t)
			first := dhvClaimFSMSetup(t, fsm, false)
			for i, id := range []string{"claim-b", "claim-c"} {
				must.NoError(t, fsm.State().UpsertDynamicHostVolumeClaims(uint64(14+i), []*structs.DynamicHostVolumeClaim{{
					ID: id, Namespace: "default", JobID: first.JobID, TaskGroupName: first.TaskGroupName,
					VolumeName: first.VolumeName, VolumeID: first.VolumeID,
					AllocationID:    fmt.Sprintf("00000000-0000-0000-0000-0000000000c%d", i+2),
					AllocationIndex: uint64(i + 2)}}))
			}
			must.Len(t, 3, dhvClaimsOf(t, fsm))
			must.Nil(t, fsm.Apply(&raft.Log{Index: 20, Term: 1, Type: raft.LogCommand, Data: buf}))
			tables = append(tables, dhvClaimsOf(t, fsm))
			idx, err := fsm.State().Index(state.TableDynamicHostVolumeClaims)
			must.NoError(t, err)
			indexes = append(indexes, idx)
		}

		must.Eq(t, tables[0], tables[1])
		must.Eq(t, []uint64{20, 20}, indexes)
		must.Len(t, 2, tables[0])
		must.Eq(t, &structs.DynamicHostVolumeClaim{ID: "claim-c", Namespace: "default",
			JobID: "job-a", TaskGroupName: "web",
			VolumeName: "data-volume-a", VolumeID: "volume-a",
			AllocationID: "00000000-0000-0000-0000-0000000000c3", AllocationIndex: 3,
			CreateIndex: 15, ModifyIndex: 15}, tables[0][1])
		must.Eq(t, &structs.DynamicHostVolumeClaim{ID: "claim-a", Namespace: "default",
			JobID: "job-a", TaskGroupName: "web",
			VolumeName: "data-volume-a", VolumeID: "volume-a",
			AllocationID: "00000000-0000-0000-0000-0000000000c1", AllocationIndex: 1,
			CreateIndex: 12, ModifyIndex: 12}, tables[0][0])
	})

	t.Run("a held claim is refused and stays", func(t *testing.T) {
		fsm := testFSM(t)
		claim := dhvClaimFSMSetup(t, fsm, true)
		resp := applyDynamicHostVolume(t, fsm, 20, structs.DynamicHostVolumeClaimDeleteRequestType, delReq(claim))
		err, ok := resp.(error)
		must.True(t, ok)
		must.ErrorIs(t, err, state.ErrDynamicHostVolumeStateInUse)
		must.Eq(t, []*structs.DynamicHostVolumeClaim{claim}, dhvClaimsOf(t, fsm))
	})

	t.Run("a stale modify index is refused and the claim stays", func(t *testing.T) {
		fsm := testFSM(t)
		claim := dhvClaimFSMSetup(t, fsm, false)
		req := delReq(claim)
		req.Claims[0].ExpectedModifyIndex++
		resp := applyDynamicHostVolume(t, fsm, 20, structs.DynamicHostVolumeClaimDeleteRequestType, req)
		err, ok := resp.(error)
		must.True(t, ok)
		must.ErrorIs(t, err, state.ErrDynamicHostVolumeStateCAS)
		must.Len(t, 1, dhvClaimsOf(t, fsm))
	})
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/open-wander/wander/ci"
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

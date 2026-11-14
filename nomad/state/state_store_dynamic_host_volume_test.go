// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/go-memdb"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestStateStore_DynamicHostVolumeCRUD(t *testing.T) {
	store := TestStateStore(t)
	volume := &structs.DynamicHostVolume{ID: "volume-a", Name: "data-a", Namespace: "default", Type: structs.VolumeTypeHost}
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{volume}))
	stored, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	require.Equal(t, uint64(10), stored.CreateIndex)
	require.Equal(t, uint64(10), stored.ModifyIndex)
	require.Zero(t, volume.ModifyIndex)
}

func dynamicHostVolumeStateFixture(id string) *structs.DynamicHostVolume {
	return &structs.DynamicHostVolume{ID: id, Name: "data-" + id, Namespace: "default", Type: structs.VolumeTypeHost,
		NodeID: "node-a", PluginID: "plugin-a", HostPath: "/srv/wander-synthetic/data-a",
		Parameters:            map[string]string{"synthetic-key": "synthetic-value"},
		Constraints:           []*structs.Constraint{{LTarget: "synthetic-left", RTarget: "synthetic-right", Operand: "="}, nil},
		RequestedCapabilities: []*structs.DynamicHostVolumeCapability{{AttachmentMode: structs.DynamicHostVolumeAttachmentModeFilesystem}, nil},
		Attachments:           []*structs.DynamicHostVolumeAttachment{{AllocationID: "allocation-a"}, nil}}
}

func dynamicHostVolumeClaimStateFixture() *structs.DynamicHostVolumeClaim {
	return &structs.DynamicHostVolumeClaim{ID: "claim-a", Namespace: "default", JobID: "job-a", TaskGroupName: "group-a",
		VolumeName: "data-volume-a", VolumeID: "volume-a", AllocationIndex: 1}
}

func TestStateStore_DynamicHostVolumeCASAndCopies(t *testing.T) {
	store := TestStateStore(t)
	input := dynamicHostVolumeStateFixture("volume-a")
	before := input.Copy()
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{input}))
	require.Equal(t, before, input)
	input.Parameters["synthetic-key"] = "caller-change"
	input.Constraints[0].RTarget = "caller-change"
	input.RequestedCapabilities[0].AccessMode = "caller-change"
	input.Attachments[0].AllocationID = "caller-change"
	read, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	require.Equal(t, "synthetic-value", read.Parameters["synthetic-key"])
	require.Equal(t, "synthetic-right", read.Constraints[0].RTarget)
	require.Equal(t, "allocation-a", read.Attachments[0].AllocationID)
	require.Empty(t, read.RequestedCapabilities[0].AccessMode)
	read.PluginID = "plugin-b"
	read.Parameters["synthetic-key"] = "updated"
	baseline := read.Copy()
	require.NoError(t, store.UpsertDynamicHostVolumes(20, []*structs.DynamicHostVolume{read}))
	require.Equal(t, baseline, read)
	updated, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	require.Equal(t, uint64(10), updated.CreateIndex)
	require.Equal(t, uint64(20), updated.ModifyIndex)
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(30, []*structs.DynamicHostVolume{read}), ErrDynamicHostVolumeStateCAS)
	createOnly := updated.Copy()
	createOnly.ModifyIndex = 0
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(30, []*structs.DynamicHostVolume{createOnly}), ErrDynamicHostVolumeStateCAS)
	missing := dynamicHostVolumeStateFixture("missing")
	missing.ModifyIndex = 20
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(30, []*structs.DynamicHostVolume{missing}), ErrDynamicHostVolumeStateCAS)
	for _, field := range []string{"Name", "Type", "NodeID", "Origin", "CreateTime", "CreateIndex"} {
		candidate := updated.Copy()
		value := reflect.ValueOf(candidate).Elem().FieldByName(field)
		switch value.Kind() {
		case reflect.String:
			value.SetString("synthetic-other")
		case reflect.Int64:
			value.SetInt(1)
		case reflect.Uint64:
			value.SetUint(99)
		}
		require.Error(t, store.UpsertDynamicHostVolumes(30, []*structs.DynamicHostVolume{candidate}), field)
	}
	idx, err := store.Index(TableDynamicHostVolumes)
	require.NoError(t, err)
	require.Equal(t, uint64(20), idx)
}

func TestStateStore_DynamicHostVolumeIndexesAndWatches(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "synthetic-b"}}))
	first := dynamicHostVolumeStateFixture("volume-a")
	second := first.Copy()
	second.Namespace = "synthetic-b"
	optional := dynamicHostVolumeStateFixture("volume-b")
	optional.NodeID, optional.PluginID = "", ""
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{first, second, optional}))
	for _, query := range []func() (memdb.ResultIterator, error){
		func() (memdb.ResultIterator, error) { return store.DynamicHostVolumesByNamespace(nil, "default") },
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumesByIDPrefix(nil, "default", "volume-")
		},
	} {
		iter, err := query()
		require.NoError(t, err)
		require.Equal(t, "default", iter.Next().(*structs.DynamicHostVolume).Namespace)
		require.Equal(t, "volume-b", iter.Next().(*structs.DynamicHostVolume).ID)
		require.Nil(t, iter.Next())
	}
	for _, query := range []func() (memdb.ResultIterator, error){
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumesByNodeID(nil, "default", "node-a")
		},
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumesByPluginID(nil, "default", "plugin-a")
		},
	} {
		iter, err := query()
		require.NoError(t, err)
		value := iter.Next().(*structs.DynamicHostVolume)
		value.Parameters["synthetic-key"] = "iterator-change"
		value.Constraints[0].RTarget = "iterator-change"
		require.Nil(t, iter.Next())
		stored, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
		require.NoError(t, err)
		require.Equal(t, "synthetic-value", stored.Parameters["synthetic-key"])
		require.Equal(t, "synthetic-right", stored.Constraints[0].RTarget)
	}
	prefixed := dynamicHostVolumeStateFixture("volume-prefix")
	prefixed.NodeID, prefixed.PluginID = "node-ab", "plugin-ab"
	require.NoError(t, store.UpsertDynamicHostVolumes(15, []*structs.DynamicHostVolume{prefixed}))
	for _, query := range []func() (memdb.ResultIterator, error){
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumesByNodeIDPrefix(nil, "default", "node-a")
		},
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumesByPluginIDPrefix(nil, "default", "plugin-a")
		},
	} {
		iter, err := query()
		require.NoError(t, err)
		require.Equal(t, "volume-a", iter.Next().(*structs.DynamicHostVolume).ID)
		require.Equal(t, "volume-prefix", iter.Next().(*structs.DynamicHostVolume).ID)
		require.Nil(t, iter.Next())
	}
	ws := memdb.NewWatchSet()
	stored, err := store.DynamicHostVolumeByID(ws, "default", "volume-a")
	require.NoError(t, err)
	iter, err := store.DynamicHostVolumesByPluginID(ws, "default", "plugin-a")
	require.NoError(t, err)
	watch := iter.WatchCh()
	require.NotNil(t, watch)
	stored.PluginID = "plugin-b"
	require.NoError(t, store.UpsertDynamicHostVolumes(20, []*structs.DynamicHostVolume{stored}))
	select {
	case <-watch:
	default:
		t.Fatal("iterator watch was not forwarded")
	}
	old, err := store.DynamicHostVolumesByPluginID(nil, "default", "plugin-a")
	require.NoError(t, err)
	require.Nil(t, old.Next())
	newIndex, err := store.DynamicHostVolumesByPluginID(nil, "default", "plugin-b")
	require.NoError(t, err)
	require.NotNil(t, newIndex.Next())
	conflict := dynamicHostVolumeStateFixture("volume-c")
	conflict.Name = first.Name
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(30, []*structs.DynamicHostVolume{conflict}), ErrDynamicHostVolumeStateConflict)
	optional2 := optional.Copy()
	optional2.ID = "volume-c"
	require.NoError(t, store.UpsertDynamicHostVolumes(30, []*structs.DynamicHostVolume{optional2}))
}

func TestStateStore_DynamicHostVolumeAtomicityAndInvalidKeys(t *testing.T) {
	store := TestStateStore(t)
	iter, err := store.DynamicHostVolumes(nil)
	require.NoError(t, err)
	watch := iter.WatchCh()
	valid := dynamicHostVolumeStateFixture("volume-a")
	invalid := dynamicHostVolumeStateFixture("volume-b")
	invalid.Namespace = "missing-namespace"
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{valid, invalid}), ErrDynamicHostVolumeStateNamespace)
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{valid, valid}), ErrDynamicHostVolumeStateConflict)
	forged := valid.Copy()
	forged.CreateIndex = 99
	require.ErrorIs(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{forged}), ErrDynamicHostVolumeStateInvalid)
	for _, field := range []string{"ID", "Namespace", "Name", "NodeID", "PluginID"} {
		candidate := valid.Copy()
		reflect.ValueOf(candidate).Elem().FieldByName(field).SetString("synthetic-hidden\x00value")
		err := store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{valid, candidate})
		require.ErrorIs(t, err, ErrDynamicHostVolumeStateInvalid)
		require.NotContains(t, err.Error(), "synthetic-hidden")
	}
	for _, index := range []uint64{0} {
		require.ErrorIs(t, store.UpsertDynamicHostVolumes(index, []*structs.DynamicHostVolume{valid}), ErrDynamicHostVolumeStateIndex)
	}
	require.NoError(t, store.UpsertDynamicHostVolumes(0, nil))
	require.NoError(t, store.DeleteDynamicHostVolumes(0, nil))
	index, err := store.Index(TableDynamicHostVolumes)
	require.NoError(t, err)
	require.Zero(t, index)
	select {
	case <-watch:
		t.Fatal("failed batch changed watches")
	default:
	}
	missing, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	require.Nil(t, missing)
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{valid}))
	for _, index := range []uint64{0, 9, 10} {
		require.ErrorIs(t, store.DeleteDynamicHostVolumes(index, []DynamicHostVolumeDelete{{Namespace: "default", ID: valid.ID, ExpectedModifyIndex: 10}}), ErrDynamicHostVolumeStateIndex)
	}
	for _, expected := range []uint64{0, 9} {
		require.ErrorIs(t, store.DeleteDynamicHostVolumes(20, []DynamicHostVolumeDelete{{Namespace: "default", ID: valid.ID, ExpectedModifyIndex: expected}}), ErrDynamicHostVolumeStateCAS)
	}
	key := DynamicHostVolumeDelete{Namespace: "default", ID: valid.ID, ExpectedModifyIndex: 10}
	require.ErrorIs(t, store.DeleteDynamicHostVolumes(20, []DynamicHostVolumeDelete{key, key}), ErrDynamicHostVolumeStateConflict)
	require.ErrorIs(t, store.DeleteDynamicHostVolumes(20, []DynamicHostVolumeDelete{key, {Namespace: "default", ID: "missing", ExpectedModifyIndex: 10}}), ErrDynamicHostVolumeStateCAS)
	missing, err = store.DynamicHostVolumeByID(nil, "default", valid.ID)
	require.NoError(t, err)
	require.NotNil(t, missing)
	require.NoError(t, store.DeleteDynamicHostVolumes(20, []DynamicHostVolumeDelete{key}))
	missing, err = store.DynamicHostVolumeByID(nil, "default", valid.ID)
	require.NoError(t, err)
	require.Nil(t, missing)
	_, err = store.DynamicHostVolumeByID(nil, "default", "synthetic-hidden\x00value")
	require.ErrorIs(t, err, ErrDynamicHostVolumeStateInvalid)
	_, err = store.DynamicHostVolumesByIDPrefix(nil, "default\x00hidden", "")
	require.ErrorIs(t, err, ErrDynamicHostVolumeStateInvalid)
}

func TestStateStore_DynamicHostVolumeClaims(t *testing.T) {
	store := TestStateStore(t)
	volume := dynamicHostVolumeStateFixture("volume-a")
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{volume}))
	claim := dynamicHostVolumeClaimStateFixture()
	before := claim.Copy()
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{claim}))
	require.Equal(t, before, claim)
	stored, err := store.DynamicHostVolumeClaimByID(nil, "default", "claim-a")
	require.NoError(t, err)
	require.Equal(t, uint64(11), stored.CreateIndex)
	for _, query := range []func() (memdb.ResultIterator, error){
		func() (memdb.ResultIterator, error) { return store.DynamicHostVolumeClaims(nil) },
		func() (memdb.ResultIterator, error) { return store.DynamicHostVolumeClaimsByNamespace(nil, "default") },
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByIDPrefix(nil, "default", "claim-")
		},
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByVolumeID(nil, "default", "volume-a")
		},
		func() (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByJobID(nil, "default", "job-a")
		},
	} {
		iter, err := query()
		require.NoError(t, err)
		got := iter.Next().(*structs.DynamicHostVolumeClaim)
		got.JobID = "caller-change"
		require.Nil(t, iter.Next())
		read, err := store.DynamicHostVolumeClaimByID(nil, "default", "claim-a")
		require.NoError(t, err)
		require.Equal(t, "job-a", read.JobID)
	}
	for _, field := range []string{"JobID", "TaskGroupName", "VolumeName", "VolumeID", "AllocationIndex", "CreateIndex"} {
		candidate := stored.Copy()
		value := reflect.ValueOf(candidate).Elem().FieldByName(field)
		if value.Kind() == reflect.String {
			value.SetString("synthetic-other")
		} else {
			value.SetUint(99)
		}
		require.ErrorIs(t, store.UpsertDynamicHostVolumeClaims(20, []*structs.DynamicHostVolumeClaim{candidate}), ErrDynamicHostVolumeStateConflict)
	}
	stored.AllocationID = "allocation-replacement"
	ws := memdb.NewWatchSet()
	iter, err := store.DynamicHostVolumeClaimsByVolumeID(ws, "default", volume.ID)
	require.NoError(t, err)
	watch := iter.WatchCh()
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(20, []*structs.DynamicHostVolumeClaim{stored}))
	select {
	case <-watch:
	default:
		t.Fatal("claim watch not forwarded")
	}
	require.ErrorIs(t, store.UpsertDynamicHostVolumeClaims(21, []*structs.DynamicHostVolumeClaim{stored}), ErrDynamicHostVolumeStateCAS)
	key := DynamicHostVolumeDelete{Namespace: "default", ID: volume.ID, ExpectedModifyIndex: 10}
	require.ErrorIs(t, store.DeleteDynamicHostVolumes(30, []DynamicHostVolumeDelete{key}), ErrDynamicHostVolumeStateInUse)
	require.ErrorIs(t, store.DeleteDynamicHostVolumeClaims(21, []DynamicHostVolumeClaimDelete{{Namespace: "default", ID: "claim-a", ExpectedModifyIndex: 11}}), ErrDynamicHostVolumeStateCAS)
	require.NoError(t, store.DeleteDynamicHostVolumeClaims(21, []DynamicHostVolumeClaimDelete{{Namespace: "default", ID: "claim-a", ExpectedModifyIndex: 20}}))
	require.NoError(t, store.DeleteDynamicHostVolumes(30, []DynamicHostVolumeDelete{key}))
}

func TestStateStore_DynamicHostVolumeClaimsAtomicity(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "synthetic-b"}}))
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{dynamicHostVolumeStateFixture("volume-a")}))
	claim := dynamicHostVolumeClaimStateFixture()
	for _, field := range []string{"ID", "Namespace", "JobID", "TaskGroupName", "VolumeName", "VolumeID", "AllocationID"} {
		candidate := claim.Copy()
		reflect.ValueOf(candidate).Elem().FieldByName(field).SetString("synthetic-hidden\x00value")
		err := store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{claim, candidate})
		require.Error(t, err)
		require.NotContains(t, err.Error(), "synthetic-hidden")
	}
	for _, field := range []string{"Namespace", "VolumeName", "VolumeID"} {
		candidate := claim.Copy()
		candidate.ID = "claim-b"
		reflect.ValueOf(candidate).Elem().FieldByName(field).SetString("synthetic-b")
		require.Error(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{claim, candidate}))
	}
	require.ErrorIs(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{claim, claim}), ErrDynamicHostVolumeStateConflict)
	forged := claim.Copy()
	forged.CreateIndex = 99
	require.ErrorIs(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{forged}), ErrDynamicHostVolumeStateInvalid)
	require.ErrorIs(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{claim, nil}), ErrDynamicHostVolumeStateInvalid)
	index, err := store.Index(TableDynamicHostVolumeClaims)
	require.NoError(t, err)
	require.Zero(t, index)
	missing, err := store.DynamicHostVolumeClaimByID(nil, "default", claim.ID)
	require.NoError(t, err)
	require.Nil(t, missing)
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(0, nil))
	require.NoError(t, store.DeleteDynamicHostVolumeClaims(0, nil))
}

func TestStateStore_DynamicHostVolumeNamespaceIntegrityAndConcurrentCAS(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "synthetic-b"}, {Name: "synthetic-c"}}))
	volume := dynamicHostVolumeStateFixture("volume-a")
	volume.Namespace = "synthetic-b"
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{volume}))
	require.ErrorIs(t, store.DeleteNamespaces(20, []string{"synthetic-c", "synthetic-b"}), ErrDynamicHostVolumeStateInUse)
	ns, err := store.NamespaceByName(nil, "synthetic-c")
	require.NoError(t, err)
	require.NotNil(t, ns, "namespace delete batch must roll back")
	current, err := store.DynamicHostVolumeByID(nil, "synthetic-b", "volume-a")
	require.NoError(t, err)
	var workers sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, index := range []uint64{20, 21} {
		candidate := current.Copy()
		workers.Add(1)
		go func(index uint64) {
			defer workers.Done()
			<-start
			results <- store.UpsertDynamicHostVolumes(index, []*structs.DynamicHostVolume{candidate})
		}(index)
	}
	close(start)
	workers.Wait()
	close(results)
	var successes int
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.True(t, errors.Is(err, ErrDynamicHostVolumeStateCAS) || errors.Is(err, ErrDynamicHostVolumeStateIndex))
		}
	}
	require.Equal(t, 1, successes)
	updated, err := store.DynamicHostVolumeByID(nil, "synthetic-b", "volume-a")
	require.NoError(t, err)
	require.NoError(t, store.DeleteDynamicHostVolumes(30, []DynamicHostVolumeDelete{{Namespace: updated.Namespace, ID: updated.ID, ExpectedModifyIndex: updated.ModifyIndex}}))
	require.NoError(t, store.DeleteNamespaces(31, []string{"synthetic-b", "synthetic-c"}))
	require.False(t, strings.Contains(ErrDynamicHostVolumeStateInUse.Error(), volume.HostPath))
}

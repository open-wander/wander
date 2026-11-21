// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/open-wander/wander/nomad/mock"
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

func TestStateStore_DynamicHostVolumeBlockingQueries(t *testing.T) {
	vol := func() *structs.DynamicHostVolume { return dynamicHostVolumeStateFixture("volume-a") }
	type watchCase struct {
		name  string
		table string
		query func(s *StateStore, ws memdb.WatchSet) error
	}
	iterQuery := func(f func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error)) func(*StateStore, memdb.WatchSet) error {
		return func(s *StateStore, ws memdb.WatchSet) error {
			iter, err := f(s, ws)
			if err != nil {
				return err
			}
			for iter.Next() != nil {
			}
			return nil
		}
	}
	cases := []watchCase{
		{"volume by id", TableDynamicHostVolumes, func(s *StateStore, ws memdb.WatchSet) error {
			_, err := s.DynamicHostVolumeByID(ws, "default", "volume-a")
			return err
		}},
		{"volumes", TableDynamicHostVolumes, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumes(ws)
		})},
		{"volumes by namespace", TableDynamicHostVolumes, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumesByNamespace(ws, "default")
		})},
		{"volumes by id prefix", TableDynamicHostVolumes, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumesByIDPrefix(ws, "default", "volume-")
		})},
		{"volumes by node id prefix", TableDynamicHostVolumes, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumesByNodeIDPrefix(ws, "default", "node-")
		})},
		{"volumes by plugin id prefix", TableDynamicHostVolumes, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumesByPluginIDPrefix(ws, "default", "plugin-")
		})},
		{"claim by id", TableDynamicHostVolumeClaims, func(s *StateStore, ws memdb.WatchSet) error {
			_, err := s.DynamicHostVolumeClaimByID(ws, "default", "claim-a")
			return err
		}},
		{"claims", TableDynamicHostVolumeClaims, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumeClaims(ws)
		})},
		{"claims by volume id", TableDynamicHostVolumeClaims, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumeClaimsByVolumeID(ws, "default", "volume-a")
		})},
		{"claims by job id", TableDynamicHostVolumeClaims, iterQuery(func(s *StateStore, ws memdb.WatchSet) (memdb.ResultIterator, error) {
			return s.DynamicHostVolumeClaimsByJobID(ws, "default", "job-a")
		})},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := TestStateStore(t)
			claims := tc.table == TableDynamicHostVolumeClaims
			require.NoError(t, store.UpsertDynamicHostVolumes(5, []*structs.DynamicHostVolume{vol()}))
			if !claims {
				// Start from an empty volume table for the create step.
				require.NoError(t, store.DeleteDynamicHostVolumes(6, []DynamicHostVolumeDelete{{Namespace: "default", ID: "volume-a", ExpectedModifyIndex: 5}}))
			}

			fired := func(ws memdb.WatchSet) bool { return !ws.Watch(time.After(2 * time.Second)) }
			prev := func() uint64 {
				idx, err := store.Index(tc.table)
				require.NoError(t, err)
				return idx
			}
			write := func(index uint64, step string) {
				switch {
				case claims && step == "delete":
					require.NoError(t, store.DeleteDynamicHostVolumeClaims(index, []DynamicHostVolumeClaimDelete{{Namespace: "default", ID: "claim-a", ExpectedModifyIndex: prev()}}))
				case claims:
					c := dynamicHostVolumeClaimStateFixture()
					c.VolumeName = "data-volume-a"
					if step == "update" {
						existing, err := store.DynamicHostVolumeClaimByID(nil, "default", "claim-a")
						require.NoError(t, err)
						c = existing
						c.AllocationID = "allocation-updated"
					}
					require.NoError(t, store.UpsertDynamicHostVolumeClaims(index, []*structs.DynamicHostVolumeClaim{c}))
				case step == "delete":
					require.NoError(t, store.DeleteDynamicHostVolumes(index, []DynamicHostVolumeDelete{{Namespace: "default", ID: "volume-a", ExpectedModifyIndex: prev()}}))
				case step == "update":
					existing, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
					require.NoError(t, err)
					existing.State = structs.DynamicHostVolumeStateReady
					require.NoError(t, store.UpsertDynamicHostVolumes(index, []*structs.DynamicHostVolume{existing}))
				default:
					require.NoError(t, store.UpsertDynamicHostVolumes(index, []*structs.DynamicHostVolume{vol()}))
				}
			}
			if claims {
				// Claim references the stored volume name.
				stored, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
				require.NoError(t, err)
				require.Equal(t, "data-volume-a", stored.Name)
			}

			before := prev()
			for _, step := range []struct {
				name  string
				index uint64
			}{{"create", 10}, {"update", 20}, {"delete", 30}} {
				ws := memdb.NewWatchSet()
				require.NoError(t, tc.query(store, ws))
				write(step.index, step.name)
				require.Truef(t, fired(ws), "watch did not fire on %s", step.name)
				now := prev()
				require.Greaterf(t, now, before, "table index must increase on %s", step.name)
				require.Equal(t, step.index, now)
				before = now
			}
		})
	}
}

func TestStateStore_DynamicHostVolumeBlockingQueriesUnrelatedWrites(t *testing.T) {
	store := TestStateStore(t)
	require.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "synthetic-b"}}))
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{dynamicHostVolumeStateFixture("volume-a")}))
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{dynamicHostVolumeClaimStateFixture()}))
	volumeBefore, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	claimBefore, err := store.DynamicHostVolumeClaimByID(nil, "default", "claim-a")
	require.NoError(t, err)

	// A write in another namespace must not change what readers of the first
	// namespace see; the watch itself may or may not fire.
	other := dynamicHostVolumeStateFixture("volume-a")
	other.Namespace = "synthetic-b"
	require.NoError(t, store.UpsertDynamicHostVolumes(20, []*structs.DynamicHostVolume{other}))
	volumeAfter, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
	require.NoError(t, err)
	require.True(t, volumeBefore.Equal(volumeAfter))
	iter, err := store.DynamicHostVolumesByNamespace(nil, "default")
	require.NoError(t, err)
	require.Equal(t, "default", iter.Next().(*structs.DynamicHostVolume).Namespace)
	require.Nil(t, iter.Next())
	claimIndex, err := store.Index(TableDynamicHostVolumeClaims)
	require.NoError(t, err)
	require.Equal(t, uint64(11), claimIndex, "volume write must not move the claim table index")

	// A claim write leaves the volume table index and volume record alone.
	second := dynamicHostVolumeClaimStateFixture()
	second.ID = "claim-b"
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(30, []*structs.DynamicHostVolumeClaim{second}))
	volumeIndex, err := store.Index(TableDynamicHostVolumes)
	require.NoError(t, err)
	require.Equal(t, uint64(20), volumeIndex, "claim write must not move the volume table index")
	claimAfter, err := store.DynamicHostVolumeClaimByID(nil, "default", "claim-a")
	require.NoError(t, err)
	require.Equal(t, claimBefore, claimAfter)

	// An unrelated table write changes neither dynamic table index.
	require.NoError(t, store.UpsertNamespaces(40, []*structs.Namespace{{Name: "synthetic-c"}}))
	for table, want := range map[string]uint64{TableDynamicHostVolumes: 20, TableDynamicHostVolumeClaims: 30} {
		got, err := store.Index(table)
		require.NoError(t, err)
		require.Equal(t, want, got, table)
	}
}

func TestStateStore_DynamicHostVolumeRestoreRejectsInvalid(t *testing.T) {
	validVolume := func() *structs.DynamicHostVolume {
		v := dynamicHostVolumeStateFixture("volume-a")
		v.CreateIndex, v.ModifyIndex = 5, 6
		return v
	}
	validClaim := func() *structs.DynamicHostVolumeClaim {
		c := dynamicHostVolumeClaimStateFixture()
		c.CreateIndex, c.ModifyIndex = 5, 6
		return c
	}
	volumeCases := []struct {
		name    string
		mutate  func(v *structs.DynamicHostVolume) *structs.DynamicHostVolume
		wantErr error
	}{
		{"nil", func(*structs.DynamicHostVolume) *structs.DynamicHostVolume { return nil }, ErrDynamicHostVolumeStateInvalid},
		{"wrong type", func(v *structs.DynamicHostVolume) *structs.DynamicHostVolume { v.Type = "csi"; return v }, ErrDynamicHostVolumeStateInvalid},
		{"empty id", func(v *structs.DynamicHostVolume) *structs.DynamicHostVolume { v.ID = ""; return v }, ErrDynamicHostVolumeStateInvalid},
		{"nul in namespace", func(v *structs.DynamicHostVolume) *structs.DynamicHostVolume { v.Namespace = "a\x00b"; return v }, ErrDynamicHostVolumeStateInvalid},
		{"zero create index", func(v *structs.DynamicHostVolume) *structs.DynamicHostVolume { v.CreateIndex = 0; return v }, ErrDynamicHostVolumeStateInvalid},
		{"create after modify", func(v *structs.DynamicHostVolume) *structs.DynamicHostVolume { v.CreateIndex = 9; return v }, ErrDynamicHostVolumeStateInvalid},
	}
	for _, tc := range volumeCases {
		t.Run("volume "+tc.name, func(t *testing.T) {
			restore, err := TestStateStore(t).Restore()
			require.NoError(t, err)
			defer restore.Abort()
			require.ErrorIs(t, restore.DynamicHostVolumeRestore(tc.mutate(validVolume())), tc.wantErr)
		})
	}
	claimCases := []struct {
		name   string
		mutate func(c *structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaim
	}{
		{"nil", func(*structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaim { return nil }},
		{"fails validate", func(c *structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaim { c.JobID = ""; return c }},
		{"nul in volume id", func(c *structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaim {
			c.VolumeID = "a\x00"
			return c
		}},
		{"zero create index", func(c *structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaim { c.CreateIndex = 0; return c }},
		{"create after modify", func(c *structs.DynamicHostVolumeClaim) *structs.DynamicHostVolumeClaim { c.CreateIndex = 9; return c }},
	}
	for _, tc := range claimCases {
		t.Run("claim "+tc.name, func(t *testing.T) {
			restore, err := TestStateStore(t).Restore()
			require.NoError(t, err)
			defer restore.Abort()
			require.ErrorIs(t, restore.DynamicHostVolumeClaimRestore(tc.mutate(validClaim())), ErrDynamicHostVolumeStateInvalid)
		})
	}

	t.Run("duplicates and node name conflict", func(t *testing.T) {
		restore, err := TestStateStore(t).Restore()
		require.NoError(t, err)
		defer restore.Abort()
		require.NoError(t, restore.DynamicHostVolumeRestore(validVolume()))
		require.ErrorIs(t, restore.DynamicHostVolumeRestore(validVolume()), ErrDynamicHostVolumeStateConflict)
		clash := validVolume()
		clash.ID = "volume-b"
		clash.Name = validVolume().Name
		require.ErrorIs(t, restore.DynamicHostVolumeRestore(clash), ErrDynamicHostVolumeStateConflict)

		require.NoError(t, restore.DynamicHostVolumeClaimRestore(validClaim()))
		require.ErrorIs(t, restore.DynamicHostVolumeClaimRestore(validClaim()), ErrDynamicHostVolumeStateConflict)
	})

	t.Run("no referential checks and copies input", func(t *testing.T) {
		store := TestStateStore(t)
		restore, err := store.Restore()
		require.NoError(t, err)
		orphan := validClaim()
		orphan.Namespace = "missing-namespace"
		require.NoError(t, restore.DynamicHostVolumeClaimRestore(orphan))
		volume := validVolume()
		require.NoError(t, restore.DynamicHostVolumeRestore(volume))
		volume.Parameters["synthetic-key"] = "mutated-after-restore"
		require.NoError(t, restore.Commit())
		stored, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
		require.NoError(t, err)
		require.Equal(t, "synthetic-value", stored.Parameters["synthetic-key"])
		require.Equal(t, uint64(5), stored.CreateIndex)
		require.Equal(t, uint64(6), stored.ModifyIndex)
		idx, err := store.Index(TableDynamicHostVolumes)
		require.NoError(t, err)
		require.Zero(t, idx, "restore methods must not write table indexes")
	})
}

func dynamicHostVolumeClaimKeys(iter memdb.ResultIterator) []string {
	var keys []string
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claim := raw.(*structs.DynamicHostVolumeClaim)
		keys = append(keys, claim.Namespace+"/"+claim.ID)
	}
	return keys
}

// Both namespaces hold the same volume ID, job ID and claim ID prefix. Claims
// in a single namespace cannot tell a scoped lister from an unscoped one.
func TestStateStore_DynamicHostVolumeClaimsNamespaceScoped(t *testing.T) {
	store := TestStateStore(t)
	// "defaul" is a string prefix of "default" and must not match it.
	require.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "synthetic-b"}, {Name: "defaul"}}))
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{
		{ID: "volume-a", Name: "data-a", Namespace: "default", Type: structs.VolumeTypeHost},
		{ID: "volume-a", Name: "data-a", Namespace: "synthetic-b", Type: structs.VolumeTypeHost},
	}))
	require.NoError(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{
		{ID: "claim-a", Namespace: "default", JobID: "job-a", TaskGroupName: "group-a", VolumeName: "data-a", VolumeID: "volume-a"},
		{ID: "claim-b", Namespace: "synthetic-b", JobID: "job-a", TaskGroupName: "group-a", VolumeName: "data-a", VolumeID: "volume-a"},
	}))
	listers := map[string]func(string) (memdb.ResultIterator, error){
		"by namespace": func(namespace string) (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByNamespace(nil, namespace)
		},
		"by id prefix": func(namespace string) (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByIDPrefix(nil, namespace, "claim-")
		},
		"by volume id": func(namespace string) (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByVolumeID(nil, namespace, "volume-a")
		},
		"by job id": func(namespace string) (memdb.ResultIterator, error) {
			return store.DynamicHostVolumeClaimsByJobID(nil, namespace, "job-a")
		},
	}
	expected := map[string][]string{
		"default":     {"default/claim-a"},
		"synthetic-b": {"synthetic-b/claim-b"},
		"defaul":      nil,
		"absent":      nil,
	}
	for name, list := range listers {
		for namespace, want := range expected {
			iter, err := list(namespace)
			require.NoError(t, err, "%s in namespace %q", name, namespace)
			require.Equal(t, want, dynamicHostVolumeClaimKeys(iter), "%s in namespace %q", name, namespace)
		}
		for _, namespace := range []string{"", "default\x00hidden"} {
			_, err := list(namespace)
			require.ErrorIs(t, err, ErrDynamicHostVolumeStateInvalid, "%s with namespace %q", name, namespace)
		}
	}
}

// Restore rejects a volume of any other type, so one stored record of another
// type would make every later snapshot fail to restore.
func TestStateStore_DynamicHostVolumeRejectsNonHostType(t *testing.T) {
	for _, volumeType := range []string{"csi", "", "Host", "synthetic-other"} {
		t.Run("type "+volumeType, func(t *testing.T) {
			store := TestStateStore(t)
			candidate := &structs.DynamicHostVolume{ID: "volume-a", Name: "data-a", Namespace: "default", Type: volumeType}
			require.ErrorIs(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{candidate}),
				ErrDynamicHostVolumeStateInvalid)
			stored, err := store.DynamicHostVolumeByID(nil, "default", "volume-a")
			require.NoError(t, err)
			require.Nil(t, stored)
		})
	}
	store := TestStateStore(t)
	control := &structs.DynamicHostVolume{ID: "volume-a", Name: "data-a", Namespace: "default", Type: structs.VolumeTypeHost}
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{control}))
}

// Each field that DynamicHostVolumeClaim.Validate requires must be rejected as
// an invalid record. A later lookup or index error would also stop most of
// them, but with a different error, and would let an empty task group through.
func TestStateStore_DynamicHostVolumeClaimRequiredFields(t *testing.T) {
	seeded := func(t *testing.T) *StateStore {
		store := TestStateStore(t)
		require.NoError(t, store.UpsertDynamicHostVolumes(10,
			[]*structs.DynamicHostVolume{dynamicHostVolumeStateFixture("volume-a")}))
		return store
	}
	for _, field := range []string{"ID", "Namespace", "JobID", "TaskGroupName", "VolumeName", "VolumeID"} {
		t.Run(field, func(t *testing.T) {
			store := seeded(t)
			candidate := dynamicHostVolumeClaimStateFixture()
			reflect.ValueOf(candidate).Elem().FieldByName(field).SetString("")
			require.ErrorIs(t, store.UpsertDynamicHostVolumeClaims(11, []*structs.DynamicHostVolumeClaim{candidate}),
				ErrDynamicHostVolumeStateInvalid)
			iter, err := store.DynamicHostVolumeClaims(nil)
			require.NoError(t, err)
			require.Nil(t, iter.Next())
		})
	}
	require.NoError(t, seeded(t).UpsertDynamicHostVolumeClaims(11,
		[]*structs.DynamicHostVolumeClaim{dynamicHostVolumeClaimStateFixture()}))
}

// planResultsClaimFixture is a store with one volume record and a plan result
// that places an allocation of job-a, group-a on it with a claim.
func planResultsClaimFixture(t *testing.T) (*StateStore, *structs.ApplyPlanResultsRequest, *structs.Allocation) {
	t.Helper()
	store := TestStateStore(t)
	require.NoError(t, store.UpsertDynamicHostVolumes(10, []*structs.DynamicHostVolume{dynamicHostVolumeStateFixture("volume-a")}))

	alloc := mock.Alloc()
	alloc.Resources = &structs.Resources{}
	job := alloc.Job
	alloc.Job = nil
	claim := dynamicHostVolumeClaimStateFixture()
	claim.AllocationID = alloc.ID
	req := &structs.ApplyPlanResultsRequest{
		AllocUpdateRequest:      structs.AllocUpdateRequest{Job: job, AllocsUpdated: []*structs.Allocation{alloc}},
		DynamicHostVolumeClaims: []*structs.DynamicHostVolumeClaim{claim},
	}
	require.NoError(t, store.UpsertJobSummary(11, mock.JobSummary(alloc.JobID)))
	return store, req, alloc
}

func TestStateStore_UpsertPlanResults_DynamicHostVolumeClaims(t *testing.T) {
	store, req, alloc := planResultsClaimFixture(t)
	claim := req.DynamicHostVolumeClaims[0]
	before := claim.Copy()

	require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req))
	require.Equal(t, before, claim, "the request is not modified")

	// The claim and the allocation are written by the same apply.
	storedAlloc, err := store.AllocByID(nil, alloc.ID)
	require.NoError(t, err)
	require.NotNil(t, storedAlloc)
	require.Equal(t, uint64(20), storedAlloc.CreateIndex)
	stored, err := store.DynamicHostVolumeClaimByID(nil, "default", claim.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, alloc.ID, stored.AllocationID)
	require.Equal(t, uint64(20), stored.CreateIndex)
	require.Equal(t, uint64(20), stored.ModifyIndex)
	index, err := store.Index(TableDynamicHostVolumeClaims)
	require.NoError(t, err)
	require.Equal(t, uint64(20), index)

	// The claim survives a snapshot and restore.
	snap, err := store.Snapshot()
	require.NoError(t, err)
	restored := TestStateStore(t)
	restore, err := restored.Restore()
	require.NoError(t, err)
	iter, err := snap.DynamicHostVolumeClaims(nil)
	require.NoError(t, err)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		require.NoError(t, restore.DynamicHostVolumeClaimRestore(raw.(*structs.DynamicHostVolumeClaim)))
	}
	require.NoError(t, restore.Commit())
	got, err := restored.DynamicHostVolumeClaimByID(nil, "default", claim.ID)
	require.NoError(t, err)
	require.Equal(t, stored, got)
}

func TestStateStore_UpsertPlanResults_DynamicHostVolumeClaimsWatch(t *testing.T) {
	store, req, _ := planResultsClaimFixture(t)
	ws := memdb.NewWatchSet()
	iter, err := store.DynamicHostVolumeClaimsByJobID(ws, "default", "job-a")
	require.NoError(t, err)
	require.Nil(t, iter.Next())
	require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req))
	require.True(t, ws.Watch(time.After(time.Second)) == false, "the claims watch fires")
}

func TestStateStore_UpsertPlanResults_DynamicHostVolumeClaimsSkipped(t *testing.T) {
	t.Run("a claim already held for the volume is kept", func(t *testing.T) {
		store, req, alloc := planResultsClaimFixture(t)
		require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req))

		// A later placement of the group on the same volume carries a new
		// claim ID, as a racing plan would.
		second := mock.Alloc()
		second.Resources = &structs.Resources{}
		secondReq := &structs.ApplyPlanResultsRequest{
			AllocUpdateRequest: structs.AllocUpdateRequest{Job: req.Job, AllocsUpdated: []*structs.Allocation{second}},
			DynamicHostVolumeClaims: []*structs.DynamicHostVolumeClaim{func() *structs.DynamicHostVolumeClaim {
				c := dynamicHostVolumeClaimStateFixture()
				c.ID, c.AllocationID = "claim-racing", second.ID
				return c
			}()},
		}
		second.Job = nil
		require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 21, secondReq))

		iter, err := store.DynamicHostVolumeClaimsByJobID(nil, "default", "job-a")
		require.NoError(t, err)
		first := iter.Next().(*structs.DynamicHostVolumeClaim)
		require.Nil(t, iter.Next(), "one claim per volume")
		require.Equal(t, "claim-a", first.ID)
		require.Equal(t, alloc.ID, first.AllocationID, "the claim is left as it is")
		placed, err := store.AllocByID(nil, second.ID)
		require.NoError(t, err)
		require.NotNil(t, placed, "the allocation is still committed")
		index, err := store.Index(TableDynamicHostVolumeClaims)
		require.NoError(t, err)
		require.Equal(t, uint64(20), index, "a skipped claim writes nothing")
	})

	t.Run("a vanished volume skips the claim and keeps the allocation", func(t *testing.T) {
		store, req, alloc := planResultsClaimFixture(t)
		require.NoError(t, store.DeleteDynamicHostVolumes(15, []DynamicHostVolumeDelete{{Namespace: "default", ID: "volume-a", ExpectedModifyIndex: 10}}))
		require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req))
		placed, err := store.AllocByID(nil, alloc.ID)
		require.NoError(t, err)
		require.NotNil(t, placed)
		claims, err := store.DynamicHostVolumeClaimsByJobID(nil, "default", "job-a")
		require.NoError(t, err)
		require.Nil(t, claims.Next())
	})
}

func TestStateStore_UpsertPlanResults_DynamicHostVolumeClaimsAtomic(t *testing.T) {
	t.Run("an invalid claim aborts the allocations too", func(t *testing.T) {
		store, req, alloc := planResultsClaimFixture(t)
		req.DynamicHostVolumeClaims[0].JobID = ""
		require.ErrorIs(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req), ErrDynamicHostVolumeStateInvalid)
		placed, err := store.AllocByID(nil, alloc.ID)
		require.NoError(t, err)
		require.Nil(t, placed, "no partial write")
		index, err := store.Index(TableDynamicHostVolumeClaims)
		require.NoError(t, err)
		require.Zero(t, index)
	})

	t.Run("an index not above the claims index is refused without a partial write", func(t *testing.T) {
		store, req, _ := planResultsClaimFixture(t)
		require.NoError(t, store.UpsertDynamicHostVolumeClaims(30, []*structs.DynamicHostVolumeClaim{{
			ID: "claim-other", Namespace: "default", JobID: "job-b", TaskGroupName: "group-a",
			VolumeName: "data-volume-a", VolumeID: "volume-a",
		}}))
		require.ErrorIs(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 30, req), ErrDynamicHostVolumeStateIndex)
		placed, err := store.AllocByID(nil, req.AllocsUpdated[0].ID)
		require.NoError(t, err)
		require.Nil(t, placed)
	})

	t.Run("applying the same request twice is idempotent", func(t *testing.T) {
		store, req, alloc := planResultsClaimFixture(t)
		require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req))
		require.NoError(t, store.UpsertPlanResults(structs.MsgTypeTestSetup, 20, req))
		iter, err := store.DynamicHostVolumeClaimsByJobID(nil, "default", "job-a")
		require.NoError(t, err)
		require.NotNil(t, iter.Next())
		require.Nil(t, iter.Next())
		placed, err := store.AllocByID(nil, alloc.ID)
		require.NoError(t, err)
		require.NotNil(t, placed)
	})
}

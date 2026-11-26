// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/go-memdb"
	"github.com/open-wander/wander/nomad/structs"
)

// State errors contain no record values or underlying memdb diagnostics.
var (
	ErrDynamicHostVolumeStateInvalid   = errors.New("dynamic host volume state: invalid record or key")
	ErrDynamicHostVolumeStateCAS       = errors.New("dynamic host volume state: compare-and-set conflict")
	ErrDynamicHostVolumeStateIndex     = errors.New("dynamic host volume state: invalid table index")
	ErrDynamicHostVolumeStateConflict  = errors.New("dynamic host volume state: identity conflict")
	ErrDynamicHostVolumeStateNamespace = errors.New("dynamic host volume state: namespace missing")
	ErrDynamicHostVolumeStateInUse     = errors.New("dynamic host volume state: referenced record")
	ErrDynamicHostVolumeStateStorage   = errors.New("dynamic host volume state: storage failure")
)

// DynamicHostVolumeDelete is a state-only exact-key CAS descriptor.
type DynamicHostVolumeDelete struct {
	Namespace           string
	ID                  string
	ExpectedModifyIndex uint64
}

type DynamicHostVolumeClaimDelete = DynamicHostVolumeDelete

func dynamicHostVolumeKeys(required int, values ...string) error {
	for index, value := range values {
		if strings.ContainsRune(value, 0) || (index < required && value == "") {
			return ErrDynamicHostVolumeStateInvalid
		}
	}
	return nil
}

func dynamicHostVolumeBatchKey(seen map[string]bool, namespace, id string) error {
	if err := dynamicHostVolumeKeys(2, namespace, id); err != nil {
		return err
	}
	key := namespace + "\x00" + id
	if seen[key] {
		return ErrDynamicHostVolumeStateConflict
	}
	seen[key] = true
	return nil
}

func dynamicHostVolumeWriteKey(tx *txn, seen map[string]bool, namespace, id string) error {
	if err := dynamicHostVolumeBatchKey(seen, namespace, id); err != nil {
		return err
	}
	return dynamicHostVolumeNamespace(tx, namespace)
}

func dynamicHostVolumeCreateIndex(raw interface{}, supplied, transaction uint64) (uint64, error) {
	if raw != nil {
		return supplied, nil
	}
	if supplied != 0 {
		return 0, ErrDynamicHostVolumeStateInvalid
	}
	return transaction, nil
}

func (s *StateStore) writeDynamicHostVolumeBatch(index uint64, table string, count int, apply func(*txn, int, map[string]bool) error) error {
	if count == 0 {
		return nil
	}
	tx := s.db.WriteTxn(index)
	defer tx.Abort()
	raw, err := tx.First(tableIndex, indexID, table)
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	if index == 0 || (raw != nil && index <= raw.(*IndexEntry).Value) {
		return ErrDynamicHostVolumeStateIndex
	}
	seen := make(map[string]bool, count)
	for position := 0; position < count; position++ {
		if err := apply(tx, position, seen); err != nil {
			return err
		}
	}
	if err := tx.Insert(tableIndex, indexEntry(table, index)); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	if err := tx.Commit(); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

func dynamicHostVolumeExisting(tx *txn, table, namespace, id string, expected uint64) (interface{}, error) {
	raw, err := tx.First(table, indexID, namespace, id)
	if err != nil {
		return nil, ErrDynamicHostVolumeStateStorage
	}
	if raw == nil {
		if expected != 0 {
			return nil, ErrDynamicHostVolumeStateCAS
		}
		return nil, nil
	}
	var modify uint64
	switch record := raw.(type) {
	case *structs.DynamicHostVolume:
		modify = record.ModifyIndex
	case *structs.DynamicHostVolumeClaim:
		modify = record.ModifyIndex
	default:
		return nil, ErrDynamicHostVolumeStateStorage
	}
	if expected == 0 || expected != modify {
		return nil, ErrDynamicHostVolumeStateCAS
	}
	return raw, nil
}

func dynamicHostVolumeNamespace(tx *txn, namespace string) error {
	raw, err := tx.First(TableNamespaces, indexID, namespace)
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	if raw == nil {
		return ErrDynamicHostVolumeStateNamespace
	}
	return nil
}

// UpsertDynamicHostVolumes stores metadata only. Operation-specific validation
// remains the responsibility of callers; this layer enforces storage identity.
func (s *StateStore) UpsertDynamicHostVolumes(index uint64, volumes []*structs.DynamicHostVolume) error {
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumes, len(volumes), func(tx *txn, position int, seen map[string]bool) error {
		candidate := volumes[position]
		if candidate == nil || candidate.Type != structs.VolumeTypeHost {
			return ErrDynamicHostVolumeStateInvalid
		}
		if err := dynamicHostVolumeKeys(3, candidate.Namespace, candidate.ID, candidate.Name, candidate.NodeID, candidate.PluginID); err != nil {
			return err
		}
		if err := dynamicHostVolumeWriteKey(tx, seen, candidate.Namespace, candidate.ID); err != nil {
			return err
		}
		raw, err := dynamicHostVolumeExisting(tx, TableDynamicHostVolumes, candidate.Namespace, candidate.ID, candidate.ModifyIndex)
		if err != nil {
			return err
		}
		volume := candidate.Copy()
		if raw != nil {
			existing := raw.(*structs.DynamicHostVolume)
			if err := existing.ValidateImmutableFields(volume); err != nil {
				return fmt.Errorf("volume identity: %w", err)
			}
		}
		volume.CreateIndex, err = dynamicHostVolumeCreateIndex(raw, volume.CreateIndex, index)
		if err != nil {
			return err
		}
		return insertDynamicHostVolumeTxn(tx, index, volume)
	})
}

// insertDynamicHostVolumeTxn stamps the modify index and inserts the volume. A
// node holds one volume per name and namespace, which memdb's unique indexes do
// not enforce against another object's key, so it is checked here.
func insertDynamicHostVolumeTxn(tx *txn, index uint64, volume *structs.DynamicHostVolume) error {
	if volume.NodeID != "" {
		conflict, err := tx.First(TableDynamicHostVolumes, dynamicHostVolumeNodeNameIndex, volume.Namespace, volume.NodeID, volume.Name)
		if err != nil {
			return ErrDynamicHostVolumeStateStorage
		}
		if conflict != nil && conflict.(*structs.DynamicHostVolume).ID != volume.ID {
			return ErrDynamicHostVolumeStateConflict
		}
	}
	volume.ModifyIndex = index
	if err := tx.Insert(TableDynamicHostVolumes, volume); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

func (s *StateStore) UpsertDynamicHostVolumeClaims(index uint64, claims []*structs.DynamicHostVolumeClaim) error {
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumeClaims, len(claims), func(tx *txn, position int, seen map[string]bool) error {
		return upsertDynamicHostVolumeClaimTxn(tx, index, claims[position], seen)
	})
}

// upsertDynamicHostVolumeClaimTxn validates and inserts one claim inside the
// caller's transaction. It is the only claim insert path: the batch method and
// the plan results both use it. The caller checks and bumps the table index.
func upsertDynamicHostVolumeClaimTxn(tx *txn, index uint64, candidate *structs.DynamicHostVolumeClaim, seen map[string]bool) error {
	if candidate == nil || candidate.Validate() != nil {
		return ErrDynamicHostVolumeStateInvalid
	}
	if err := dynamicHostVolumeKeys(2, candidate.Namespace, candidate.ID, candidate.JobID, candidate.VolumeID,
		candidate.TaskGroupName, candidate.VolumeName, candidate.AllocationID); err != nil {
		return err
	}
	if err := dynamicHostVolumeWriteKey(tx, seen, candidate.Namespace, candidate.ID); err != nil {
		return err
	}
	volume, err := tx.First(TableDynamicHostVolumes, indexID, candidate.Namespace, candidate.VolumeID)
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	if volume == nil || volume.(*structs.DynamicHostVolume).Name != candidate.VolumeName {
		return ErrDynamicHostVolumeStateConflict
	}
	raw, err := dynamicHostVolumeExisting(tx, TableDynamicHostVolumeClaims, candidate.Namespace, candidate.ID, candidate.ModifyIndex)
	if err != nil {
		return err
	}
	claim := candidate.Copy()
	if raw != nil {
		existing := raw.(*structs.DynamicHostVolumeClaim)
		if !dynamicHostVolumeClaimIdentityEqual(existing, claim) {
			return ErrDynamicHostVolumeStateConflict
		}
	}
	claim.CreateIndex, err = dynamicHostVolumeCreateIndex(raw, claim.CreateIndex, index)
	if err != nil {
		return err
	}
	claim.ModifyIndex = index
	if err := tx.Insert(TableDynamicHostVolumeClaims, claim); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

// upsertPlanResultsClaims writes the claims a plan creates, inside the
// transaction that writes its allocations. A claim is skipped, not refused,
// when the group already holds a claim on the volume (two plans can race to
// create one) or when the job or the volume record has gone since the plan was
// made, so the allocations are never lost to a claim. The table index is
// bumped only when a claim is written, and never moves backwards. The index is
// not checked against the table: the plan applier applies a plan to its own
// snapshot at the applied Raft index plus one, which can trail the state by an
// entry, and a refusal would drop the plan's allocations from that snapshot.
// Raft ordering already applies the committed entries in order.
func upsertPlanResultsClaims(tx *txn, index uint64, claims []*structs.DynamicHostVolumeClaim) error {
	if len(claims) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(claims))
	written := 0
	for _, candidate := range claims {
		if candidate == nil || candidate.Validate() != nil {
			return ErrDynamicHostVolumeStateInvalid
		}
		skip, err := planResultsClaimSkipped(tx, candidate)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if err := upsertDynamicHostVolumeClaimTxn(tx, index, candidate, seen); err != nil {
			return err
		}
		written++
	}
	if written == 0 {
		return nil
	}
	raw, err := tx.First(tableIndex, indexID, TableDynamicHostVolumeClaims)
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	if raw != nil && raw.(*IndexEntry).Value >= index {
		return nil
	}
	if err := tx.Insert(tableIndex, indexEntry(TableDynamicHostVolumeClaims, index)); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

// planResultsClaimSkipped reports whether a plan's claim is left unwritten.
func planResultsClaimSkipped(tx *txn, candidate *structs.DynamicHostVolumeClaim) (bool, error) {
	// A claim for a job that has gone since the plan was made would never be
	// removed, because job removal is what clears a job's claims.
	job, err := tx.First("jobs", "id", candidate.Namespace, candidate.JobID)
	if err != nil {
		return false, ErrDynamicHostVolumeStateStorage
	}
	if job == nil {
		return true, nil
	}
	volume, err := tx.First(TableDynamicHostVolumes, indexID, candidate.Namespace, candidate.VolumeID)
	if err != nil {
		return false, ErrDynamicHostVolumeStateStorage
	}
	if volume == nil || volume.(*structs.DynamicHostVolume).Name != candidate.VolumeName {
		return true, nil
	}
	existing, err := tx.Get(TableDynamicHostVolumeClaims, indexJob+"_prefix", candidate.Namespace, candidate.JobID, "")
	if err != nil {
		return false, ErrDynamicHostVolumeStateStorage
	}
	for raw := existing.Next(); raw != nil; raw = existing.Next() {
		claim := raw.(*structs.DynamicHostVolumeClaim)
		if claim.TaskGroupName == candidate.TaskGroupName && claim.VolumeName == candidate.VolumeName &&
			claim.VolumeID == candidate.VolumeID {
			return true, nil
		}
	}
	return false, nil
}

func dynamicHostVolumeClaimIdentityEqual(left, right *structs.DynamicHostVolumeClaim) bool {
	return left.ID == right.ID && left.Namespace == right.Namespace && left.JobID == right.JobID &&
		left.TaskGroupName == right.TaskGroupName && left.VolumeName == right.VolumeName &&
		left.VolumeID == right.VolumeID && left.AllocationIndex == right.AllocationIndex && left.CreateIndex == right.CreateIndex
}

func (s *StateStore) deleteDynamicHostVolumeRecords(index uint64, table string, records []DynamicHostVolumeDelete) error {
	return s.writeDynamicHostVolumeBatch(index, table, len(records), func(tx *txn, position int, seen map[string]bool) error {
		key := records[position]
		if err := dynamicHostVolumeBatchKey(seen, key.Namespace, key.ID); err != nil {
			return err
		}
		if key.ExpectedModifyIndex == 0 {
			return ErrDynamicHostVolumeStateCAS
		}
		raw, err := dynamicHostVolumeExisting(tx, table, key.Namespace, key.ID, key.ExpectedModifyIndex)
		if err != nil {
			return err
		}
		if table == TableDynamicHostVolumes {
			return deleteDynamicHostVolumeTxn(tx, index, raw.(*structs.DynamicHostVolume))
		}
		// A claim that an allocation still holds stays, so a replayed or
		// racing entry cannot release a volume identity that is in use.
		inUse, err := dynamicHostVolumeClaimInUse(tx, raw.(*structs.DynamicHostVolumeClaim))
		if err != nil {
			return err
		}
		if inUse {
			return ErrDynamicHostVolumeStateInUse
		}
		if err := tx.Delete(table, raw); err != nil {
			return ErrDynamicHostVolumeStateStorage
		}
		return nil
	})
}

// deleteDynamicHostVolumeTxn deletes a volume record and every claim on it. It
// refuses while a live allocation uses the volume. The claims table index is
// checked and bumped only when a claim is removed, so a volume without claims
// is never refused because of that index.
func deleteDynamicHostVolumeTxn(tx *txn, index uint64, vol *structs.DynamicHostVolume) error {
	inUse, err := dynamicHostVolumeInUse(tx, vol)
	if err != nil {
		return err
	}
	if inUse {
		return ErrDynamicHostVolumeStateInUse
	}
	iter, err := tx.Get(TableDynamicHostVolumeClaims, dynamicHostVolumeClaimVolumeIndex+"_prefix", vol.Namespace, vol.ID, "")
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	var claims []interface{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claims = append(claims, raw)
	}
	if len(claims) > 0 {
		// The index may equal the current one when an earlier volume of the
		// same batch (one Raft index) bumped it.
		raw, err := tx.First(tableIndex, indexID, TableDynamicHostVolumeClaims)
		if err != nil {
			return ErrDynamicHostVolumeStateStorage
		}
		if index == 0 || (raw != nil && index < raw.(*IndexEntry).Value) {
			return ErrDynamicHostVolumeStateIndex
		}
	}
	if err := deleteDynamicHostVolumeClaimsTxn(tx, index, claims); err != nil {
		return err
	}
	if err := tx.Delete(TableDynamicHostVolumes, vol); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

// dynamicHostVolumeInUse reports whether an allocation on the volume's node
// uses it. An allocation counts until the client reports it terminal, the
// same release point the scheduler's proposed allocations use, so one the
// server has stopped but the client still runs keeps the volume.
func dynamicHostVolumeInUse(tx *txn, vol *structs.DynamicHostVolume) (bool, error) {
	// An empty node ID would scan every allocation, and no allocation uses a
	// volume that is not on a node.
	if vol.NodeID == "" {
		return false, nil
	}
	allocs, err := allocsByNodeTxn(tx, nil, vol.NodeID)
	if err != nil {
		return false, ErrDynamicHostVolumeStateStorage
	}
	for _, alloc := range allocs {
		// The node index is a prefix index, so another node's allocations can
		// be returned as well.
		if alloc.NodeID != vol.NodeID || alloc.ClientTerminalStatus() {
			continue
		}
		if len(structs.DynamicHostVolumeAllocRequests(vol, alloc, nil)) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// dynamicHostVolumeClaimInUse reports whether an allocation holds the claim:
// one of the claim's namespace, job and task group that is not client-terminal
// and uses the claim's volume on the volume's node. The claim records only the
// first allocation of the group, and replacements reuse the claim without
// updating it, so the recorded allocation alone would let a claim in use be
// deleted. The release point is the one of dynamicHostVolumeInUse. A claim
// whose volume record is missing is not expected, because removing a volume
// removes its claims; it falls back to the recorded allocation.
func dynamicHostVolumeClaimInUse(tx *txn, claim *structs.DynamicHostVolumeClaim) (bool, error) {
	raw, err := tx.First(TableDynamicHostVolumes, indexID, claim.Namespace, claim.VolumeID)
	if err != nil {
		return false, ErrDynamicHostVolumeStateStorage
	}
	if raw == nil {
		recorded, err := tx.First("allocs", "id", claim.AllocationID)
		if err != nil {
			return false, ErrDynamicHostVolumeStateStorage
		}
		if recorded == nil {
			return false, nil
		}
		alloc := recorded.(*structs.Allocation)
		return alloc.Namespace == claim.Namespace && !alloc.ClientTerminalStatus(), nil
	}
	vol := raw.(*structs.DynamicHostVolume)
	if vol.NodeID == "" {
		return false, nil
	}
	iter, err := tx.Get("allocs", "job", claim.Namespace, claim.JobID)
	if err != nil {
		return false, ErrDynamicHostVolumeStateStorage
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		alloc := raw.(*structs.Allocation)
		if alloc.TaskGroup != claim.TaskGroupName || alloc.NodeID != vol.NodeID || alloc.ClientTerminalStatus() {
			continue
		}
		if len(structs.DynamicHostVolumeAllocRequests(vol, alloc, nil)) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// DynamicHostVolumeClaimInUse reports whether an allocation holds the claim, by
// the rule the claim delete enforces when it is applied.
func (s *StateStore) DynamicHostVolumeClaimInUse(claim *structs.DynamicHostVolumeClaim) (bool, error) {
	tx := s.db.ReadTxn()
	defer tx.Abort()
	return dynamicHostVolumeClaimInUse(tx, claim)
}

// deleteDynamicHostVolumeClaimsByJobTxn deletes the claims of one job when the
// job leaves state. The job index is a prefix index whose parts are
// null-terminated, so a job ID matches exactly.
func deleteDynamicHostVolumeClaimsByJobTxn(tx *txn, index uint64, namespace, jobID string) error {
	iter, err := tx.Get(TableDynamicHostVolumeClaims, indexJob+"_prefix", namespace, jobID, "")
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	var claims []interface{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		claims = append(claims, raw)
	}
	return deleteDynamicHostVolumeClaimsTxn(tx, index, claims)
}

// deleteDynamicHostVolumeClaimsTxn deletes the collected claims (never delete
// while walking an iterator) and bumps the claims table index when there were
// any. Job removal does not check the index, like the other tables it clears:
// Raft indexes already arrive in order, and a refusal would fail the purge.
func deleteDynamicHostVolumeClaimsTxn(tx *txn, index uint64, claims []interface{}) error {
	if len(claims) == 0 {
		return nil
	}
	for _, claim := range claims {
		if err := tx.Delete(TableDynamicHostVolumeClaims, claim); err != nil {
			return ErrDynamicHostVolumeStateStorage
		}
	}
	if err := tx.Insert(tableIndex, indexEntry(TableDynamicHostVolumeClaims, index)); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

func (s *StateStore) DeleteDynamicHostVolumes(index uint64, records []DynamicHostVolumeDelete) error {
	return s.deleteDynamicHostVolumeRecords(index, TableDynamicHostVolumes, records)
}

func (s *StateStore) DeleteDynamicHostVolumeClaims(index uint64, records []DynamicHostVolumeClaimDelete) error {
	return s.deleteDynamicHostVolumeRecords(index, TableDynamicHostVolumeClaims, records)
}

type dynamicHostVolumeCopyable[T any] interface{ Copy() T }

type dynamicHostVolumeCopyIterator[T dynamicHostVolumeCopyable[T]] struct {
	memdb.ResultIterator
}

func (iter *dynamicHostVolumeCopyIterator[T]) Next() interface{} {
	raw := iter.ResultIterator.Next()
	if raw == nil {
		return nil
	}
	return raw.(T).Copy()
}

func dynamicHostVolumeByID[T dynamicHostVolumeCopyable[T]](s *StateStore, ws memdb.WatchSet, table, namespace, id string) (T, error) {
	var zero T
	if err := dynamicHostVolumeKeys(2, namespace, id); err != nil {
		return zero, err
	}
	tx := s.db.ReadTxn()
	watch, raw, err := tx.FirstWatch(table, indexID, namespace, id)
	if err != nil {
		return zero, ErrDynamicHostVolumeStateStorage
	}
	ws.Add(watch)
	if raw == nil {
		return zero, nil
	}
	return raw.(T).Copy(), nil
}

func dynamicHostVolumeRecords[T dynamicHostVolumeCopyable[T]](s *StateStore, ws memdb.WatchSet, table, index string, args ...string) (memdb.ResultIterator, error) {
	required := len(args)
	if strings.HasSuffix(index, "_prefix") {
		required--
	}
	if err := dynamicHostVolumeKeys(required, args...); err != nil {
		return nil, err
	}
	values := make([]interface{}, len(args))
	for position, arg := range args {
		values[position] = arg
	}
	tx := s.db.ReadTxn()
	raw, err := tx.Get(table, index, values...)
	if err != nil {
		return nil, ErrDynamicHostVolumeStateStorage
	}
	iter := &dynamicHostVolumeCopyIterator[T]{ResultIterator: raw}
	ws.Add(iter.WatchCh())
	return iter, nil
}

func (s *StateStore) DynamicHostVolumeByID(ws memdb.WatchSet, namespace, id string) (*structs.DynamicHostVolume, error) {
	return dynamicHostVolumeByID[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, namespace, id)
}

// DynamicHostVolumeByNodeName returns the record a node holds under a name in
// a namespace, or nil. The pair is unique, so it finds the record of a source
// the node no longer advertises.
func (s *StateStore) DynamicHostVolumeByNodeName(ws memdb.WatchSet, namespace, nodeID, name string) (*structs.DynamicHostVolume, error) {
	if err := dynamicHostVolumeKeys(3, namespace, nodeID, name); err != nil {
		return nil, err
	}
	tx := s.db.ReadTxn()
	watch, raw, err := tx.FirstWatch(TableDynamicHostVolumes, dynamicHostVolumeNodeNameIndex, namespace, nodeID, name)
	if err != nil {
		return nil, ErrDynamicHostVolumeStateStorage
	}
	ws.Add(watch)
	if raw == nil {
		return nil, nil
	}
	return raw.(*structs.DynamicHostVolume).Copy(), nil
}

func (s *StateStore) DynamicHostVolumeClaimByID(ws memdb.WatchSet, namespace, id string) (*structs.DynamicHostVolumeClaim, error) {
	return dynamicHostVolumeByID[*structs.DynamicHostVolumeClaim](s, ws, TableDynamicHostVolumeClaims, namespace, id)
}
func (s *StateStore) DynamicHostVolumes(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, indexID)
}
func (s *StateStore) DynamicHostVolumeClaims(ws memdb.WatchSet) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolumeClaim](s, ws, TableDynamicHostVolumeClaims, indexID)
}
func (s *StateStore) DynamicHostVolumesByIDPrefix(ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, indexID+"_prefix", namespace, prefix)
}
func (s *StateStore) DynamicHostVolumesByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	return s.DynamicHostVolumesByIDPrefix(ws, namespace, "")
}
func (s *StateStore) DynamicHostVolumesByNodeID(ws memdb.WatchSet, namespace, node string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, indexNodeID+"_prefix", namespace, node, "")
}

// DynamicHostVolumesByNode returns the volumes recorded on a node in every
// namespace. The node read API uses it to attribute an advertised volume to a
// namespace.
func (s *StateStore) DynamicHostVolumesByNode(ws memdb.WatchSet, nodeID string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, dynamicHostVolumeNodeOnlyIndex, nodeID)
}

func (s *StateStore) DynamicHostVolumesByPluginID(ws memdb.WatchSet, namespace, plugin string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, dynamicHostVolumePluginIndex+"_prefix", namespace, plugin, "")
}
func (s *StateStore) DynamicHostVolumesByNodeIDPrefix(ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, indexNodeID+"_prefix", namespace, prefix)
}
func (s *StateStore) DynamicHostVolumesByPluginIDPrefix(ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolume](s, ws, TableDynamicHostVolumes, dynamicHostVolumePluginIndex+"_prefix", namespace, prefix)
}
func (s *StateStore) DynamicHostVolumeClaimsByIDPrefix(ws memdb.WatchSet, namespace, prefix string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolumeClaim](s, ws, TableDynamicHostVolumeClaims, indexID+"_prefix", namespace, prefix)
}
func (s *StateStore) DynamicHostVolumeClaimsByNamespace(ws memdb.WatchSet, namespace string) (memdb.ResultIterator, error) {
	return s.DynamicHostVolumeClaimsByIDPrefix(ws, namespace, "")
}
func (s *StateStore) DynamicHostVolumeClaimsByVolumeID(ws memdb.WatchSet, namespace, volume string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolumeClaim](s, ws, TableDynamicHostVolumeClaims, dynamicHostVolumeClaimVolumeIndex+"_prefix", namespace, volume, "")
}
func (s *StateStore) DynamicHostVolumeClaimsByJobID(ws memdb.WatchSet, namespace, job string) (memdb.ResultIterator, error) {
	return dynamicHostVolumeRecords[*structs.DynamicHostVolumeClaim](s, ws, TableDynamicHostVolumeClaims, indexJob+"_prefix", namespace, job, "")
}

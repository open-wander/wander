// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/open-wander/wander/nomad/structs"
)

// ErrDynamicHostVolumeStateTransition is wrapped by every refused change of
// state. The message names only the current state, never a record value.
var ErrDynamicHostVolumeStateTransition = errors.New("dynamic host volume state: transition refused")

// dynamicHostVolumeTransitions is the single table of allowed changes of state;
// the empty state is a volume that does not exist yet. The ready and
// unavailable entries are listed although no command here moves a volume
// between them: a later task derives them from node updates and must not be
// blocked by this table.
var dynamicHostVolumeTransitions = map[structs.DynamicHostVolumeState]map[structs.DynamicHostVolumeState]bool{
	structs.DynamicHostVolumeStateUnknown: {
		structs.DynamicHostVolumeStateRequested: true,
		structs.DynamicHostVolumeStatePending:   true,
	},
	structs.DynamicHostVolumeStateRequested: {
		structs.DynamicHostVolumeStatePending:  true,
		structs.DynamicHostVolumeStateFailed:   true,
		structs.DynamicHostVolumeStateDeleting: true,
	},
	structs.DynamicHostVolumeStatePending: {
		structs.DynamicHostVolumeStateReady:    true,
		structs.DynamicHostVolumeStateFailed:   true,
		structs.DynamicHostVolumeStateDeleting: true,
	},
	structs.DynamicHostVolumeStateReady: {
		structs.DynamicHostVolumeStateReady:       true,
		structs.DynamicHostVolumeStateUnavailable: true,
		structs.DynamicHostVolumeStateDeleting:    true,
	},
	structs.DynamicHostVolumeStateUnavailable: {
		structs.DynamicHostVolumeStateReady:    true,
		structs.DynamicHostVolumeStateDeleting: true,
	},
	structs.DynamicHostVolumeStateFailed: {
		structs.DynamicHostVolumeStateDeleting: true,
	},
	structs.DynamicHostVolumeStateDeleting: {
		structs.DynamicHostVolumeStateDeleting: true,
	},
}

func dynamicHostVolumeTransitionAllowed(from, to structs.DynamicHostVolumeState) bool {
	return dynamicHostVolumeTransitions[from][to]
}

// dynamicHostVolumeTransitionError refuses a change from the current state. A
// state is quoted only when it is one the table knows, so a record value never
// reaches the message.
func dynamicHostVolumeTransitionError(from structs.DynamicHostVolumeState) error {
	name := "none"
	if from != structs.DynamicHostVolumeStateUnknown {
		name = "unknown"
		if _, known := dynamicHostVolumeTransitions[from]; known {
			name = string(from)
		}
	}
	return fmt.Errorf("%w: current state %q", ErrDynamicHostVolumeStateTransition, name)
}

// dynamicHostVolumeWriteLookup checks the key and its namespace, refuses a
// second write of the same volume in one batch, and returns the stored record
// or nil.
func dynamicHostVolumeWriteLookup(tx *txn, seen map[string]bool, namespace, id string) (*structs.DynamicHostVolume, error) {
	if err := dynamicHostVolumeWriteKey(tx, seen, namespace, id); err != nil {
		return nil, err
	}
	return dynamicHostVolumeLookup(tx, namespace, id)
}

func dynamicHostVolumeLookup(tx *txn, namespace, id string) (*structs.DynamicHostVolume, error) {
	raw, err := tx.First(TableDynamicHostVolumes, indexID, namespace, id)
	if err != nil {
		return nil, ErrDynamicHostVolumeStateStorage
	}
	if raw == nil {
		return nil, nil
	}
	return raw.(*structs.DynamicHostVolume), nil
}

func dynamicHostVolumeCandidate(candidate *structs.DynamicHostVolume) error {
	if candidate == nil || candidate.Type != structs.VolumeTypeHost {
		return ErrDynamicHostVolumeStateInvalid
	}
	return dynamicHostVolumeKeys(3, candidate.Namespace, candidate.ID, candidate.Name, candidate.NodeID, candidate.PluginID)
}

// dynamicHostVolumeSameContent reports whether a create or register repeats
// what is stored. A retried request carries new timestamps and may arrive after
// the volume has moved on, so the indexes, times and state are ignored, and so
// is any server-filled field (node, path, capacity, diagnostic, attachments)
// that the request leaves unset.
func dynamicHostVolumeSameContent(existing, candidate *structs.DynamicHostVolume) bool {
	left, right := existing.Copy(), candidate.Copy()
	for _, v := range []*structs.DynamicHostVolume{left, right} {
		v.CreateIndex, v.ModifyIndex, v.CreateTime, v.ModifyTime = 0, 0, 0, 0
		v.State = structs.DynamicHostVolumeStateUnknown
	}
	if right.NodeID == "" {
		left.NodeID = ""
	}
	if right.HostPath == "" {
		left.HostPath = ""
	}
	if right.CapacityBytes == 0 {
		left.CapacityBytes = 0
	}
	if right.StatusDescription == "" {
		left.StatusDescription = ""
	}
	if len(right.Attachments) == 0 {
		left.Attachments, right.Attachments = nil, nil
	}
	return left.Equal(right)
}

// CreateDynamicHostVolumes is the upsert command: it creates volumes as
// requested (no node yet) or pending (node known), or moves a requested volume
// to pending by giving it its node. Repeating the same content is a no-op that
// leaves the record alone; the table index still moves with the Raft index.
func (s *StateStore) CreateDynamicHostVolumes(index uint64, volumes []*structs.DynamicHostVolume) error {
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumes, len(volumes), func(tx *txn, position int, seen map[string]bool) error {
		candidate := volumes[position]
		if err := dynamicHostVolumeCandidate(candidate); err != nil {
			return err
		}
		existing, err := dynamicHostVolumeWriteLookup(tx, seen, candidate.Namespace, candidate.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return createDynamicHostVolumeTxn(tx, index, candidate)
		}
		if dynamicHostVolumeSameContent(existing, candidate) {
			return nil
		}
		return assignDynamicHostVolumeNodeTxn(tx, index, existing, candidate)
	})
}

func createDynamicHostVolumeTxn(tx *txn, index uint64, candidate *structs.DynamicHostVolume) error {
	if !dynamicHostVolumeTransitionAllowed(structs.DynamicHostVolumeStateUnknown, candidate.State) {
		return dynamicHostVolumeTransitionError(structs.DynamicHostVolumeStateUnknown)
	}
	// A requested volume waits for a node and a pending one has it.
	if (candidate.State == structs.DynamicHostVolumeStatePending) == (candidate.NodeID == "") {
		return ErrDynamicHostVolumeStateInvalid
	}
	// A new record has no modify index to compare with.
	if _, err := dynamicHostVolumeExisting(tx, TableDynamicHostVolumes, candidate.Namespace, candidate.ID, candidate.ModifyIndex); err != nil {
		return err
	}
	if candidate.CreateIndex != 0 {
		return ErrDynamicHostVolumeStateInvalid
	}
	volume := candidate.Copy()
	volume.CreateIndex = index
	volume.StatusDescription = boundDynamicHostVolumeDiagnostic(volume.StatusDescription)
	return insertDynamicHostVolumeTxn(tx, index, volume)
}

// assignDynamicHostVolumeNodeTxn moves a requested volume to pending. NodeID is
// immutable everywhere else, so the generic check runs against a candidate whose
// node is set back to the empty one, and the move may change nothing besides the
// node, the state and the times.
func assignDynamicHostVolumeNodeTxn(tx *txn, index uint64, existing, candidate *structs.DynamicHostVolume) error {
	if existing.State != structs.DynamicHostVolumeStateRequested || existing.NodeID != "" ||
		candidate.State != structs.DynamicHostVolumeStatePending || candidate.NodeID == "" ||
		!dynamicHostVolumeTransitionAllowed(existing.State, candidate.State) {
		return dynamicHostVolumeTransitionError(existing.State)
	}
	if _, err := dynamicHostVolumeExisting(tx, TableDynamicHostVolumes, existing.Namespace, existing.ID, candidate.ModifyIndex); err != nil {
		return err
	}
	probe := candidate.Copy()
	probe.NodeID = existing.NodeID
	if err := existing.ValidateImmutableFields(probe); err != nil {
		return fmt.Errorf("volume identity: %w", err)
	}
	if !dynamicHostVolumeSameContent(existing, probe) {
		return ErrDynamicHostVolumeStateConflict
	}
	volume := candidate.Copy()
	volume.StatusDescription = boundDynamicHostVolumeDiagnostic(volume.StatusDescription)
	return insertDynamicHostVolumeTxn(tx, index, volume)
}

// boundDynamicHostVolumeDiagnostic cuts a diagnostic to the stored bound at a
// rune boundary. The result depends only on the input, so every server stores
// the same bytes.
func boundDynamicHostVolumeDiagnostic(diagnostic string) string {
	diagnostic = strings.ToValidUTF8(diagnostic, "�")
	if len(diagnostic) <= structs.DynamicHostVolumeMaxDiagnosticBytes {
		return diagnostic
	}
	cut := structs.DynamicHostVolumeMaxDiagnosticBytes
	for cut > 0 && !utf8.RuneStart(diagnostic[cut]) {
		cut--
	}
	return diagnostic[:cut]
}

// RecordDynamicHostVolumeOutcome records the result of the client call: ready
// (from pending only, with the host path and capacity) or failed (from requested
// or pending, with a diagnostic). Repeating the recorded outcome is a no-op.
func (s *StateStore) RecordDynamicHostVolumeOutcome(index uint64, req *structs.DynamicHostVolumeOutcomeRequest) error {
	if req == nil {
		return ErrDynamicHostVolumeStateInvalid
	}
	diagnostic := boundDynamicHostVolumeDiagnostic(req.Diagnostic)
	switch req.State {
	case structs.DynamicHostVolumeStateReady:
		if req.HostPath == "" || !filepath.IsAbs(req.HostPath) || strings.ContainsRune(req.HostPath, 0) || req.CapacityBytes < 0 {
			return ErrDynamicHostVolumeStateInvalid
		}
	case structs.DynamicHostVolumeStateFailed:
		if diagnostic == "" {
			return ErrDynamicHostVolumeStateInvalid
		}
	default:
		return ErrDynamicHostVolumeStateInvalid
	}
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumes, 1, func(tx *txn, _ int, seen map[string]bool) error {
		existing, err := dynamicHostVolumeWriteLookup(tx, seen, req.Key.Namespace, req.Key.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return dynamicHostVolumeTransitionError(structs.DynamicHostVolumeStateUnknown)
		}
		if existing.State == req.State {
			if req.State == structs.DynamicHostVolumeStateReady && existing.HostPath == req.HostPath && existing.CapacityBytes == req.CapacityBytes {
				return nil
			}
			if req.State == structs.DynamicHostVolumeStateFailed && existing.StatusDescription == diagnostic {
				return nil
			}
			return dynamicHostVolumeTransitionError(existing.State)
		}
		// The outcome of a client call only ever follows a pending volume. The
		// table also lets ready repeat itself (the update command) and lets an
		// unavailable volume return to ready (derived from node updates).
		if req.State == structs.DynamicHostVolumeStateReady && existing.State != structs.DynamicHostVolumeStatePending ||
			!dynamicHostVolumeTransitionAllowed(existing.State, req.State) {
			return dynamicHostVolumeTransitionError(existing.State)
		}
		volume := existing.Copy()
		volume.State, volume.ModifyTime = req.State, req.ModifyTime
		if req.State == structs.DynamicHostVolumeStateReady {
			volume.HostPath, volume.CapacityBytes, volume.StatusDescription = req.HostPath, req.CapacityBytes, ""
		} else {
			volume.StatusDescription = diagnostic
		}
		return insertDynamicHostVolumeTxn(tx, index, volume)
	})
}

// UpdateDynamicHostVolume changes the mutable fields of a ready volume. The
// volume's ModifyIndex is the compare-and-set index. Identity is held by the
// generic immutability check, and the state can only change through the other
// commands. The host path, capacity, diagnostic and attachments belong to the
// server and the client: an update that carries a different value for one of
// them is refused rather than silently ignored, so a caller that believes it
// changed them learns it did not. The stored record is built from the stored
// copy, so none of those fields can be rewritten through this command.
func (s *StateStore) UpdateDynamicHostVolume(index uint64, candidate *structs.DynamicHostVolume) error {
	if err := dynamicHostVolumeCandidate(candidate); err != nil {
		return err
	}
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumes, 1, func(tx *txn, _ int, seen map[string]bool) error {
		existing, err := dynamicHostVolumeWriteLookup(tx, seen, candidate.Namespace, candidate.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return dynamicHostVolumeTransitionError(structs.DynamicHostVolumeStateUnknown)
		}
		if existing.State != structs.DynamicHostVolumeStateReady || candidate.State != structs.DynamicHostVolumeStateReady {
			return dynamicHostVolumeTransitionError(existing.State)
		}
		if _, err := dynamicHostVolumeExisting(tx, TableDynamicHostVolumes, candidate.Namespace, candidate.ID, candidate.ModifyIndex); err != nil {
			return err
		}
		if err := existing.ValidateImmutableFields(candidate); err != nil {
			return fmt.Errorf("volume identity: %w", err)
		}
		if err := dynamicHostVolumeServerOwnedUnchanged(existing, candidate); err != nil {
			return err
		}
		volume := existing.Copy()
		volume.NodePool, volume.PluginID = candidate.NodePool, candidate.PluginID
		volume.RequestedCapacityMinBytes = candidate.RequestedCapacityMinBytes
		volume.RequestedCapacityMaxBytes = candidate.RequestedCapacityMaxBytes
		volume.RequestedCapabilities = candidate.Copy().RequestedCapabilities
		volume.Constraints = candidate.Copy().Constraints
		volume.Parameters = candidate.Copy().Parameters
		volume.ModifyTime = candidate.ModifyTime
		return insertDynamicHostVolumeTxn(tx, index, volume)
	})
}

// dynamicHostVolumeServerOwnedUnchanged refuses a candidate that differs from
// the stored record in a field only the server or the client may write. The
// error names the field, never its value.
func dynamicHostVolumeServerOwnedUnchanged(existing, candidate *structs.DynamicHostVolume) error {
	for _, field := range []struct {
		name string
		same bool
	}{
		{"HostPath", existing.HostPath == candidate.HostPath},
		{"CapacityBytes", existing.CapacityBytes == candidate.CapacityBytes},
		{"StatusDescription", existing.StatusDescription == candidate.StatusDescription},
		{"Attachments", slices.EqualFunc(existing.Attachments, candidate.Attachments,
			func(a, b *structs.DynamicHostVolumeAttachment) bool { return a.Equal(b) })},
	} {
		if !field.same {
			return fmt.Errorf("volume server-owned field %s: %w", field.name, structs.ErrDynamicHostVolumeImmutable)
		}
	}
	return nil
}

// MarkDynamicHostVolumesDeleting moves volumes to deleting. A volume already
// deleting is left untouched, so its modify index still matches a later
// removal. The whole batch is refused if one volume cannot be marked.
func (s *StateStore) MarkDynamicHostVolumesDeleting(index uint64, keys []structs.DynamicHostVolumeKey, modifyTime int64) error {
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumes, len(keys), func(tx *txn, position int, seen map[string]bool) error {
		key := keys[position]
		existing, err := dynamicHostVolumeWriteLookup(tx, seen, key.Namespace, key.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return dynamicHostVolumeTransitionError(structs.DynamicHostVolumeStateUnknown)
		}
		if existing.State == structs.DynamicHostVolumeStateDeleting {
			return nil
		}
		if !dynamicHostVolumeTransitionAllowed(existing.State, structs.DynamicHostVolumeStateDeleting) {
			return dynamicHostVolumeTransitionError(existing.State)
		}
		volume := existing.Copy()
		volume.State, volume.ModifyTime = structs.DynamicHostVolumeStateDeleting, modifyTime
		return insertDynamicHostVolumeTxn(tx, index, volume)
	})
}

// RemoveDynamicHostVolumes removes volumes that are deleting, with their sticky
// claims in the same transaction. A volume that is already gone is skipped so a
// retry after a lost reply succeeds. A live allocation that uses a volume still
// refuses its removal.
func (s *StateStore) RemoveDynamicHostVolumes(index uint64, keys []structs.DynamicHostVolumeKey) error {
	return s.writeDynamicHostVolumeBatch(index, TableDynamicHostVolumes, len(keys), func(tx *txn, position int, seen map[string]bool) error {
		key := keys[position]
		if err := dynamicHostVolumeBatchKey(seen, key.Namespace, key.ID); err != nil {
			return err
		}
		existing, err := dynamicHostVolumeLookup(tx, key.Namespace, key.ID)
		if err != nil {
			return err
		}
		if existing == nil {
			return nil
		}
		if existing.State != structs.DynamicHostVolumeStateDeleting {
			return dynamicHostVolumeTransitionError(existing.State)
		}
		return deleteDynamicHostVolumeTxn(tx, index, existing)
	})
}

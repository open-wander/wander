// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"github.com/open-wander/wander/nomad/structs"
)

// deriveDynamicHostVolumesForNodeTxn moves the dynamic host volumes of one node
// between ready and unavailable, inside the transaction that writes the node,
// so the volume and the node change in the same Raft apply. A nil node means
// the node has been deleted.
//
// A ready volume becomes unavailable when the node is down or gone, or no
// longer advertises the volume. An unavailable volume becomes ready when the
// node is not down and advertises it. Volumes are matched by ID only: a static
// volume has an empty ID and never counts, whatever its name. Volumes in any
// other state, and volumes on other nodes, are never changed.
//
// The result depends only on the node and the stored records. No clock is read,
// so ModifyTime and StatusDescription are left as they are. The table index is
// bumped only when a record changes, and never moves backwards.
//
// The caller's node write swallows errors and drops the whole update, so this
// skips anything that does not qualify and returns an error only when storage
// fails.
func deriveDynamicHostVolumesForNodeTxn(tx *txn, index uint64, nodeID string, node *structs.Node) error {
	volumes, err := dynamicHostVolumesOnNodeTxn(tx, nodeID)
	if err != nil {
		return err
	}
	if len(volumes) == 0 {
		return nil
	}

	// Only membership is tested, so the order of the map never reaches the
	// records.
	down := node == nil || node.UnresponsiveStatus()
	advertised := make(map[string]bool)
	if node != nil {
		for _, config := range node.HostVolumes {
			if config != nil && config.ID != "" {
				advertised[config.ID] = true
			}
		}
	}

	changed := 0
	for _, existing := range volumes {
		var next structs.DynamicHostVolumeState
		switch {
		case existing.State == structs.DynamicHostVolumeStateReady && (down || !advertised[existing.ID]):
			next = structs.DynamicHostVolumeStateUnavailable
		case existing.State == structs.DynamicHostVolumeStateUnavailable && !down && advertised[existing.ID]:
			next = structs.DynamicHostVolumeStateReady
		default:
			continue
		}
		// Refusing here would drop the node update with it, so a move the
		// transition table does not allow leaves the volume as it is.
		if !dynamicHostVolumeTransitionAllowed(existing.State, next) {
			continue
		}
		volume := existing.Copy()
		volume.State = next
		if err := insertDynamicHostVolumeTxn(tx, index, volume); err != nil {
			return err
		}
		changed++
	}
	if changed == 0 {
		return nil
	}

	raw, err := tx.First(tableIndex, indexID, TableDynamicHostVolumes)
	if err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	if raw != nil && raw.(*IndexEntry).Value >= index {
		return nil
	}
	if err := tx.Insert(tableIndex, indexEntry(TableDynamicHostVolumes, index)); err != nil {
		return ErrDynamicHostVolumeStateStorage
	}
	return nil
}

// dynamicHostVolumesOnNodeTxn collects the volumes of a node in every
// namespace, in node-only index order, before anything is written. An empty
// node ID matches nothing: a requested volume has no node and belongs to none.
func dynamicHostVolumesOnNodeTxn(tx *txn, nodeID string) ([]*structs.DynamicHostVolume, error) {
	if nodeID == "" {
		return nil, nil
	}
	iter, err := tx.Get(TableDynamicHostVolumes, dynamicHostVolumeNodeOnlyIndex, nodeID)
	if err != nil {
		return nil, ErrDynamicHostVolumeStateStorage
	}
	var volumes []*structs.DynamicHostVolume
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		volumes = append(volumes, raw.(*structs.DynamicHostVolume))
	}
	return volumes, nil
}

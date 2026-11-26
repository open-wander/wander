// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
)

// nodeHostVolumesForRead returns the host volumes of a node that the caller may
// see. Static volumes, which have no ID, are always kept. A dynamic volume is
// kept only when a record for it exists on the node and the caller may read host
// volumes in every namespace that holds such a record. An advertised ID with no
// record cannot be attributed to a namespace and is hidden.
//
// A nil ACL (ACLs disabled, or a client identity), a management ACL, and a node
// with no dynamic volumes are returned unchanged and cost no lookup. The bool
// reports whether the returned map differs from the input. The input is shared
// with the state store and is never modified: a filtered result is a new map,
// and stays non-nil when the input was non-nil.
//
// The lookup takes no watch set, so a change to a volume record alone does not
// wake a blocked node query.
func nodeHostVolumesForRead(aclObj *acl.ACL, store *state.StateStore, nodeID string, vols map[string]*structs.ClientHostVolumeConfig) (map[string]*structs.ClientHostVolumeConfig, bool, error) {
	if aclObj == nil || aclObj.IsManagement() || !hasDynamicHostVolume(vols) {
		return vols, false, nil
	}

	iter, err := store.DynamicHostVolumesByNode(nil, nodeID)
	if err != nil {
		return nil, false, err
	}
	// An ID is readable only while every record carrying it is readable.
	readable := make(map[string]bool)
	canRead := aclObj.AllowNsOpFunc(acl.NamespaceCapabilityHostVolumeRead)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		record := raw.(*structs.DynamicHostVolume)
		allowed := canRead(record.Namespace)
		if seen, ok := readable[record.ID]; ok {
			allowed = allowed && seen
		}
		readable[record.ID] = allowed
	}

	out := make(map[string]*structs.ClientHostVolumeConfig, len(vols))
	for name, vol := range vols {
		if vol == nil || vol.ID == "" || readable[vol.ID] {
			out[name] = vol
		}
	}
	return out, true, nil
}

func hasDynamicHostVolume(vols map[string]*structs.ClientHostVolumeConfig) bool {
	for _, vol := range vols {
		if vol != nil && vol.ID != "" {
			return true
		}
	}
	return false
}

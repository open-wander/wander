// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"github.com/open-wander/wander/nomad/structs"
)

// stripNodeEventDynamicHostVolumes returns the node an event carries, without
// its dynamic host volumes. The event broker builds one payload for every
// subscriber and the node topic needs only node:read, so the payload cannot
// follow the per-namespace rule of the node read API. Dynamic entries are left
// out for everyone, management included; static entries are kept. The input
// is never changed: a node is copied only when it has a dynamic entry.
func stripNodeEventDynamicHostVolumes(node *structs.Node) *structs.Node {
	if node == nil {
		return nil
	}
	dynamic := false
	for _, config := range node.HostVolumes {
		if config != nil && config.ID != "" {
			dynamic = true
			break
		}
	}
	if !dynamic {
		return node
	}

	out := node.Copy()
	out.HostVolumes = make(map[string]*structs.ClientHostVolumeConfig, len(node.HostVolumes))
	for name, config := range node.HostVolumes {
		if config == nil || config.ID == "" {
			out.HostVolumes[name] = config.Copy()
		}
	}
	return out
}

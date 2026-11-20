// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	log "github.com/hashicorp/go-hclog"

	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/scheduler"
)

// dynamicHostVolumePluginAttributePrefix is the node attribute prefix a client
// uses to advertise a healthy host volume plugin: the prefix plus the plugin
// ID, with the plugin version as the value.
const dynamicHostVolumePluginAttributePrefix = "plugins.host_volume.version."

// Stable reason codes. Typed errors do not survive the RPC transport, so the
// codes are plain strings.
const (
	// DynamicHostVolumeNoEligibleNode is the top-level reason of a diagnostic.
	DynamicHostVolumeNoEligibleNode = "no_eligible_node"

	// Per-reason filter codes, in the order the rules are applied.
	DynamicHostVolumeReasonNodePool        = "node_pool"
	DynamicHostVolumeReasonNodeNotReady    = "node_not_ready"
	DynamicHostVolumeReasonNodeIneligible  = "node_ineligible"
	DynamicHostVolumeReasonNodeDraining    = "node_draining"
	DynamicHostVolumeReasonPluginUnhealthy = "plugin_unhealthy"
	DynamicHostVolumeReasonConstraint      = "constraint"
	DynamicHostVolumeReasonNameConflict    = "name_conflict"
)

// Rejections of the request itself.
var (
	ErrDynamicHostVolumeNodeNotFound     = errors.New("node_not_found")
	ErrDynamicHostVolumeNodePoolMismatch = errors.New("node_pool_mismatch")
)

// DynamicHostVolumeNodeReasonCount is the number of nodes filtered for one reason.
type DynamicHostVolumeNodeReasonCount struct {
	Reason string
	Count  int
}

// DynamicHostVolumeNodeDiagnostic explains why no node is eligible. It holds
// only reason codes and counts, never node IDs, constraint text or parameters.
type DynamicHostVolumeNodeDiagnostic struct {
	Reason   string
	Filtered []DynamicHostVolumeNodeReasonCount
}

// selectDynamicHostVolumeNode picks the node on which a volume is created. It
// reads the snapshot only. A request that cannot be honoured returns a
// sentinel error; when no node passes the rules the diagnostic is returned.
//
// Each node is counted once, under the first rule it fails: node pool (when no
// node ID is given), readiness, scheduling eligibility, drain, plugin health,
// constraints, then name conflict. An empty or "all" node pool means every
// pool. Among eligible nodes the one with the fewest dynamic volume records
// wins, then the lowest node ID (Wander policy).
//
// A volume's own record and its entry in the node's host volumes count as a
// name conflict, so this is for creation only: re-running it for an existing
// volume reports name_conflict on the node that holds it.
func selectDynamicHostVolumeNode(snap *state.StateSnapshot, vol *structs.DynamicHostVolume) (*structs.Node, *DynamicHostVolumeNodeDiagnostic, error) {
	var candidates []*structs.Node
	explicit := vol.NodeID != ""
	anyPool := vol.NodePool == "" || vol.NodePool == structs.NodePoolAll

	if explicit {
		node, err := snap.NodeByID(nil, vol.NodeID)
		if err != nil {
			return nil, nil, fmt.Errorf("looking up node: %w", err)
		}
		if node == nil {
			return nil, nil, ErrDynamicHostVolumeNodeNotFound
		}
		if !anyPool && node.NodePool != vol.NodePool {
			return nil, nil, ErrDynamicHostVolumeNodePoolMismatch
		}
		candidates = []*structs.Node{node}
	} else {
		iter, err := snap.Nodes(nil)
		if err != nil {
			return nil, nil, fmt.Errorf("listing nodes: %w", err)
		}
		for raw := iter.Next(); raw != nil; raw = iter.Next() {
			candidates = append(candidates, raw.(*structs.Node))
		}
	}

	// All namespaces and states: a name is unique per node across the table.
	conflicts := map[string]bool{}
	records := map[string]int{}
	iter, err := snap.DynamicHostVolumes(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("listing dynamic host volumes: %w", err)
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		rec := raw.(*structs.DynamicHostVolume)
		records[rec.NodeID]++
		if rec.Name == vol.Name {
			conflicts[rec.NodeID] = true
		}
	}

	// The context is per call; its metrics are never exposed because they
	// record constraint text.
	checker := scheduler.NewConstraintChecker(
		scheduler.NewEvalContext(nil, snap, nil, log.NewNullLogger()), vol.Constraints)
	attribute := dynamicHostVolumePluginAttributePrefix + vol.PluginID

	counts := map[string]int{}
	var best *structs.Node
	for _, node := range candidates {
		var reason string
		switch {
		case !explicit && !anyPool && node.NodePool != vol.NodePool:
			reason = DynamicHostVolumeReasonNodePool
		case node.Status != structs.NodeStatusReady:
			reason = DynamicHostVolumeReasonNodeNotReady
		case node.SchedulingEligibility != structs.NodeSchedulingEligible:
			reason = DynamicHostVolumeReasonNodeIneligible
		case node.DrainStrategy != nil:
			reason = DynamicHostVolumeReasonNodeDraining
		case !hasAttribute(node, attribute):
			reason = DynamicHostVolumeReasonPluginUnhealthy
		case !checker.Feasible(node):
			reason = DynamicHostVolumeReasonConstraint
		case conflicts[node.ID] || node.HostVolumes[vol.Name] != nil:
			reason = DynamicHostVolumeReasonNameConflict
		}
		if reason != "" {
			counts[reason]++
			continue
		}
		// Nodes arrive in ID order, so the first of equal counts already has
		// the lowest ID. The ID comparison keeps that true if the order of
		// iteration ever changes.
		if best == nil || records[node.ID] < records[best.ID] ||
			(records[node.ID] == records[best.ID] && node.ID < best.ID) {
			best = node
		}
	}
	if best != nil {
		return best, nil, nil
	}

	diag := &DynamicHostVolumeNodeDiagnostic{Reason: DynamicHostVolumeNoEligibleNode}
	for reason, n := range counts {
		diag.Filtered = append(diag.Filtered, DynamicHostVolumeNodeReasonCount{Reason: reason, Count: n})
	}
	slices.SortFunc(diag.Filtered, func(a, b DynamicHostVolumeNodeReasonCount) int {
		return strings.Compare(a.Reason, b.Reason)
	})
	return nil, diag, nil
}

func hasAttribute(node *structs.Node, key string) bool {
	_, ok := node.Attributes[key]
	return ok
}

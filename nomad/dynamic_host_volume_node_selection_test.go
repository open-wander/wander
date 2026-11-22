// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func selectionNode(id, pool string, mutate func(n *structs.Node)) *structs.Node {
	n := mock.Node()
	n.ID = id
	n.NodePool = pool
	n.Attributes = map[string]string{
		"kernel.name":                       "linux",
		"nomad.version":                     "1.6.5",
		"plugins.host_volume.version.mkdir": "0.0.1",
	}
	n.HostVolumes = nil
	if mutate != nil {
		mutate(n)
	}
	return n
}

func selectionVolume(mutate func(v *structs.DynamicHostVolume)) *structs.DynamicHostVolume {
	v := &structs.DynamicHostVolume{
		ID: "vol-id-new", Name: "vol-a", Namespace: "default", Type: structs.VolumeTypeHost,
		PluginID: "mkdir",
	}
	if mutate != nil {
		mutate(v)
	}
	return v
}

func TestSelectDynamicHostVolumeNode(t *testing.T) {
	ci.Parallel(t)

	otherNS := func(state structs.DynamicHostVolumeState) *structs.DynamicHostVolume {
		return &structs.DynamicHostVolume{ID: "vol-id-other", Name: "vol-a", Namespace: "other", Type: structs.VolumeTypeHost,
			NodeID: "00000000-0000-0000-0000-000000000001", PluginID: "mkdir", State: state}
	}

	tests := []struct {
		name      string
		nodes     []*structs.Node
		records   []*structs.DynamicHostVolume
		vol       *structs.DynamicHostVolume
		wantNode  string
		wantErr   error
		wantCode  string
		wantCount []DynamicHostVolumeNodeReasonCount
	}{
		{
			name:     "explicit node selected",
			nodes:    []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil), selectionNode("00000000-0000-0000-0000-000000000002", "default", nil)},
			vol:      selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000002" }),
			wantNode: "00000000-0000-0000-0000-000000000002",
		},
		{
			name:    "explicit unknown node",
			nodes:   []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			vol:     selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000009" }),
			wantErr: ErrDynamicHostVolumeNodeNotFound,
		},
		{
			name:  "explicit node pool mismatch",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			vol: selectionVolume(func(v *structs.DynamicHostVolume) {
				v.NodeID = "00000000-0000-0000-0000-000000000001"
				v.NodePool = "gpu"
			}),
			wantErr: ErrDynamicHostVolumeNodePoolMismatch,
		},
		{
			name:  "explicit node pool matches",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "gpu", nil)},
			vol: selectionVolume(func(v *structs.DynamicHostVolume) {
				v.NodeID = "00000000-0000-0000-0000-000000000001"
				v.NodePool = "gpu"
			}),
			wantNode: "00000000-0000-0000-0000-000000000001",
		},
		{
			name:  "explicit node with all pool is not a mismatch",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "gpu", nil)},
			vol: selectionVolume(func(v *structs.DynamicHostVolume) {
				v.NodeID = "00000000-0000-0000-0000-000000000001"
				v.NodePool = "all"
			}),
			wantNode: "00000000-0000-0000-0000-000000000001",
		},
		{
			name:      "explicit node failing a rule gives count of one",
			nodes:     []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) { n.Status = structs.NodeStatusDown })},
			vol:       selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000001" }),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodeNotReady, Count: 1}},
		},
		{
			name: "explicit node without the plugin gives count of one",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				delete(n.Attributes, "plugins.host_volume.version.mkdir")
			})},
			vol:       selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000001" }),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonPluginUnhealthy, Count: 1}},
		},
		{
			name: "explicit node with a static volume of the name gives count of one",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"vol-a": {Name: "vol-a", Path: "/srv/synthetic"}}
			})},
			vol:       selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000001" }),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNameConflict, Count: 1}},
		},
		{
			name:      "explicit node with a record of the name in another namespace gives count of one",
			nodes:     []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			records:   []*structs.DynamicHostVolume{otherNS(structs.DynamicHostVolumeStateReady)},
			vol:       selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000001" }),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNameConflict, Count: 1}},
		},
		{
			name: "explicit draining node gives count of one",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.DrainStrategy = &structs.DrainStrategy{}
				n.SchedulingEligibility = structs.NodeSchedulingEligible
			})},
			vol:       selectionVolume(func(v *structs.DynamicHostVolume) { v.NodeID = "00000000-0000-0000-0000-000000000001" }),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodeDraining, Count: 1}},
		},
		{
			name: "node both down and ineligible is counted only as not ready",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.Status = structs.NodeStatusDown
				n.SchedulingEligibility = structs.NodeSchedulingIneligible
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodeNotReady, Count: 1}},
		},
		{
			// Records of every namespace count, because a name is unique per
			// node across the table.
			name: "tie-break is decided by a record in another namespace",
			nodes: []*structs.Node{
				selectionNode("00000000-0000-0000-0000-000000000001", "default", nil),
				selectionNode("00000000-0000-0000-0000-000000000002", "default", nil),
			},
			records: []*structs.DynamicHostVolume{
				{ID: "r1", Name: "vol-y", Namespace: "other", Type: structs.VolumeTypeHost, NodeID: "00000000-0000-0000-0000-000000000001", PluginID: "mkdir"},
			},
			vol:      selectionVolume(nil),
			wantNode: "00000000-0000-0000-0000-000000000002",
		},
		{
			name: "implicit pool filters other pools",
			nodes: []*structs.Node{
				selectionNode("00000000-0000-0000-0000-000000000001", "gpu", nil),
				selectionNode("00000000-0000-0000-0000-000000000002", "default", nil),
			},
			vol:      selectionVolume(func(v *structs.DynamicHostVolume) { v.NodePool = "default" }),
			wantNode: "00000000-0000-0000-0000-000000000002",
		},
		{
			name: "no pool considers every pool",
			nodes: []*structs.Node{
				selectionNode("00000000-0000-0000-0000-000000000002", "default", nil),
				selectionNode("00000000-0000-0000-0000-000000000001", "gpu", nil),
			},
			vol:      selectionVolume(nil),
			wantNode: "00000000-0000-0000-0000-000000000001",
		},
		{
			name: "all pool considers every pool",
			nodes: []*structs.Node{
				selectionNode("00000000-0000-0000-0000-000000000002", "default", nil),
				selectionNode("00000000-0000-0000-0000-000000000001", "gpu", nil),
			},
			vol:      selectionVolume(func(v *structs.DynamicHostVolume) { v.NodePool = "all" }),
			wantNode: "00000000-0000-0000-0000-000000000001",
		},
		{
			name:      "pool without nodes",
			nodes:     []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "gpu", nil)},
			vol:       selectionVolume(func(v *structs.DynamicHostVolume) { v.NodePool = "empty" }),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodePool, Count: 1}},
		},
		{
			name:      "node not ready",
			nodes:     []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) { n.Status = structs.NodeStatusDown })},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodeNotReady, Count: 1}},
		},
		{
			name: "node ineligible",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.SchedulingEligibility = structs.NodeSchedulingIneligible
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodeIneligible, Count: 1}},
		},
		{
			name: "node draining",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.DrainStrategy = &structs.DrainStrategy{}
				n.SchedulingEligibility = structs.NodeSchedulingEligible
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNodeDraining, Count: 1}},
		},
		{
			name: "plugin not advertised",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				delete(n.Attributes, "plugins.host_volume.version.mkdir")
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonPluginUnhealthy, Count: 1}},
		},
		{
			name: "plugin attribute with segments reversed does not count",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				delete(n.Attributes, "plugins.host_volume.version.mkdir")
				n.Attributes["plugins.host_volume.mkdir.version"] = "0.0.1"
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonPluginUnhealthy, Count: 1}},
		},
		{
			name:  "constraint regexp fails",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			vol: selectionVolume(func(v *structs.DynamicHostVolume) {
				v.Constraints = []*structs.Constraint{{LTarget: "${attr.kernel.name}", RTarget: "secret-regexp-[0-9]+", Operand: "regexp"}}
			}),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonConstraint, Count: 1}},
		},
		{
			name:  "constraint version fails",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			vol: selectionVolume(func(v *structs.DynamicHostVolume) {
				v.Constraints = []*structs.Constraint{{LTarget: "${attr.nomad.version}", RTarget: ">= 99.0.0", Operand: "version"}}
			}),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonConstraint, Count: 1}},
		},
		{
			name:  "constraint satisfied",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			vol: selectionVolume(func(v *structs.DynamicHostVolume) {
				v.Constraints = []*structs.Constraint{{LTarget: "${attr.kernel.name}", RTarget: "linux", Operand: "="}}
			}),
			wantNode: "00000000-0000-0000-0000-000000000001",
		},
		{
			name: "static volume name conflict",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"vol-a": {Name: "vol-a", Path: "/srv/synthetic"}}
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNameConflict, Count: 1}},
		},
		{
			name: "ready dynamic volume name conflict",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) {
				n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{"vol-a": {Name: "vol-a", Path: "/srv/synthetic", ID: "vol-id-x"}}
			})},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNameConflict, Count: 1}},
		},
		{
			name:      "pending record in another namespace conflicts",
			nodes:     []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			records:   []*structs.DynamicHostVolume{otherNS(structs.DynamicHostVolumeStatePending)},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNameConflict, Count: 1}},
		},
		{
			name:      "deleting record in another namespace conflicts",
			nodes:     []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			records:   []*structs.DynamicHostVolume{otherNS(structs.DynamicHostVolumeStateDeleting)},
			vol:       selectionVolume(nil),
			wantCode:  DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{{Reason: DynamicHostVolumeReasonNameConflict, Count: 1}},
		},
		{
			name:  "record with a different name does not conflict",
			nodes: []*structs.Node{selectionNode("00000000-0000-0000-0000-000000000001", "default", nil)},
			records: []*structs.DynamicHostVolume{{ID: "vol-id-o", Name: "vol-b", Namespace: "other", Type: structs.VolumeTypeHost,
				NodeID: "00000000-0000-0000-0000-000000000001", PluginID: "mkdir"}},
			vol:      selectionVolume(nil),
			wantNode: "00000000-0000-0000-0000-000000000001",
		},
		{
			name: "tie-break prefers fewest records then lowest id",
			nodes: []*structs.Node{
				selectionNode("00000000-0000-0000-0000-000000000001", "default", nil),
				selectionNode("00000000-0000-0000-0000-000000000002", "default", nil),
				selectionNode("00000000-0000-0000-0000-000000000003", "default", nil),
			},
			records: []*structs.DynamicHostVolume{
				{ID: "r1", Name: "vol-x", Namespace: "default", Type: structs.VolumeTypeHost, NodeID: "00000000-0000-0000-0000-000000000001", PluginID: "mkdir"},
				{ID: "r2", Name: "vol-y", Namespace: "other", Type: structs.VolumeTypeHost, NodeID: "00000000-0000-0000-0000-000000000001", PluginID: "mkdir"},
				{ID: "r3", Name: "vol-z", Namespace: "default", Type: structs.VolumeTypeHost, NodeID: "00000000-0000-0000-0000-000000000002", PluginID: "mkdir"},
			},
			vol:      selectionVolume(nil),
			wantNode: "00000000-0000-0000-0000-000000000003",
		},
		{
			name: "counts are ordered by reason code and count each node once",
			nodes: []*structs.Node{
				selectionNode("00000000-0000-0000-0000-000000000001", "default", func(n *structs.Node) { n.Status = structs.NodeStatusDown }),
				selectionNode("00000000-0000-0000-0000-000000000002", "default", func(n *structs.Node) {
					n.Status = structs.NodeStatusDown
					delete(n.Attributes, "plugins.host_volume.version.mkdir")
				}),
				selectionNode("00000000-0000-0000-0000-000000000003", "default", func(n *structs.Node) { delete(n.Attributes, "plugins.host_volume.version.mkdir") }),
				selectionNode("00000000-0000-0000-0000-000000000004", "other", nil),
			},
			vol:      selectionVolume(func(v *structs.DynamicHostVolume) { v.NodePool = "default" }),
			wantCode: DynamicHostVolumeNoEligibleNode,
			wantCount: []DynamicHostVolumeNodeReasonCount{
				{Reason: DynamicHostVolumeReasonNodeNotReady, Count: 2},
				{Reason: DynamicHostVolumeReasonNodePool, Count: 1},
				{Reason: DynamicHostVolumeReasonPluginUnhealthy, Count: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ci.Parallel(t)
			store := state.TestStateStore(t)
			for i, n := range tc.nodes {
				must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, uint64(100+i), n))
			}
			if len(tc.records) > 0 {
				must.NoError(t, store.UpsertNamespaces(150, []*structs.Namespace{{Name: "other"}}))
				must.NoError(t, store.UpsertDynamicHostVolumes(200, tc.records))
			}
			snap, err := store.Snapshot()
			must.NoError(t, err)

			// Repeat to assert determinism.
			for range 3 {
				node, diag, err := selectDynamicHostVolumeNode(snap, tc.vol)
				switch {
				case tc.wantErr != nil:
					must.ErrorIs(t, err, tc.wantErr)
					must.Nil(t, node)
					must.Nil(t, diag)
				case tc.wantNode != "":
					must.NoError(t, err)
					must.Nil(t, diag)
					must.NotNil(t, node)
					must.Eq(t, tc.wantNode, node.ID)
				default:
					must.NoError(t, err)
					must.Nil(t, node)
					must.NotNil(t, diag)
					must.Eq(t, tc.wantCode, diag.Reason)
					must.Eq(t, tc.wantCount, diag.Filtered)
					rendered := fmt.Sprintf("%#v", diag)
					for _, secret := range []string{"secret-regexp", "99.0.0", "kernel.name", "00000000-", "vol-a"} {
						must.False(t, strings.Contains(rendered, secret))
					}
				}
			}
		})
	}
}

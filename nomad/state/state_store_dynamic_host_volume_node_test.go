// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package state

import (
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

const (
	dhvNodeSeedIndex = uint64(10)
	dhvNodeA         = "aaaaaaaa-0000-0000-0000-000000000001"
	dhvNodeB         = "bbbbbbbb-0000-0000-0000-000000000002"
)

func dhvNodeVolume(id, node string, state structs.DynamicHostVolumeState) *structs.DynamicHostVolume {
	v := dynamicHostVolumeStateFixture(id)
	v.NodeID, v.State = node, state
	v.Constraints, v.RequestedCapabilities, v.Attachments = nil, nil, nil
	return v
}

// dhvNodeFixture seeds a node that advertises the listed volume IDs and the
// volumes themselves, all at dhvNodeSeedIndex.
func dhvNodeFixture(t *testing.T, status string, advertised []string, volumes ...*structs.DynamicHostVolume) (*StateStore, *structs.Node) {
	t.Helper()
	store := TestStateStore(t)
	node := mock.Node()
	node.ID = dhvNodeA
	node.Status = status
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{}
	for _, id := range advertised {
		node.HostVolumes["data-"+id] = &structs.ClientHostVolumeConfig{Name: "data-" + id, Path: "/srv/wander-synthetic/" + id, ID: id}
	}
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, 5, node))
	for _, v := range volumes {
		dhvNodeEnsureNamespace(t, store, v.Namespace)
	}
	if len(volumes) > 0 {
		must.NoError(t, store.UpsertDynamicHostVolumes(dhvNodeSeedIndex, volumes))
	}
	return store, node
}

func dhvNodeEnsureNamespace(t *testing.T, store *StateStore, name string) {
	t.Helper()
	if name == structs.DefaultNamespace {
		return
	}
	must.NoError(t, store.UpsertNamespaces(4, []*structs.Namespace{{Name: name}}))
}

func dhvNodeGet(t *testing.T, store *StateStore, id string) *structs.DynamicHostVolume {
	t.Helper()
	v, err := store.DynamicHostVolumeByID(nil, "default", id)
	must.NoError(t, err)
	must.NotNil(t, v)
	return v
}

func dhvNodeTableIndex(t *testing.T, store *StateStore) uint64 {
	t.Helper()
	idx, err := store.Index(TableDynamicHostVolumes)
	must.NoError(t, err)
	return idx
}

// reupsert registers a fresh copy of the node with the given status and
// advertised volume IDs.
func dhvReupsert(t *testing.T, store *StateStore, base *structs.Node, index uint64, status string, advertised ...string) {
	t.Helper()
	node := base.Copy()
	node.Status = status
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{}
	for _, id := range advertised {
		node.HostVolumes["data-"+id] = &structs.ClientHostVolumeConfig{Name: "data-" + id, Path: "/srv/wander-synthetic/" + id, ID: id}
	}
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, index, node))
}

func TestStateStore_DynamicHostVolumeNode_Upsert(t *testing.T) {
	const (
		ready       = structs.DynamicHostVolumeStateReady
		unavailable = structs.DynamicHostVolumeStateUnavailable
	)
	cases := []struct {
		name       string
		start      structs.DynamicHostVolumeState
		advertised []string
		status     string
		want       structs.DynamicHostVolumeState
	}{
		{"ready, no longer advertised", ready, nil, structs.NodeStatusReady, unavailable},
		{"ready, still advertised", ready, []string{"volume-a"}, structs.NodeStatusReady, ready},
		{"unavailable, advertised again", unavailable, []string{"volume-a"}, structs.NodeStatusReady, ready},
		{"unavailable, not advertised", unavailable, nil, structs.NodeStatusReady, unavailable},
		{"unavailable, advertised on initializing node", unavailable, []string{"volume-a"}, structs.NodeStatusInit, ready},
		{"unavailable, advertised but node down", unavailable, []string{"volume-a"}, structs.NodeStatusDown, unavailable},
		{"unavailable, advertised but node disconnected", unavailable, []string{"volume-a"}, structs.NodeStatusDisconnected, unavailable},
		{"ready, advertised but node down", ready, []string{"volume-a"}, structs.NodeStatusDown, unavailable},
		{"ready, advertised but node disconnected", ready, []string{"volume-a"}, structs.NodeStatusDisconnected, unavailable},
		{"ready, not advertised on initializing node", ready, nil, structs.NodeStatusInit, unavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, node := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
				dhvNodeVolume("volume-a", dhvNodeA, tc.start))
			before := dhvNodeGet(t, store, "volume-a")
			dhvReupsert(t, store, node, 20, tc.status, tc.advertised...)
			got := dhvNodeGet(t, store, "volume-a")
			must.Eq(t, tc.want, got.State)
			if tc.want == tc.start {
				must.Eq(t, before, got)
				must.Eq(t, dhvNodeSeedIndex, dhvNodeTableIndex(t, store))
				return
			}
			must.Eq(t, uint64(20), got.ModifyIndex)
			must.Eq(t, uint64(20), dhvNodeTableIndex(t, store))
			must.Eq(t, before.ModifyTime, got.ModifyTime)
			must.Eq(t, before.StatusDescription, got.StatusDescription)
			must.Eq(t, before.CreateIndex, got.CreateIndex)
		})
	}
}

func TestStateStore_DynamicHostVolumeNode_StatusUpdate(t *testing.T) {
	store, _ := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))

	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 20, dhvNodeA, structs.NodeStatusDown, time.Now().Unix(), nil))
	got := dhvNodeGet(t, store, "volume-a")
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, got.State)
	must.Eq(t, uint64(20), got.ModifyIndex)
	must.Eq(t, uint64(20), dhvNodeTableIndex(t, store))

	// Down to down changes nothing.
	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 21, dhvNodeA, structs.NodeStatusDown, time.Now().Unix(), nil))
	must.Eq(t, uint64(20), dhvNodeGet(t, store, "volume-a").ModifyIndex)
	must.Eq(t, uint64(20), dhvNodeTableIndex(t, store))

	// The stored node still lists the volume, so ready brings it back.
	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 22, dhvNodeA, structs.NodeStatusReady, time.Now().Unix(), nil))
	got = dhvNodeGet(t, store, "volume-a")
	must.Eq(t, structs.DynamicHostVolumeStateReady, got.State)
	must.Eq(t, uint64(22), got.ModifyIndex)
	must.Eq(t, uint64(22), dhvNodeTableIndex(t, store))

	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 23, dhvNodeA, structs.NodeStatusDisconnected, time.Now().Unix(), nil))
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, dhvNodeGet(t, store, "volume-a").State)
}

func TestStateStore_DynamicHostVolumeNode_Delete(t *testing.T) {
	store, _ := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))
	must.NoError(t, store.DeleteNode(structs.NodeDeregisterRequestType, 20, []string{dhvNodeA}))
	got := dhvNodeGet(t, store, "volume-a")
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, got.State)
	must.Eq(t, uint64(20), got.ModifyIndex)
	must.Eq(t, uint64(20), dhvNodeTableIndex(t, store))
}

func TestStateStore_DynamicHostVolumeNode_OtherStatesAndNodesUntouched(t *testing.T) {
	var volumes []*structs.DynamicHostVolume
	for _, state := range []structs.DynamicHostVolumeState{
		structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending,
		structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeStateFailed,
	} {
		volumes = append(volumes, dhvNodeVolume("volume-"+string(state), dhvNodeA, state))
	}
	// The requested record in the loop above is synthetic: it carries the
	// node's ID, which the transition rules never produce for a requested
	// volume. It is written straight through the test helper so that the
	// state filter in the derivation is the only thing leaving it unchanged.
	// The record below is a real requested volume with no node.
	volumes = append(volumes, dhvNodeVolume("volume-requested-empty", "", structs.DynamicHostVolumeStateRequested))
	volumes = append(volumes,
		dhvNodeVolume("volume-other", dhvNodeB, structs.DynamicHostVolumeStateReady),
		dhvNodeVolume("volume-other-unavailable", dhvNodeB, structs.DynamicHostVolumeStateUnavailable))
	store, node := dhvNodeFixture(t, structs.NodeStatusReady, nil, volumes...)

	snapshot := map[string]*structs.DynamicHostVolume{}
	for _, v := range volumes {
		snapshot[v.ID] = dhvNodeGet(t, store, v.ID)
	}

	dhvReupsert(t, store, node, 20, structs.NodeStatusReady)
	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 21, dhvNodeA, structs.NodeStatusDown, 1, nil))
	must.NoError(t, store.DeleteNode(structs.NodeDeregisterRequestType, 22, []string{dhvNodeA}))

	for id, want := range snapshot {
		must.Eq(t, want, dhvNodeGet(t, store, id))
	}
	must.Eq(t, dhvNodeSeedIndex, dhvNodeTableIndex(t, store))
}

func TestStateStore_DynamicHostVolumeNode_MatchesByID(t *testing.T) {
	store, node := dhvNodeFixture(t, structs.NodeStatusReady, nil,
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))

	// A static volume with the dynamic volume's name and an empty ID does not
	// keep it ready.
	static := node.Copy()
	static.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"data-volume-a": {Name: "data-volume-a", Path: "/srv/wander-synthetic/static"},
	}
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, 20, static))
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, dhvNodeGet(t, store, "volume-a").State)

	// An entry under another name but with the volume ID keeps it ready.
	renamed := node.Copy()
	renamed.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"other-name": {Name: "other-name", Path: "/srv/wander-synthetic/a", ID: "volume-a"},
	}
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, 21, renamed))
	must.Eq(t, structs.DynamicHostVolumeStateReady, dhvNodeGet(t, store, "volume-a").State)
}

func TestStateStore_DynamicHostVolumeNode_NoChangeLeavesIndexes(t *testing.T) {
	store, node := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))
	before := dhvNodeGet(t, store, "volume-a")

	dhvReupsert(t, store, node, 20, structs.NodeStatusReady, "volume-a")
	must.NoError(t, store.UpdateNodeEligibility(structs.NodeUpdateEligibilityRequestType, 21, dhvNodeA, structs.NodeSchedulingIneligible, 1, nil))
	must.NoError(t, store.UpdateNodeDrain(structs.NodeUpdateDrainRequestType, 22, dhvNodeA,
		&structs.DrainStrategy{DrainSpec: structs.DrainSpec{Deadline: time.Hour}}, false, 1, nil, nil, ""))

	must.Eq(t, before, dhvNodeGet(t, store, "volume-a"))
	must.Eq(t, dhvNodeSeedIndex, dhvNodeTableIndex(t, store))
}

func TestStateStore_DynamicHostVolumeNode_RestartFlap(t *testing.T) {
	store, node := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))

	// The restarted client registers as initializing before it restores the
	// volume, then lists it.
	dhvReupsert(t, store, node, 20, structs.NodeStatusInit)
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, dhvNodeGet(t, store, "volume-a").State)
	dhvReupsert(t, store, node, 21, structs.NodeStatusInit, "volume-a")
	got := dhvNodeGet(t, store, "volume-a")
	must.Eq(t, structs.DynamicHostVolumeStateReady, got.State)
	must.Eq(t, uint64(21), got.ModifyIndex)
}

func TestStateStore_DynamicHostVolumeNode_WatchFires(t *testing.T) {
	store, _ := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))
	ws := memdb.NewWatchSet()
	_, err := store.DynamicHostVolumeByID(ws, "default", "volume-a")
	must.NoError(t, err)
	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 20, dhvNodeA, structs.NodeStatusDown, 1, nil))
	must.True(t, watchFired(ws))
}

func TestStateStore_DynamicHostVolumeNode_OtherNodeOnlyChangesItsOwn(t *testing.T) {
	store, _ := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a"},
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady),
		dhvNodeVolume("volume-b", dhvNodeB, structs.DynamicHostVolumeStateReady))
	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 20, dhvNodeA, structs.NodeStatusDown, 1, nil))
	must.Eq(t, structs.DynamicHostVolumeStateUnavailable, dhvNodeGet(t, store, "volume-a").State)
	must.Eq(t, structs.DynamicHostVolumeStateReady, dhvNodeGet(t, store, "volume-b").State)
	must.Eq(t, dhvNodeSeedIndex, dhvNodeGet(t, store, "volume-b").ModifyIndex)
}

// A node with no volumes changes nothing, even when other nodes hold volumes
// in several namespaces.
func TestStateStore_DynamicHostVolumeNode_NodeWithoutVolumesChangesNothing(t *testing.T) {
	store := TestStateStore(t)
	node := mock.Node()
	node.ID = dhvNodeA
	node.HostVolumes = nil
	must.NoError(t, store.UpsertNode(structs.NodeRegisterRequestType, 5, node))

	var volumes []*structs.DynamicHostVolume
	for _, namespace := range []string{"default", "team-one", "team-two"} {
		for _, state := range []structs.DynamicHostVolumeState{structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeStateUnavailable} {
			v := dhvNodeVolume("volume-"+namespace+"-"+string(state), dhvNodeB, state)
			v.Namespace = namespace
			volumes = append(volumes, v)
		}
		dhvNodeEnsureNamespace(t, store, namespace)
	}
	must.NoError(t, store.UpsertDynamicHostVolumes(dhvNodeSeedIndex, volumes))

	read := func() []*structs.DynamicHostVolume {
		var out []*structs.DynamicHostVolume
		for _, v := range volumes {
			got, err := store.DynamicHostVolumeByID(nil, v.Namespace, v.ID)
			must.NoError(t, err)
			must.NotNil(t, got)
			out = append(out, got)
		}
		return out
	}
	before := read()

	dhvReupsert(t, store, node, 20, structs.NodeStatusReady)
	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 21, dhvNodeA, structs.NodeStatusDown, 1, nil))
	must.NoError(t, store.DeleteNode(structs.NodeDeregisterRequestType, 22, []string{dhvNodeA}))

	must.Eq(t, before, read())
	must.Eq(t, dhvNodeSeedIndex, dhvNodeTableIndex(t, store))
}

// Volumes in different namespaces on one node are all derived.
func TestStateStore_DynamicHostVolumeNode_TwoNamespacesSameNode(t *testing.T) {
	first := dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady)
	second := dhvNodeVolume("volume-b", dhvNodeA, structs.DynamicHostVolumeStateReady)
	second.Namespace = "team-one"
	store, _ := dhvNodeFixture(t, structs.NodeStatusReady, []string{"volume-a", "volume-b"}, first, second)

	must.NoError(t, store.UpdateNodeStatus(structs.NodeUpdateStatusRequestType, 20, dhvNodeA, structs.NodeStatusDown, 1, nil))
	for _, v := range []*structs.DynamicHostVolume{first, second} {
		got, err := store.DynamicHostVolumeByID(nil, v.Namespace, v.ID)
		must.NoError(t, err)
		must.Eq(t, structs.DynamicHostVolumeStateUnavailable, got.State)
		must.Eq(t, uint64(20), got.ModifyIndex)
	}
	must.Eq(t, uint64(20), dhvNodeTableIndex(t, store))
}

// An empty node ID is never matched, and a requested volume without a node is
// not found by any node.
func TestStateStore_DynamicHostVolumeNode_EmptyNodeIDMatchesNothing(t *testing.T) {
	store, _ := dhvNodeFixture(t, structs.NodeStatusReady, nil,
		dhvNodeVolume("volume-requested", "", structs.DynamicHostVolumeStateRequested),
		dhvNodeVolume("volume-a", dhvNodeA, structs.DynamicHostVolumeStateReady))
	tx := store.db.ReadTxn()
	defer tx.Abort()
	got, err := dynamicHostVolumesOnNodeTxn(tx, "")
	must.NoError(t, err)
	must.SliceEmpty(t, got)
	got, err = dynamicHostVolumesOnNodeTxn(tx, dhvNodeA)
	must.NoError(t, err)
	must.Len(t, 1, got)
}

func TestStateStore_DynamicHostVolumesByNode_AllNamespaces(t *testing.T) {
	store := testStateStore(t)
	must.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "ns-a"}, {Name: "ns-b"}}))
	mk := func(ns, id, node string) *structs.DynamicHostVolume {
		return &structs.DynamicHostVolume{
			ID: id, Name: "name-" + id, Namespace: ns, Type: structs.VolumeTypeHost,
			NodeID: node, NodePool: "default", HostPath: "/synthetic/" + id, PluginID: "plug",
			State: structs.DynamicHostVolumeStateReady,
		}
	}
	must.NoError(t, store.UpsertDynamicHostVolumes(2, []*structs.DynamicHostVolume{
		mk("ns-a", "vol-1", "n1"), mk("ns-b", "vol-2", "n1"), mk("ns-a", "vol-3", "n2"),
	}))

	iter, err := store.DynamicHostVolumesByNode(nil, "n1")
	must.NoError(t, err)
	var ids []string
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		ids = append(ids, raw.(*structs.DynamicHostVolume).ID)
	}
	sort.Strings(ids)
	must.Eq(t, []string{"vol-1", "vol-2"}, ids)
}

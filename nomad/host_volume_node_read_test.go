// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"sort"
	"testing"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// nodeReadFixture holds one node that advertises one static volume and four
// dynamic ones: one per namespace, one whose record lives on another node, and
// one with no record at all.
type nodeReadFixture struct {
	node *structs.Node
}

const (
	nrStatic = "static-data"
	nrDynA   = "dyn-a"
	nrDynB   = "dyn-b"
	nrDynP   = "dyn-prod"
	nrMoved  = "dyn-moved"
	nrGhost  = "dyn-ghost"
)

func newNodeReadFixture(t *testing.T, s *Server) *nodeReadFixture {
	t.Helper()
	upsertHostVolumeNamespaces(t, s, 90, "ns-a", "ns-b", "prod-1")

	node := mock.Node()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		nrStatic: {Name: nrStatic, Path: "/synthetic/static"},
		nrDynA:   {Name: nrDynA, Path: "/synthetic/vol-a", ID: "vol-a"},
		nrDynB:   {Name: nrDynB, Path: "/synthetic/vol-b", ID: "vol-b"},
		nrDynP:   {Name: nrDynP, Path: "/synthetic/vol-p", ID: "vol-p"},
		nrMoved:  {Name: nrMoved, Path: "/synthetic/vol-m", ID: "vol-m"},
		nrGhost:  {Name: nrGhost, Path: "/synthetic/vol-g", ID: "vol-g"},
	}
	must.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))
	must.NoError(t, s.State().UpsertDynamicHostVolumes(110, []*structs.DynamicHostVolume{
		testHostVolume("ns-a", "vol-a", node.ID, "default"),
		testHostVolume("ns-b", "vol-b", node.ID, "default"),
		testHostVolume("prod-1", "vol-p", node.ID, "default"),
		testHostVolume("ns-a", "vol-m", "other-node", "default"),
	}))
	return &nodeReadFixture{node: node}
}

func nodeReadNames(vols map[string]*structs.ClientHostVolumeConfig) []string {
	names := make([]string, 0, len(vols))
	for name := range vols {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedNames(names ...string) []string {
	sort.Strings(names)
	return names
}

// checkNodeUnchanged proves a filtered read did not modify the stored node.
func checkNodeUnchanged(t *testing.T, s *Server, f *nodeReadFixture) {
	t.Helper()
	stored, err := s.State().NodeByID(nil, f.node.ID)
	must.NoError(t, err)
	must.Eq(t, nodeReadNames(f.node.HostVolumes), nodeReadNames(stored.HostVolumes))
}

type nodeReadCase struct {
	name   string
	policy string
	root   bool
	want   []string
}

func nodeReadCases() []nodeReadCase {
	nodeRead := mock.NodePolicy(acl.PolicyRead) + "\n" + mock.NodePoolPolicy("*", acl.PolicyRead, nil) + "\n"
	hv := []string{acl.NamespaceCapabilityHostVolumeRead}
	return []nodeReadCase{
		{
			name:   "node read only",
			policy: nodeRead,
			want:   []string{nrStatic},
		},
		{
			name:   "namespace a",
			policy: nodeRead + mock.NamespacePolicy("ns-a", "", hv),
			want:   sortedNames(nrStatic, nrDynA),
		},
		{
			name:   "wildcard namespace",
			policy: nodeRead + mock.NamespacePolicy("*", "", hv),
			want:   sortedNames(nrStatic, nrDynA, nrDynB, nrDynP),
		},
		{
			name:   "glob namespace",
			policy: nodeRead + mock.NamespacePolicy("prod-*", "", hv),
			want:   sortedNames(nrStatic, nrDynP),
		},
		{
			name: "management",
			root: true,
			want: sortedNames(nrStatic, nrDynA, nrDynB, nrDynP, nrMoved, nrGhost),
		},
	}
}

func TestHostVolumeNodeRead_RPC(t *testing.T) {
	ci.Parallel(t)
	s, root, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	f := newNodeReadFixture(t, s)

	for i, tc := range nodeReadCases() {
		t.Run(tc.name, func(t *testing.T) {
			secret := root.SecretID
			if !tc.root {
				secret = mock.CreatePolicyAndToken(t, s.State(), uint64(200+i*2), "p-"+tc.name, tc.policy).SecretID
			}
			qo := structs.QueryOptions{Region: "global", AuthToken: secret}

			var get structs.SingleNodeResponse
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.GetNode",
				&structs.NodeSpecificRequest{NodeID: f.node.ID, QueryOptions: qo}, &get))
			must.NotNil(t, get.Node)
			must.Eq(t, tc.want, nodeReadNames(get.Node.HostVolumes))
			must.Eq(t, uint64(100), get.Index)
			checkNodeUnchanged(t, s, f)

			for _, fields := range []*structs.NodeStubFields{nil, {Resources: true, OS: true}} {
				var list structs.NodeListResponse
				must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.List",
					&structs.NodeListRequest{QueryOptions: qo, Fields: fields}, &list))
				must.Len(t, 1, list.Nodes)
				must.Eq(t, tc.want, nodeReadNames(list.Nodes[0].HostVolumes))
				checkNodeUnchanged(t, s, f)
			}

			var pool structs.NodePoolNodesResponse
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "NodePool.ListNodes",
				&structs.NodePoolNodesRequest{Name: f.node.NodePool, QueryOptions: qo}, &pool))
			must.Len(t, 1, pool.Nodes)
			must.Eq(t, tc.want, nodeReadNames(pool.Nodes[0].HostVolumes))
			checkNodeUnchanged(t, s, f)
		})
	}
}

func TestHostVolumeNodeRead_EmptyMapStaysNonNil(t *testing.T) {
	ci.Parallel(t)
	s, _, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	node := mock.Node()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		nrDynA: {Name: nrDynA, Path: "/synthetic/vol-a", ID: "vol-a"},
	}
	must.NoError(t, s.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))
	tok := mock.CreatePolicyAndToken(t, s.State(), 200, "node-read", mock.NodePolicy(acl.PolicyRead))

	var get structs.SingleNodeResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.GetNode", &structs.NodeSpecificRequest{
		NodeID: node.ID, QueryOptions: structs.QueryOptions{Region: "global", AuthToken: tok.SecretID}}, &get))
	must.NotNil(t, get.Node)
	must.NotNil(t, get.Node.HostVolumes)
	must.MapLen(t, 0, get.Node.HostVolumes)
}

func TestHostVolumeNodeRead_ACLDisabled(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	f := newNodeReadFixture(t, s)
	all := sortedNames(nrStatic, nrDynA, nrDynB, nrDynP, nrMoved, nrGhost)

	var get structs.SingleNodeResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.GetNode",
		&structs.NodeSpecificRequest{NodeID: f.node.ID, QueryOptions: structs.QueryOptions{Region: "global"}}, &get))
	must.Eq(t, all, nodeReadNames(get.Node.HostVolumes))

	var list structs.NodeListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.List",
		&structs.NodeListRequest{QueryOptions: structs.QueryOptions{Region: "global"}}, &list))
	must.Eq(t, all, nodeReadNames(list.Nodes[0].HostVolumes))
}

// A client reading its own node through its secret ID is not filtered.
func TestHostVolumeNodeRead_ClientIdentityUnfiltered(t *testing.T) {
	ci.Parallel(t)
	s, _, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	f := newNodeReadFixture(t, s)

	var get structs.SingleNodeResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.GetNode", &structs.NodeSpecificRequest{
		NodeID: f.node.ID, QueryOptions: structs.QueryOptions{Region: "global", AuthToken: f.node.SecretID}}, &get))
	must.Eq(t, sortedNames(nrStatic, nrDynA, nrDynB, nrDynP, nrMoved, nrGhost), nodeReadNames(get.Node.HostVolumes))
}

func TestNodeHostVolumesForRead(t *testing.T) {
	ci.Parallel(t)
	store := state.TestStateStore(t)
	must.NoError(t, store.UpsertNamespaces(1, []*structs.Namespace{{Name: "ns-a"}, {Name: "ns-b"}}))

	vols := map[string]*structs.ClientHostVolumeConfig{
		"s": {Name: "s", Path: "/synthetic/s"},
		"a": {Name: "a", Path: "/synthetic/a", ID: "vol-a"},
		"b": {Name: "b", Path: "/synthetic/b", ID: "vol-b"},
		"g": {Name: "g", Path: "/synthetic/g", ID: "vol-g"},
	}
	// vol-a exists in two namespaces on the node: shown only when both are readable.
	must.NoError(t, store.UpsertDynamicHostVolumes(2, []*structs.DynamicHostVolume{
		testHostVolume("ns-a", "vol-a", "n1", "default"),
		testHostVolume("ns-b", "vol-a", "n1", "default"),
		testHostVolume("ns-b", "vol-b", "n1", "default"),
	}))

	mkACL := func(t *testing.T, policy string) *acl.ACL {
		p, err := acl.Parse(policy)
		must.NoError(t, err)
		a, err := acl.NewACL(false, []*acl.Policy{p})
		must.NoError(t, err)
		return a
	}
	hv := []string{acl.NamespaceCapabilityHostVolumeRead}

	cases := []struct {
		name    string
		acl     *acl.ACL
		changed bool
		want    []string
	}{
		{name: "nil acl", acl: nil, want: []string{"a", "b", "g", "s"}},
		{name: "management", acl: acl.ManagementACL, want: []string{"a", "b", "g", "s"}},
		{name: "no capability", acl: mkACL(t, mock.NodePolicy(acl.PolicyRead)), changed: true, want: []string{"s"}},
		{name: "ns-b only", acl: mkACL(t, mock.NamespacePolicy("ns-b", "", hv)), changed: true, want: []string{"b", "s"}},
		{name: "both namespaces", acl: mkACL(t, mock.NamespacePolicy("ns-*", "", hv)), changed: true, want: []string{"a", "b", "s"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := nodeHostVolumesForRead(tc.acl, store, "n1", vols)
			must.NoError(t, err)
			must.Eq(t, tc.changed, changed)
			must.Eq(t, tc.want, nodeReadNames(got))
			must.MapLen(t, 4, vols)
		})
	}

	t.Run("static only needs no lookup", func(t *testing.T) {
		// A closed-over nil store would panic on any lookup.
		staticOnly := map[string]*structs.ClientHostVolumeConfig{"s": {Name: "s"}}
		got, changed, err := nodeHostVolumesForRead(mkACL(t, mock.NodePolicy(acl.PolicyRead)), nil, "n1", staticOnly)
		must.NoError(t, err)
		must.False(t, changed)
		must.Eq(t, []string{"s"}, nodeReadNames(got))
	})
	t.Run("nil map stays nil", func(t *testing.T) {
		got, changed, err := nodeHostVolumesForRead(mkACL(t, mock.NodePolicy(acl.PolicyRead)), nil, "n1", nil)
		must.NoError(t, err)
		must.False(t, changed)
		must.Nil(t, got)
	})
}

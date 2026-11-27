// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// seedHTTPNodeHostVolumes stores one node that advertises a static volume and
// two dynamic volumes, one in each of two namespaces.
func seedHTTPNodeHostVolumes(t *testing.T, s *TestAgent) *structs.Node {
	t.Helper()
	store := s.Agent.Server().State()
	must.NoError(t, store.UpsertNamespaces(90, []*structs.Namespace{{Name: "ns-a"}, {Name: "ns-b"}}))
	node := mock.Node()
	node.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
		"static-data": {Name: "static-data", Path: "/synthetic/static"},
		"dyn-a":       {Name: "dyn-a", Path: "/synthetic/vol-a", ID: "vol-a"},
		"dyn-b":       {Name: "dyn-b", Path: "/synthetic/vol-b", ID: "vol-b"},
	}
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 100, node))
	vol := func(ns, id string) *structs.DynamicHostVolume {
		return &structs.DynamicHostVolume{
			ID: id, Name: "name-" + id, Namespace: ns, Type: structs.VolumeTypeHost,
			NodeID: node.ID, NodePool: "default", HostPath: "/synthetic/" + id, PluginID: "plug",
			State: structs.DynamicHostVolumeStateReady,
		}
	}
	must.NoError(t, store.UpsertDynamicHostVolumes(110, []*structs.DynamicHostVolume{vol("ns-a", "vol-a"), vol("ns-b", "vol-b")}))
	return node
}

func httpHostVolumeNames(vols map[string]*structs.ClientHostVolumeConfig) []string {
	names := make([]string, 0, len(vols))
	for name := range vols {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestHTTP_NodeHostVolumes_ACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		node := seedHTTPNodeHostVolumes(t, s)
		store := s.Agent.Server().State()
		nodeRead := mock.NodePolicy(acl.PolicyRead) + "\n" + mock.NodePoolPolicy("*", acl.PolicyRead, nil) + "\n"
		plain := mock.CreatePolicyAndToken(t, store, 200, "node-only", nodeRead)
		scoped := mock.CreatePolicyAndToken(t, store, 202, "ns-a", nodeRead+
			mock.NamespacePolicy("ns-a", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
		root := s.RootToken

		cases := []struct {
			name  string
			token *structs.ACLToken
			want  []string
		}{
			{"node read only", plain, []string{"static-data"}},
			{"namespace a", scoped, []string{"dyn-a", "static-data"}},
			{"management", root, []string{"dyn-a", "dyn-b", "static-data"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req, err := http.NewRequest(http.MethodGet, "/v1/node/"+node.ID, nil)
				must.NoError(t, err)
				setToken(req, tc.token)
				obj, err := s.Server.NodeSpecificRequest(httptest.NewRecorder(), req)
				must.NoError(t, err)
				must.Eq(t, tc.want, httpHostVolumeNames(obj.(*structs.Node).HostVolumes))

				req, err = http.NewRequest(http.MethodGet, "/v1/nodes?resources=true", nil)
				must.NoError(t, err)
				setToken(req, tc.token)
				obj, err = s.Server.NodesRequest(httptest.NewRecorder(), req)
				must.NoError(t, err)
				stubs := obj.([]*structs.NodeListStub)
				must.Eq(t, tc.want, httpHostVolumeNames(stubFor(t, stubs, node.ID).HostVolumes))

				req, err = http.NewRequest(http.MethodGet, "/v1/node/pool/default/nodes", nil)
				must.NoError(t, err)
				setToken(req, tc.token)
				obj, err = s.Server.NodePoolSpecificRequest(httptest.NewRecorder(), req)
				must.NoError(t, err)
				stubs = obj.([]*structs.NodeListStub)
				must.Eq(t, tc.want, httpHostVolumeNames(stubFor(t, stubs, node.ID).HostVolumes))
			})
		}

		// The stored node still advertises everything.
		stored, err := store.NodeByID(nil, node.ID)
		must.NoError(t, err)
		must.MapLen(t, 3, stored.HostVolumes)
	})
}

// stubFor picks the seeded node out of a list that also holds the test agent's
// own client node.
func stubFor(t *testing.T, stubs []*structs.NodeListStub, id string) *structs.NodeListStub {
	t.Helper()
	for _, stub := range stubs {
		if stub.ID == id {
			return stub
		}
	}
	t.Fatalf("node %s not listed", id)
	return nil
}

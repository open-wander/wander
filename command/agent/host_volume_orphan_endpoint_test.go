// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// seedOrphanNodes stores two nodes that advertise dynamic volumes with no
// record, and one that advertises a volume with a record. No client touches
// them.
func seedOrphanNodes(t *testing.T, s *TestAgent) (nodeA, nodeB string) {
	t.Helper()
	store := s.Agent.Server().State()
	must.NoError(t, store.UpsertNamespaces(90, []*structs.Namespace{{Name: "ns-b"}}))
	mk := func(vols ...*structs.ClientHostVolumeConfig) string {
		n := mock.Node()
		n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{}
		for _, v := range vols {
			n.HostVolumes[v.Name] = v
		}
		must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, 100, n))
		return n.ID
	}
	adv := func(id, name, ns string) *structs.ClientHostVolumeConfig {
		return &structs.ClientHostVolumeConfig{Name: name, Path: "/synthetic/" + id, ID: id, Namespace: ns}
	}
	nodeA = mk(adv("vol-a1", "a1", "default"), adv("vol-a2", "a2", "ns-b"), adv("vol-rec", "rec", "default"),
		&structs.ClientHostVolumeConfig{Name: "static", Path: "/synthetic/static"})
	nodeB = mk(adv("vol-b1", "b1", "default"))
	must.NoError(t, store.UpsertDynamicHostVolumes(110, []*structs.DynamicHostVolume{{
		ID: "vol-rec", Name: "rec", Namespace: "default", Type: structs.VolumeTypeHost,
		NodeID: nodeA, NodePool: "default", HostPath: "/synthetic/vol-rec", PluginID: "plug",
		State: structs.DynamicHostVolumeStateReady,
	}}))
	return nodeA, nodeB
}

func orphanIDs(t *testing.T, resp *httptest.ResponseRecorder) []string {
	t.Helper()
	must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
	var out []*structs.HostVolumeOrphan
	must.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out), must.Sprint(resp.Body.String()))
	ids := []string{}
	for _, o := range out {
		ids = append(ids, o.VolumeID)
	}
	slices.Sort(ids)
	return ids
}

func TestHTTP_HostVolumeOrphans_List(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		nodeA, _ := seedOrphanNodes(t, s)

		cases := []struct {
			name, query string
			want        []string
		}{
			{"default namespace", "", []string{"vol-a1", "vol-b1"}},
			{"wildcard", "?namespace=*", []string{"vol-a1", "vol-a2", "vol-b1"}},
			{"named namespace", "?namespace=ns-b", []string{"vol-a2"}},
			{"node", "?namespace=*&node_id=" + nodeA, []string{"vol-a1", "vol-a2"}},
			{"unknown node", "?namespace=*&node_id=5b0c0a5e-0000-4000-8000-000000000001", []string{}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				resp := hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans"+tc.query, "", nil)
				must.Eq(t, tc.want, orphanIDs(t, resp))
				must.NotEq(t, "", resp.Header().Get("X-Nomad-Index"))
				must.NotEq(t, "", resp.Header().Get("X-Nomad-KnownLeader"))
				must.StrNotContains(t, resp.Body.String(), "/synthetic")
			})
		}

		t.Run("an empty list is an array", func(t *testing.T) {
			resp := hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans?namespace=nope", "", nil)
			must.Eq(t, "[]", trimNewline(resp.Body.String()))
		})

		t.Run("other methods", func(t *testing.T) {
			for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
				resp := hostVolumeServe(t, s, m, "/v1/volumes/host/orphans", "", nil)
				must.Eq(t, 405, resp.Code, must.Sprint(m))
			}
		})

		t.Run("the volumes listing is unchanged", func(t *testing.T) {
			resp := hostVolumeServe(t, s, http.MethodGet, "/v1/volumes?type=host", "", nil)
			must.Eq(t, 200, resp.Code)
			resp = hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphan", "", nil)
			must.Eq(t, 404, resp.Code)
		})
	})
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n') {
		s = s[:len(s)-1]
	}
	return s
}

func TestHTTP_HostVolumeOrphans_ListACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		seedOrphanNodes(t, s)
		store := s.Agent.Server().State()
		tok := mock.CreatePolicyAndToken(t, store, 200, "reader",
			mock.NamespacePolicy("ns-b", "", []string{acl.NamespaceCapabilityHostVolumeRead}))

		resp := hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans?namespace=*", "", tok)
		must.Eq(t, []string{"vol-a2"}, orphanIDs(t, resp))
		resp = hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans", "", tok)
		must.Eq(t, 403, resp.Code)
		resp = hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans", "", nil)
		must.Eq(t, 403, resp.Code)
	})
}

func TestHTTP_HostVolumeOrphans_RemoveRouting(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		for _, tc := range []struct {
			method, path string
			code         int
		}{
			{http.MethodDelete, "/v1/volume/host/orphan/", 404},
			{http.MethodDelete, "/v1/volume/host/orphan/node", 404},
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol/extra", 404},
			{http.MethodDelete, "/v1/volume/host/orphan/node/", 404},
			{http.MethodGet, "/v1/volume/host/orphan/node/vol?mode=forget", 405},
			{http.MethodPost, "/v1/volume/host/orphan/node/vol?mode=forget", 405},
			{http.MethodPut, "/v1/volume/host/orphan/node/vol?mode=forget", 405},
			// The mode has no default.
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol", 400},
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol?mode=", 400},
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol?mode=remove", 400},
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol?mode=Delete", 400},
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol?mode=forget&namespace=*", 400},
			// A well-formed request for a volume no node advertises.
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol?mode=forget", 404},
			{http.MethodDelete, "/v1/volume/host/orphan/node/vol?mode=delete", 404},
			// The volume routes still reach their own handler.
			{http.MethodGet, "/v1/volume/host/vol-1", 200},
			// The bare word is redirected to the subtree, as the mux does for
			// every subtree route; volume IDs are UUIDs, so it costs nothing.
			{http.MethodGet, "/v1/volume/host/orphan", 307},
		} {
			resp := hostVolumeServe(t, s, tc.method, tc.path, "", nil)
			must.Eq(t, tc.code, resp.Code, must.Sprint(tc.method+" "+tc.path+": "+resp.Body.String()))
		}
	})
}

// An empty node segment never reaches the RPC, whether or not the mux has
// already cleaned the path.
func TestHTTP_HostVolumeOrphans_EmptySegment(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		req, err := http.NewRequest(http.MethodDelete, "/v1/volume/host/orphan//vol?mode=forget", nil)
		must.NoError(t, err)
		_, err = s.Server.HostVolumeOrphanSpecificRequest(httptest.NewRecorder(), req)
		var coded HTTPCodedError
		must.True(t, asCoded(err, &coded))
		must.Eq(t, 404, coded.Code())
	})
}

func TestHTTP_HostVolumeOrphans_RemoveACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		nodeA, _ := seedOrphanNodes(t, s)
		store := s.Agent.Server().State()
		reader := mock.CreatePolicyAndToken(t, store, 200, "reader",
			mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeRead}))

		path := "/v1/volume/host/orphan/" + nodeA + "/vol-a1?mode=forget"
		must.Eq(t, 403, hostVolumeServe(t, s, http.MethodDelete, path, "", reader).Code)
		must.Eq(t, 403, hostVolumeServe(t, s, http.MethodDelete, path, "", nil).Code)
	})
}

// A volume the client holds and the servers know nothing of is listed and
// removed through the real client: a registered one cannot be deleted, only
// forgotten, the directory stays and the client refuses the ID afterwards.
func TestHTTP_HostVolumeOrphans_RegisteredEndToEnd(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		nodeID := s.client.NodeID()
		testutil.WaitForResult(func() (bool, error) {
			node, err := s.Agent.Server().State().NodeByID(nil, nodeID)
			if err != nil || node == nil {
				return false, fmt.Errorf("node not stored: %v", err)
			}
			return node.Status == structs.NodeStatusReady, fmt.Errorf("node status %q", node.Status)
		}, func(err error) { must.NoError(t, err) })

		dir := t.TempDir()
		must.NoError(t, os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0o600))
		id := uuid.Generate()
		register := func() cstructs.HostVolumeRegisterResponse {
			var out cstructs.HostVolumeRegisterResponse
			must.NoError(t, s.client.ClientRPC("HostVolume.Register", &cstructs.HostVolumeRegisterRequest{
				VolumeID: id, Namespace: "default", Name: "orphaned", NodeID: nodeID, HostPath: dir,
			}, &out))
			return out
		}
		must.Eq(t, "ready", register().Status)

		listed := func() []string {
			return orphanIDs(t, hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans?node_id="+nodeID, "", nil))
		}
		testutil.WaitForResult(func() (bool, error) {
			got := listed()
			return len(got) == 1 && got[0] == id, fmt.Errorf("orphans: %v", got)
		}, func(err error) { must.NoError(t, err) })

		path := "/v1/volume/host/orphan/" + nodeID + "/" + id
		resp := hostVolumeServe(t, s, http.MethodDelete, path+"?mode=delete", "", nil)
		must.Eq(t, 400, resp.Code)
		must.StrContains(t, resp.Body.String(), "volume was registered, use forget")
		must.Eq(t, []string{id}, listed())

		before, idx := hostVolumeCount(t, s)
		resp = hostVolumeServe(t, s, http.MethodDelete, path+"?mode=forget", "", nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		count, idx2 := hostVolumeCount(t, s)
		must.Eq(t, before, count)
		must.Eq(t, idx, idx2, must.Sprint("an orphan removal wrote a record"))

		_, err := os.Stat(filepath.Join(dir, "marker"))
		must.NoError(t, err)
		testutil.WaitForResult(func() (bool, error) {
			got := listed()
			return len(got) == 0, fmt.Errorf("orphans: %v", got)
		}, func(err error) { must.NoError(t, err) })

		// The client wrote the tombstone: the same ID is refused.
		must.Eq(t, hvstate.ReasonVolumeDeleted, register().Reason)

		// A repeat names a volume the node no longer advertises.
		resp = hostVolumeServe(t, s, http.MethodDelete, path+"?mode=forget", "", nil)
		must.Eq(t, 404, resp.Code)
	})
}

// A volume that a plugin created and the servers hold no record of is removed
// in delete mode through the real server RPC and the real client: the plugin
// delete runs exactly once, the client record is gone and the client refuses
// the ID afterwards.
func TestHTTP_HostVolumeOrphans_PluginDeleteEndToEnd(t *testing.T) {
	ci.Parallel(t)

	pluginDir, volumesDir, scratch := t.TempDir(), t.TempDir(), t.TempDir()
	body := "#!/bin/sh\nS='" + scratch + `'
case "$1" in
fingerprint) echo '{"version":"1.0.0"}' ;;
create)
  mkdir -p "$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
  printf '{"path":"%s"}\n' "$DHV_VOLUMES_DIR/$DHV_VOLUME_ID" ;;
delete)
  echo "$DHV_VOLUME_ID" >> "$S/deletes"
  rm -rf "$DHV_CREATED_PATH" ;;
esac
`
	must.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plug"), []byte(body), 0o755))

	httpTest(t, func(c *Config) {
		c.Client.HostVolumePluginDir = pluginDir
		c.Client.HostVolumesDir = volumesDir
	}, func(s *TestAgent) {
		nodeID := s.client.NodeID()
		testutil.WaitForResult(func() (bool, error) {
			node, err := s.Agent.Server().State().NodeByID(nil, nodeID)
			if err != nil || node == nil {
				return false, fmt.Errorf("node not stored: %v", err)
			}
			return node.Status == structs.NodeStatusReady, fmt.Errorf("node status %q", node.Status)
		}, func(err error) { must.NoError(t, err) })

		id := uuid.Generate()
		create := func() cstructs.HostVolumeCreateResponse {
			var out cstructs.HostVolumeCreateResponse
			must.NoError(t, s.client.ClientRPC("HostVolume.Create", &cstructs.HostVolumeCreateRequest{
				VolumeID: id, Namespace: "default", Name: "made", NodeID: nodeID, PluginID: "plug",
			}, &out))
			return out
		}
		must.Eq(t, "ready", create().Status)
		dir := filepath.Join(volumesDir, id)
		_, err := os.Stat(dir)
		must.NoError(t, err)

		listed := func() []string {
			return orphanIDs(t, hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/host/orphans?node_id="+nodeID, "", nil))
		}
		testutil.WaitForResult(func() (bool, error) {
			got := listed()
			return len(got) == 1 && got[0] == id, fmt.Errorf("orphans: %v", got)
		}, func(err error) { must.NoError(t, err) })

		path := "/v1/volume/host/orphan/" + nodeID + "/" + id + "?mode=delete"
		resp := hostVolumeServe(t, s, http.MethodDelete, path, "", nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))

		raw, err := os.ReadFile(filepath.Join(scratch, "deletes"))
		must.NoError(t, err)
		must.Eq(t, []string{id}, strings.Fields(string(raw)), must.Sprint("the plugin delete did not run exactly once"))
		_, err = os.Stat(dir)
		must.True(t, os.IsNotExist(err))

		// The client record is gone: the node stops advertising the volume.
		testutil.WaitForResult(func() (bool, error) {
			got := listed()
			return len(got) == 0, fmt.Errorf("orphans: %v", got)
		}, func(err error) { must.NoError(t, err) })

		// The client wrote the tombstone: the same ID is refused.
		must.Eq(t, hvstate.ReasonVolumeDeleted, create().Reason)

		// A repeat names a volume the node no longer advertises, and no plugin
		// runs again.
		resp = hostVolumeServe(t, s, http.MethodDelete, path, "", nil)
		must.Eq(t, 404, resp.Code)
		raw, err = os.ReadFile(filepath.Join(scratch, "deletes"))
		must.NoError(t, err)
		must.Eq(t, 1, len(strings.Fields(string(raw))))
	})
}

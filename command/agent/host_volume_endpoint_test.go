// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func seedHTTPHostVolumes(t *testing.T, s *TestAgent) {
	t.Helper()
	store := s.Agent.Server().State()
	must.NoError(t, store.UpsertNamespaces(90, []*structs.Namespace{{Name: "ns-b"}}))
	vols := []*structs.DynamicHostVolume{}
	for _, v := range []struct{ ns, id, node string }{
		{"default", "vol-1", "node-1"}, {"default", "vol-2", "node-2"}, {"ns-b", "vol-3", "node-1"},
	} {
		vols = append(vols, &structs.DynamicHostVolume{
			ID: v.id, Name: "name-" + v.id, Namespace: v.ns, Type: structs.VolumeTypeHost,
			NodeID: v.node, NodePool: "default", HostPath: "/synthetic/" + v.id, PluginID: "plug",
			State: structs.DynamicHostVolumeStateReady,
		})
	}
	must.NoError(t, store.UpsertDynamicHostVolumes(100, vols))
}

func TestHTTP_HostVolumeList(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		req, err := http.NewRequest(http.MethodGet, "/v1/volumes?type=host", nil)
		must.NoError(t, err)
		resp := httptest.NewRecorder()
		obj, err := s.Server.CSIVolumesRequest(resp, req)
		must.NoError(t, err)

		stubs, ok := obj.([]*structs.DynamicHostVolumeStub)
		must.True(t, ok)
		must.Len(t, 2, stubs)
		must.Eq(t, "100", resp.Header().Get("X-Nomad-Index"))
		must.NotEq(t, "", resp.Header().Get("X-Nomad-KnownLeader"))

		// Filters and namespace flow through the query string.
		req, err = http.NewRequest(http.MethodGet, "/v1/volumes?type=host&namespace=*&node_id=node-1&prefix=vol", nil)
		must.NoError(t, err)
		obj, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		must.Len(t, 2, obj.([]*structs.DynamicHostVolumeStub))

		req, err = http.NewRequest(http.MethodGet, "/v1/volumes?type=host&namespace=*&node_pool=nope", nil)
		must.NoError(t, err)
		obj, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		must.Len(t, 0, obj.([]*structs.DynamicHostVolumeStub))
	})
}

// TestHTTP_HostVolumeList_Query covers the query-string parameters that narrow
// or page a listing.
func TestHTTP_HostVolumeList_Query(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		list := func(query string) ([]string, string) {
			t.Helper()
			req, err := http.NewRequest(http.MethodGet, "/v1/volumes?type=host&"+query, nil)
			must.NoError(t, err)
			resp := httptest.NewRecorder()
			obj, err := s.Server.CSIVolumesRequest(resp, req)
			must.NoError(t, err)
			ids := []string{}
			for _, stub := range obj.([]*structs.DynamicHostVolumeStub) {
				ids = append(ids, stub.Namespace+"/"+stub.ID)
			}
			return ids, resp.Header().Get("X-Nomad-NextToken")
		}

		t.Run("prefix narrows a namespace", func(t *testing.T) {
			ids, _ := list("prefix=vol-2")
			must.Eq(t, []string{"default/vol-2"}, ids)
		})

		t.Run("prefix narrows a wildcard", func(t *testing.T) {
			ids, _ := list("namespace=*&prefix=vol-3")
			must.Eq(t, []string{"ns-b/vol-3"}, ids)
		})

		t.Run("prefix matching nothing", func(t *testing.T) {
			ids, _ := list("namespace=*&prefix=zzz")
			must.Eq(t, []string{}, ids)
		})

		t.Run("filter expression", func(t *testing.T) {
			ids, _ := list("namespace=*&filter=" + url.QueryEscape(`NodeID == "node-2"`))
			must.Eq(t, []string{"default/vol-2"}, ids)
		})

		t.Run("per_page and next_token", func(t *testing.T) {
			first, token := list("namespace=*&per_page=2")
			must.Eq(t, []string{"default/vol-1", "default/vol-2"}, first)
			must.NotEq(t, "", token)

			second, token := list("namespace=*&per_page=2&next_token=" + url.QueryEscape(token))
			must.Eq(t, []string{"ns-b/vol-3"}, second)
			must.Eq(t, "", token)
		})
	})
}

func TestHTTP_HostVolumeList_ACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)
		store := s.Agent.Server().State()
		tok := mock.CreatePolicyAndToken(t, store, 200, "reader",
			mock.NamespacePolicy("ns-b", "", []string{acl.NamespaceCapabilityHostVolumeRead}))

		req, err := http.NewRequest(http.MethodGet, "/v1/volumes?type=host&namespace=*", nil)
		must.NoError(t, err)
		setToken(req, tok)
		obj, err := s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		stubs := obj.([]*structs.DynamicHostVolumeStub)
		must.Len(t, 1, stubs)
		must.Eq(t, "ns-b", stubs[0].Namespace)

		req, err = http.NewRequest(http.MethodGet, "/v1/volumes?type=host", nil)
		must.NoError(t, err)
		setToken(req, tok)
		_, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.ErrorContains(t, err, "Permission denied")
	})
}

func TestHTTP_HostVolumeGet(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		req, err := http.NewRequest(http.MethodGet, "/v1/volume/host/vol-1", nil)
		must.NoError(t, err)
		resp := httptest.NewRecorder()
		obj, err := s.Server.HostVolumeSpecificRequest(resp, req)
		must.NoError(t, err)
		vol, ok := obj.(*structs.DynamicHostVolume)
		must.True(t, ok)
		must.Eq(t, "vol-1", vol.ID)
		must.Eq(t, "/synthetic/vol-1", vol.HostPath)
		must.Eq(t, "100", resp.Header().Get("X-Nomad-Index"))

		// Namespace query parameter selects the volume's namespace.
		req, err = http.NewRequest(http.MethodGet, "/v1/volume/host/vol-3?namespace=ns-b", nil)
		must.NoError(t, err)
		obj, err = s.Server.HostVolumeSpecificRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		must.Eq(t, "ns-b", obj.(*structs.DynamicHostVolume).Namespace)

		// Unknown ID is a 404 and still carries the index header.
		req, err = http.NewRequest(http.MethodGet, "/v1/volume/host/nope", nil)
		must.NoError(t, err)
		resp = httptest.NewRecorder()
		_, err = s.Server.HostVolumeSpecificRequest(resp, req)
		must.Error(t, err)
		var coded HTTPCodedError
		must.True(t, asCoded(err, &coded))
		must.Eq(t, 404, coded.Code())
		must.Eq(t, "100", resp.Header().Get("X-Nomad-Index"))

		// Other methods and extra or empty segments.
		for _, tc := range []struct {
			method, path string
			code         int
		}{
			{http.MethodPost, "/v1/volume/host/x", 405},
			{http.MethodPut, "/v1/volume/host/x", 405},
			{http.MethodDelete, "/v1/volume/host/x", 405},
			{http.MethodGet, "/v1/volume/host/", 404},
			{http.MethodGet, "/v1/volume/host/vol-1/extra", 404},
		} {
			req, err = http.NewRequest(tc.method, tc.path, nil)
			must.NoError(t, err)
			_, err = s.Server.HostVolumeSpecificRequest(httptest.NewRecorder(), req)
			must.True(t, asCoded(err, &coded), must.Sprint(tc.method+" "+tc.path))
			must.Eq(t, tc.code, coded.Code(), must.Sprint(tc.method+" "+tc.path))
		}
	})
}

// TestHTTP_HostVolume_Routes goes through the real mux.
func TestHTTP_HostVolume_Routes(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		for path, code := range map[string]int{
			"/v1/volumes?type=host":             200,
			"/v1/volume/host/vol-1":             200,
			"/v1/volume/host/nope":              404,
			"/v1/volume/host/vol-1?namespace=*": 400,
		} {
			req, err := http.NewRequest(http.MethodGet, path, nil)
			must.NoError(t, err)
			resp := httptest.NewRecorder()
			s.Server.mux.ServeHTTP(resp, req)
			must.Eq(t, code, resp.Code, must.Sprint(path))
		}

		req, err := http.NewRequest(http.MethodPost, "/v1/volume/host/x", nil)
		must.NoError(t, err)
		resp := httptest.NewRecorder()
		s.Server.mux.ServeHTTP(resp, req)
		must.Eq(t, 405, resp.Code)
	})
}

// TestHTTP_VolumesType_Unchanged guards the existing /v1/volumes behaviour for
// every type other than host.
func TestHTTP_VolumesType_Unchanged(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		req, err := http.NewRequest(http.MethodGet, "/v1/volumes?type=csi", nil)
		must.NoError(t, err)
		obj, err := s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		_, ok := obj.([]*structs.CSIVolListStub)
		must.True(t, ok)

		req, err = http.NewRequest(http.MethodGet, "/v1/volumes", nil)
		must.NoError(t, err)
		obj, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		must.Eq(t, []*structs.CSIVolListStub{}, obj.([]*structs.CSIVolListStub))

		req, err = http.NewRequest(http.MethodGet, "/v1/volumes?type=bogus", nil)
		must.NoError(t, err)
		obj, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.NoError(t, err)
		must.Nil(t, obj)

		// PUT still goes to CSI registration, not the host list.
		req, err = http.NewRequest(http.MethodPut, "/v1/volumes?type=host", strings.NewReader("not json"))
		must.NoError(t, err)
		_, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.Error(t, err)
		var coded HTTPCodedError
		must.True(t, asCoded(err, &coded))
		must.Eq(t, 400, coded.Code())

		req, err = http.NewRequest(http.MethodDelete, "/v1/volumes?type=host", nil)
		must.NoError(t, err)
		_, err = s.Server.CSIVolumesRequest(httptest.NewRecorder(), req)
		must.True(t, asCoded(err, &coded))
		must.Eq(t, 405, coded.Code())
	})
}

func asCoded(err error, out *HTTPCodedError) bool {
	return errors.As(err, out)
}

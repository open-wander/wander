// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
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

// hostVolumeServe sends a request through the real mux, so status codes and
// bodies come from wrap.
func hostVolumeServe(t *testing.T, s *TestAgent, method, path, body string, token *structs.ACLToken) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader = http.NoBody
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, path, rdr)
	must.NoError(t, err)
	if token != nil {
		setToken(req, token)
	}
	resp := httptest.NewRecorder()
	s.Server.mux.ServeHTTP(resp, req)
	return resp
}

type hostVolumeWriteReply struct {
	Volume   map[string]any
	Warnings *string
}

func decodeHostVolumeWrite(t *testing.T, resp *httptest.ResponseRecorder) hostVolumeWriteReply {
	t.Helper()
	var out hostVolumeWriteReply
	must.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out), must.Sprint(resp.Body.String()))
	return out
}

// hostVolumeCount returns the number of stored volumes and the table index, so
// a test can show that a refused request changed nothing.
func hostVolumeCount(t *testing.T, s *TestAgent) (int, uint64) {
	t.Helper()
	snap, err := s.Agent.Server().State().Snapshot()
	must.NoError(t, err)
	iter, err := snap.DynamicHostVolumes(nil)
	must.NoError(t, err)
	n := 0
	for iter.Next() != nil {
		n++
	}
	idx, err := snap.Index("dynamic_host_volumes")
	must.NoError(t, err)
	return n, idx
}

// seedWritableHostVolumes stores one ready volume per origin on nodes no
// client touches, plus one that is not ready.
func seedWritableHostVolumes(t *testing.T, s *TestAgent) {
	t.Helper()
	store := s.Agent.Server().State()
	// The seed must sit below the index of the writes the test then makes
	// through Raft.
	idx, err := store.LatestIndex()
	must.NoError(t, err)
	must.NoError(t, store.UpsertNamespaces(idx, []*structs.Namespace{{Name: "ns-b"}}))
	mk := func(id, name string, origin structs.DynamicHostVolumeOrigin, st structs.DynamicHostVolumeState) *structs.DynamicHostVolume {
		return &structs.DynamicHostVolume{
			ID: id, Name: name, Namespace: "default", Type: structs.VolumeTypeHost,
			NodeID: "node-1", NodePool: "default", HostPath: "/synthetic/" + id, PluginID: "plug",
			Origin: origin, State: st,
		}
	}
	reg := mk("reg-1", "reg-name", structs.DynamicHostVolumeOriginRegistered, structs.DynamicHostVolumeStateReady)
	reg.PluginID = ""
	must.NoError(t, store.UpsertDynamicHostVolumes(idx, []*structs.DynamicHostVolume{
		mk("pc-1", "pc-name", structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeStateReady),
		reg,
		mk("pc-failed", "pc-failed-name", structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeStateFailed),
	}))
}

const hostVolumeCaps = `"RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]`

const hostVolumeNoNodeBody = `{"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`

func TestHTTP_HostVolumeWrite_Routes(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)

		for _, tc := range []struct {
			method, path string
			code         int
		}{
			{http.MethodGet, "/v1/volume/host/create", 405},
			{http.MethodGet, "/v1/volume/host/register", 405},
			{http.MethodDelete, "/v1/volume/host/create", 405},
			{http.MethodDelete, "/v1/volume/host/register", 405},
			{http.MethodPatch, "/v1/volume/host/create", 405},
			{http.MethodPut, "/v1/volume/host/create/extra", 404},
			{http.MethodPut, "/v1/volume/host/register/extra", 404},
			{http.MethodPost, "/v1/volume/host/x", 405},
			{http.MethodPut, "/v1/volume/host/vol-1", 405},
			{http.MethodGet, "/v1/volume/host/vol-1", 200},
			// PUT and POST reach the handler, which refuses the empty body.
			{http.MethodPut, "/v1/volume/host/create", 400},
			{http.MethodPost, "/v1/volume/host/create", 400},
			{http.MethodPut, "/v1/volume/host/register", 400},
			{http.MethodPost, "/v1/volume/host/register", 400},
		} {
			resp := hostVolumeServe(t, s, tc.method, tc.path, "", nil)
			must.Eq(t, tc.code, resp.Code, must.Sprint(tc.method+" "+tc.path))
		}
	})
}

func TestHTTP_HostVolumeCreate_Response(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)

		for _, method := range []string{http.MethodPut, http.MethodPost} {
			resp := hostVolumeServe(t, s, method, "/v1/volume/host/create", hostVolumeNoNodeBody, nil)
			must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
			must.NotEq(t, "", resp.Header().Get("X-Nomad-Index"))
			out := decodeHostVolumeWrite(t, resp)
			must.Nil(t, out.Warnings)
			must.Eq(t, "failed", out.Volume["State"])
			must.StrContains(t, out.Volume["StatusDescription"].(string), "no_eligible_node")
			must.Eq(t, "default", out.Volume["Namespace"])
			id := out.Volume["ID"].(string)
			must.NotEq(t, "", id)

			// The reply volume has the field names of the read API.
			get := hostVolumeServe(t, s, http.MethodGet, "/v1/volume/host/"+id, "", nil)
			must.Eq(t, 200, get.Code)
			var read map[string]any
			must.NoError(t, json.Unmarshal(get.Body.Bytes(), &read))
			must.Eq(t, slices.Sorted(maps.Keys(read)), slices.Sorted(maps.Keys(out.Volume)))
		}

		// A null Warnings is in the body, not an omitted key or an empty string.
		resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", hostVolumeNoNodeBody, nil)
		must.StrContains(t, resp.Body.String(), `"Warnings":null`)
	})
}

func TestHTTP_HostVolumeWrite_InvalidInput(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPHostVolumes(t, s)
		beforeN, beforeIdx := hostVolumeCount(t, s)

		for _, tc := range []struct {
			name, body, msg string
		}{
			{"empty body", "", "invalid request body"},
			{"malformed", `{"Volume": {"Name": `, "invalid request body"},
			{"not json", `secret-looking-text`, "invalid request body"},
			{"array", `[]`, "invalid request body"},
			{"trailing data", `{"Volume":{"Name":"a"}} {"x":1}`, "invalid request body"},
			{"wrong type", `{"Volume":{"Name":"a"},"PolicyOverride":"yes"}`, "invalid request body"},
			{"volume wrong type", `{"Volume":"a"}`, "invalid request body"},
			{"no volume", `{}`, "missing volume"},
			{"null volume", `{"Volume":null}`, "missing volume"},
			{"json null", `null`, "missing volume"},
			// A repeated top-level key is refused: the decoder merges the two
			// occurrences while the unknown-field check would see only the last.
			{"repeated volume", `{"Volume":{"Name":"a","Bogus":1},"Volume":{"Name":"b"}}`, "invalid request body"},
			{"repeated volume merging to a valid volume", `{"Volume":{"Name":"data-a","Bogus":1},"Volume":{"PluginID":"mkdir","NodePool":"pool-none",` + hostVolumeCaps + `}}`, "invalid request body"},
			{"repeated key in another case", `{"volume":{"Name":"a"},"VOLUME":{"Name":"b"}}`, "invalid request body"},
			{"repeated key under case folding", `{"\u212a":1,"k":2,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none",` + hostVolumeCaps + `}}`, "invalid request body"},
			{"repeated unknown key", `{"X":1,"x":2,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none",` + hostVolumeCaps + `}}`, "invalid request body"},
			{"repeated policy override", `{"PolicyOverride":true,"PolicyOverride":false,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none",` + hostVolumeCaps + `}}`, "invalid request body"},
		} {
			for _, path := range []string{"/v1/volume/host/create", "/v1/volume/host/register"} {
				resp := hostVolumeServe(t, s, http.MethodPut, path, tc.body, nil)
				must.Eq(t, 400, resp.Code, must.Sprint(tc.name+" "+path))
				must.Eq(t, tc.msg, resp.Body.String(), must.Sprint(tc.name+" "+path))
			}
		}

		n, idx := hostVolumeCount(t, s)
		must.Eq(t, beforeN, n)
		must.Eq(t, beforeIdx, idx)
	})
}

func TestHostVolumeRepeatedKey(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		body     string
		repeated bool
	}{
		{`{}`, false},
		{`null`, false},
		{`{"Volume":{"Name":"a","Name":"b"},"X":[{"X":1},{"X":2}]}`, false},
		{`{"Volume":1,"PolicyOverride":2,"Vol":3}`, false},
		{`{"Volume":1,"Volume":2}`, true},
		{`{"Volume":1,"vOLUME":2}`, true},
		{`{"\u212a":1,"K":2}`, true},
		{`{"\u017f":1,"S":2}`, true},
		{`{"a":{"b":[1,{"c":2}]},"A":null}`, true},
	} {
		err := hostVolumeRepeatedKey([]byte(tc.body))
		must.Eq(t, tc.repeated, err != nil, must.Sprint(tc.body))
	}
}

func TestFoldHostVolumeKey(t *testing.T) {
	ci.Parallel(t)
	// The Kelvin sign and the long s fold with k and s; sharp s does not fold
	// with ss under simple folding.
	keys := []string{"Volume", "volume", "VOLUME", "k", "K", "\u212a", "s", "S", "\u017f", "ss", "\u00df", "Name", "Nam", "a\x00b", ""}
	for _, a := range keys {
		for _, b := range keys {
			must.Eq(t, strings.EqualFold(a, b), foldHostVolumeKey(a) == foldHostVolumeKey(b), must.Sprintf("%q %q", a, b))
		}
	}
}

// A body over hostVolumeMaxBody is refused before any RPC call; one at the
// limit is read in full.
func TestHTTP_HostVolumeWrite_BodyLimit(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)

		// sized returns a valid create body of exactly n bytes, padded with an
		// unknown top-level key.
		sized := func(n int) string {
			head := `{"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none",` + hostVolumeCaps + `},"Pad":"`
			tail := `"}`
			body := head + strings.Repeat("x", n-len(head)-len(tail)) + tail
			must.Eq(t, n, len(body))
			return body
		}

		for _, path := range []string{"/v1/volume/host/create", "/v1/volume/host/register"} {
			beforeN, beforeIdx := hostVolumeCount(t, s)
			resp := hostVolumeServe(t, s, http.MethodPut, path, sized(hostVolumeMaxBody+1), nil)
			must.Eq(t, 400, resp.Code, must.Sprint(path))
			must.Eq(t, "invalid request body", resp.Body.String(), must.Sprint(path))
			n, idx := hostVolumeCount(t, s)
			must.Eq(t, beforeN, n, must.Sprint(path))
			must.Eq(t, beforeIdx, idx, must.Sprint(path))
		}

		for _, size := range []int{hostVolumeMaxBody / 2, hostVolumeMaxBody} {
			resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", sized(size), nil)
			must.Eq(t, 200, resp.Code, must.Sprint(size))
			out := decodeHostVolumeWrite(t, resp)
			must.Eq(t, "ignored unknown fields: Pad", *out.Warnings)
			must.Eq(t, "failed", out.Volume["State"])
		}
	})
}

func TestHTTP_HostVolumeWrite_Namespace(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)

		// The body namespace is the request namespace when the query has none.
		body := `{"Volume":{"Name":"data-a","Namespace":"ns-b","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`
		resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", body, nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		must.Eq(t, "ns-b", decodeHostVolumeWrite(t, resp).Volume["Namespace"])

		// The query namespace alone also works.
		resp = hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create?namespace=ns-b", hostVolumeNoNodeBody, nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		must.Eq(t, "ns-b", decodeHostVolumeWrite(t, resp).Volume["Namespace"])

		// Two different namespaces are refused and nothing is written.
		n, idx := hostVolumeCount(t, s)
		resp = hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create?namespace=default", body, nil)
		must.Eq(t, 400, resp.Code)
		must.Eq(t, "volume namespace does not match the request namespace", resp.Body.String())
		n2, idx2 := hostVolumeCount(t, s)
		must.Eq(t, n, n2)
		must.Eq(t, idx, idx2)
	})
}

func TestHTTP_HostVolumeWrite_UnknownFieldsAndOverride(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)

		post := func(body string) hostVolumeWriteReply {
			t.Helper()
			resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", body, nil)
			must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
			return decodeHostVolumeWrite(t, resp)
		}

		t.Run("known fields give no warning", func(t *testing.T) {
			out := post(`{"volume":{"name":"data-a","pluginid":"mkdir","nodepool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]},"policyoverride":false}`)
			must.Nil(t, out.Warnings)
			must.Eq(t, "data-a", out.Volume["Name"])
		})

		t.Run("unknown fields are named once, sorted", func(t *testing.T) {
			// Two names that clean to the same text are named once.
			out := post(`{"Zeta":1,"Alpha":{"x":1},"a\u0000b":1,"a\u0001b":2,"Volume":{"Name":"data-a","PluginID":"mkdir",` +
				`"NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}],"Beta":true,"Constraints":[{"LTarget":"${attr.kernel.name}","RTarget":"linux","Operand":"=","Nested":1}]}}`)
			must.NotNil(t, out.Warnings)
			must.Eq(t, "ignored unknown fields: Alpha, Volume.Beta, Zeta, a?b", *out.Warnings)
		})

		t.Run("names are bounded and printable", func(t *testing.T) {
			fields := []string{}
			for i := range 40 {
				fields = append(fields, fmt.Sprintf(`"F%02d":1`, i))
			}
			fields = append(fields, `"`+strings.Repeat("L", 200)+`":1`, `"a\u0000b":1`)
			out := post(`{` + strings.Join(fields, ",") + `,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`)
			must.NotNil(t, out.Warnings)
			w := *out.Warnings
			must.True(t, len(w) < 600, must.Sprint(len(w)))
			must.StrContains(t, w, "and 28 more")
			must.StrNotContains(t, w, "\x00")
			must.StrNotContains(t, w, strings.Repeat("L", 100))
		})

		t.Run("long and unprintable names are cut and cleaned", func(t *testing.T) {
			out := post(`{"` + strings.Repeat("L", 200) + `":1,"a\u0000b":1,"Volume":{"Name":"data-a","PluginID":"mkdir",` +
				`"NodePool":"pool-none",` + hostVolumeCaps + `}}`)
			must.Eq(t, "ignored unknown fields: "+strings.Repeat("L", 64)+"..., a?b", *out.Warnings)
		})

		t.Run("policy override has no effect", func(t *testing.T) {
			out := post(`{"PolicyOverride":true,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`)
			must.NotNil(t, out.Warnings)
			must.Eq(t, "PolicyOverride has no effect: Wander has no policy engine", *out.Warnings)
			must.Eq(t, "failed", out.Volume["State"])
		})

		t.Run("both warnings", func(t *testing.T) {
			out := post(`{"PolicyOverride":true,"X":1,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`)
			must.Eq(t, "PolicyOverride has no effect: Wander has no policy engine\nignored unknown fields: X", *out.Warnings)
		})
	})
}

// A top-level key of the body must not set a field of the RPC request.
func TestHTTP_HostVolumeWrite_BodyCannotSetRequestFields(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)
		beforeN, beforeIdx := hostVolumeCount(t, s)

		// The management secret in the body is not a credential.
		body := fmt.Sprintf(`{"AuthToken":%q,"Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`, s.RootToken.SecretID)
		resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", body, nil)
		must.Eq(t, 403, resp.Code, must.Sprint(resp.Body.String()))

		// A namespace in the body does not name the request namespace.
		store := s.Agent.Server().State()
		tok := mock.CreatePolicyAndToken(t, store, 200, "creator",
			mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeCreate}))
		body = `{"Namespace":"ns-b","Volume":{"Name":"data-a","PluginID":"mkdir","NodePool":"pool-none","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}]}}`
		resp = hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", body, tok)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		must.Eq(t, "default", decodeHostVolumeWrite(t, resp).Volume["Namespace"])

		n, idx := hostVolumeCount(t, s)
		must.Eq(t, beforeN+1, n)
		must.Greater(t, beforeIdx, idx)
	})
}

func TestHTTP_HostVolumeWrite_ACL(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)
		store := s.Agent.Server().State()

		tokens := map[string]*structs.ACLToken{"none": nil}
		for i, c := range []struct {
			name string
			caps []string
		}{
			{"read", []string{acl.NamespaceCapabilityHostVolumeRead}},
			{"create", []string{acl.NamespaceCapabilityHostVolumeCreate}},
			{"register", []string{acl.NamespaceCapabilityHostVolumeRegister}},
			{"write", []string{acl.NamespaceCapabilityHostVolumeWrite}},
			{"delete", []string{acl.NamespaceCapabilityHostVolumeDelete}},
		} {
			tokens[c.name] = mock.CreatePolicyAndToken(t, store, uint64(200+i*2), "dhv-"+c.name,
				mock.NamespacePolicy("default", "", c.caps))
		}

		// Both bodies pass the ACL and are then refused or accepted by the RPC:
		// a create with no eligible node is 200, a register with no node is 400.
		createBody := hostVolumeNoNodeBody
		registerBody := `{"Volume":{"Name":"data-a"}}`
		for _, tc := range []struct {
			token, path, body string
			code              int
		}{
			{"none", "create", createBody, 403},
			{"none", "register", registerBody, 403},
			{"read", "create", createBody, 403},
			{"read", "register", registerBody, 403},
			{"delete", "create", createBody, 403},
			{"create", "create", createBody, 200},
			{"create", "register", registerBody, 403},
			{"register", "register", registerBody, 400},
			{"write", "create", createBody, 200},
			{"write", "register", registerBody, 400},
		} {
			resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/"+tc.path, tc.body, tokens[tc.token])
			must.Eq(t, tc.code, resp.Code, must.Sprint(tc.token+" "+tc.path+": "+resp.Body.String()))
			if tc.code == 403 {
				must.Eq(t, "Permission denied", resp.Body.String())
			}
		}

		// Update needs the same capability through the same endpoint.
		upd := `{"Volume":{"ID":"pc-1","Name":"pc-name","PluginID":"plug","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}],"Parameters":{"k":"v"}}}`
		resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", upd, tokens["read"])
		must.Eq(t, 403, resp.Code)
		resp = hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", upd, tokens["register"])
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		resp = hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create", upd, tokens["write"])
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
	})
}

func TestHTTP_HostVolumeWrite_Update(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedWritableHostVolumes(t, s)
		_, before := hostVolumeCount(t, s)

		// Update through create changes the mutable fields and moves the index.
		resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/create",
			`{"Volume":{"ID":"pc-1","Name":"pc-name","PluginID":"plug","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}],"Parameters":{"k":"v"}}}`, nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		out := decodeHostVolumeWrite(t, resp)
		must.Eq(t, "pc-1", out.Volume["ID"])
		must.Eq(t, any(map[string]any{"k": "v"}), out.Volume["Parameters"])
		must.Greater(t, float64(before), out.Volume["ModifyIndex"].(float64))
		must.Eq(t, strconv.FormatUint(uint64(out.Volume["ModifyIndex"].(float64)), 10), resp.Header().Get("X-Nomad-Index"))
		must.Eq(t, "ready", out.Volume["State"])

		// Update through register, for a registered volume.
		resp = hostVolumeServe(t, s, http.MethodPost, "/v1/volume/host/register",
			`{"Volume":{"ID":"reg-1","Name":"reg-name","RequestedCapabilities":[{"AccessMode":"single-node-writer","AttachmentMode":"file-system"}],"Parameters":{"a":"b"}}}`, nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		must.Eq(t, any(map[string]any{"a": "b"}), decodeHostVolumeWrite(t, resp).Volume["Parameters"])

		// The stored record is what changed.
		get := hostVolumeServe(t, s, http.MethodGet, "/v1/volume/host/reg-1", "", nil)
		must.StrContains(t, get.Body.String(), `"a":"b"`)

		// The stored volume sent back unchanged writes nothing: the reply is the
		// stored volume and X-Nomad-Index is its modify index.
		for _, tc := range []struct{ id, method, path string }{
			{"pc-1", http.MethodPut, "create"},
			{"reg-1", http.MethodPost, "register"},
		} {
			get := hostVolumeServe(t, s, http.MethodGet, "/v1/volume/host/"+tc.id, "", nil)
			must.Eq(t, 200, get.Code)
			var stored map[string]any
			must.NoError(t, json.Unmarshal(get.Body.Bytes(), &stored))
			storedIndex := uint64(stored["ModifyIndex"].(float64))
			_, idx := hostVolumeCount(t, s)

			resp := hostVolumeServe(t, s, tc.method, "/v1/volume/host/"+tc.path, `{"Volume":`+get.Body.String()+`}`, nil)
			must.Eq(t, 200, resp.Code, must.Sprint(tc.path+": "+resp.Body.String()))
			out := decodeHostVolumeWrite(t, resp)
			must.Nil(t, out.Warnings, must.Sprint(tc.path))
			must.Eq(t, any(tc.id), out.Volume["ID"])
			must.Eq(t, storedIndex, uint64(out.Volume["ModifyIndex"].(float64)), must.Sprint(tc.path))
			must.Eq(t, strconv.FormatUint(storedIndex, 10), resp.Header().Get("X-Nomad-Index"), must.Sprint(tc.path))
			_, idx2 := hostVolumeCount(t, s)
			must.Eq(t, idx, idx2, must.Sprint(tc.path))
		}

		for _, tc := range []struct {
			name, path, body string
			code             int
			msg              string
		}{
			{"unknown ID", "create", `{"Volume":{"ID":"nope","Name":"n","PluginID":"plug"}}`, 404, "volume not found"},
			{"not ready", "create", `{"Volume":{"ID":"pc-failed","Name":"pc-failed-name","PluginID":"plug"}}`, 409, `volume state is "failed"`},
			{"plugin changed", "create", `{"Volume":{"ID":"pc-1","Name":"pc-name","PluginID":"other"}}`, 400, "immutable field PluginID"},
			{"pool changed", "create", `{"Volume":{"ID":"pc-1","Name":"pc-name","PluginID":"plug","NodePool":"other"}}`, 400, "immutable field NodePool"},
			{"create on registered", "create", `{"Volume":{"ID":"reg-1","Name":"reg-name"}}`, 400, "volume origin mismatch"},
			{"register on plugin-created", "register", `{"Volume":{"ID":"pc-1","Name":"pc-name","PluginID":"plug"}}`, 400, "volume origin mismatch"},
			{"register unknown ID", "register", `{"Volume":{"ID":"nope","Name":"n"}}`, 404, "volume not found"},
		} {
			_, idx := hostVolumeCount(t, s)
			resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/"+tc.path, tc.body, nil)
			must.Eq(t, tc.code, resp.Code, must.Sprint(tc.name+": "+resp.Body.String()))
			must.Eq(t, tc.msg, resp.Body.String(), must.Sprint(tc.name))
			_, idx2 := hostVolumeCount(t, s)
			must.Eq(t, idx, idx2, must.Sprint(tc.name))
		}
	})
}

// A coded error of the RPC keeps its status and text through the handler
// wrapper, as it does when the call was forwarded to another server or region:
// a forwarded error reaches the agent as the text of the remote error. This
// characterises the inherited wrapper; the routes are covered by the tests above.
func TestHTTP_HostVolumeWrite_CodedErrors(t *testing.T) {
	ci.Parallel(t)
	for _, code := range []int{400, 403, 404, 409, 500, 502, 503} {
		msg := fmt.Sprintf("fixed text for %d", code)
		coded := structs.NewErrRPCCoded(code, msg)
		for name, err := range map[string]error{
			"direct":    coded,
			"forwarded": errors.New(coded.Error()),
		} {
			gotCode, gotMsg := errCodeFromHandler(err)
			must.Eq(t, code, gotCode, must.Sprint(name))
			must.Eq(t, msg, gotMsg, must.Sprint(name))
		}
	}
}

// A register of a directory on the agent's own node reaches the client and
// returns the ready volume.
func TestHTTP_HostVolumeRegister_Ready(t *testing.T) {
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
		body := fmt.Sprintf(`{"Volume":{"Name":"data-a","NodeID":%q,"HostPath":%q,%s}}`, nodeID, dir, hostVolumeCaps)
		resp := hostVolumeServe(t, s, http.MethodPut, "/v1/volume/host/register", body, nil)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		out := decodeHostVolumeWrite(t, resp)
		must.Eq(t, "ready", out.Volume["State"])
		must.Eq(t, "registered", out.Volume["Origin"])
		must.Eq(t, any(nodeID), out.Volume["NodeID"])
		must.Nil(t, out.Warnings)
	})
}

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
	"time"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad"
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
			"/v1/volume/host/vol-1/other":       404,
			"/v1/volume/host/vol-1/delete":      405,
			"/v1/volume/host/vol-1/deregister":  405,
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

		// The CSI routes beside the claim routes keep their codes through the
		// real mux.
		for _, tc := range []struct {
			method, path string
			code         int
		}{
			{http.MethodGet, "/v1/volumes?type=csi", 200},
			{http.MethodGet, "/v1/volumes", 200},
			{http.MethodGet, "/v1/volume/csi/none", 404},
			// The inherited CSI deregister handler answers 500 for an unknown ID.
			{http.MethodDelete, "/v1/volume/csi/none", 500},
		} {
			resp := hostVolumeServe(t, s, tc.method, tc.path, "", nil)
			must.Eq(t, tc.code, resp.Code, must.Sprint(tc.method+" "+tc.path+": "+resp.Body.String()))
		}
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

// removeFixture is one volume the remove and claim tests seed. Each case that
// removes a volume has its own, so the cases do not depend on each other.
type removeFixture struct {
	id, node string
	origin   structs.DynamicHostVolumeOrigin
	state    structs.DynamicHostVolumeState
}

func seedRemoveNode(t *testing.T, s *TestAgent, status string) string {
	t.Helper()
	store := s.Agent.Server().State()
	idx, err := store.LatestIndex()
	must.NoError(t, err)
	node := mock.Node()
	node.Status = status
	must.NoError(t, store.UpsertNode(structs.MsgTypeTestSetup, idx, node))
	return node.ID
}

func seedRemoveVolumes(t *testing.T, s *TestAgent, fixtures []removeFixture) {
	t.Helper()
	store := s.Agent.Server().State()
	idx, err := store.LatestIndex()
	must.NoError(t, err)
	vols := []*structs.DynamicHostVolume{}
	for _, f := range fixtures {
		v := &structs.DynamicHostVolume{
			ID: f.id, Name: "name-" + f.id, Namespace: "default", Type: structs.VolumeTypeHost,
			NodeID: f.node, NodePool: "default", HostPath: "/synthetic/secret/" + f.id, PluginID: "plug",
			Origin: f.origin, State: f.state,
		}
		if f.origin == structs.DynamicHostVolumeOriginRegistered {
			v.PluginID = ""
		}
		vols = append(vols, v)
	}
	must.NoError(t, store.UpsertDynamicHostVolumes(idx, vols))
}

// volumeStored reports whether the volume is in the state store.
func volumeStored(t *testing.T, s *TestAgent, id string) *structs.DynamicHostVolume {
	t.Helper()
	vol, err := s.Agent.Server().State().DynamicHostVolumeByID(nil, "default", id)
	must.NoError(t, err)
	return vol
}

// mustFixedError checks the status, the exact fixed text and that nothing a
// caller sent or the server stored leaks into the body.
func mustFixedError(t *testing.T, resp *httptest.ResponseRecorder, code int, text string) {
	t.Helper()
	must.Eq(t, code, resp.Code, must.Sprint(resp.Body.String()))
	must.Eq(t, text, resp.Body.String())
	must.StrNotContains(t, resp.Body.String(), "/synthetic")
	must.StrNotContains(t, resp.Body.String(), "secret")
}

const (
	removeUseDelete   = "volume was created by a plugin, use delete"
	removeUseDereg    = "volume was registered, use deregister"
	removeInvalidForc = "invalid force value"
)

func TestHTTP_HostVolumeRemove(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		down := seedRemoveNode(t, s, structs.NodeStatusDown)
		ready := seedRemoveNode(t, s, structs.NodeStatusReady)
		pc, reg := structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeOriginRegistered
		st := structs.DynamicHostVolumeStateReady
		seedRemoveVolumes(t, s, []removeFixture{
			{"pc-nonode-1", "", pc, structs.DynamicHostVolumeStateFailed},
			{"pc-nonode-2", "", pc, structs.DynamicHostVolumeStateFailed},
			{"reg-nonode-1", "", reg, st},
			{"pc-down-1", down, pc, st},
			{"reg-down-1", down, reg, st},
			{"pc-keep", "", pc, structs.DynamicHostVolumeStateFailed},
			{"reg-keep", "", reg, st},
			{"pc-ready", ready, pc, st},
			{"pc-offline", ready, pc, st},
			{"pc-busy", down, pc, st},
		})
		// A running allocation of another job uses pc-busy by its name.
		store := s.Agent.Server().State()
		idx, err := store.LatestIndex()
		must.NoError(t, err)
		job := mock.Job()
		job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
			"data": {Name: "data", Type: structs.VolumeTypeHost, Source: "name-pc-busy"}}
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, idx, nil, job))
		alloc := mock.Alloc()
		alloc.Job, alloc.JobID, alloc.Namespace = job, job.ID, job.Namespace
		alloc.TaskGroup, alloc.NodeID = job.TaskGroups[0].Name, down
		alloc.ClientStatus, alloc.DesiredStatus = structs.AllocClientStatusRunning, structs.AllocDesiredStatusRun
		must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, idx, []*structs.Allocation{alloc}))

		root := s.RootToken
		serve := func(method, path string) *httptest.ResponseRecorder {
			return hostVolumeServe(t, s, method, path, "", root)
		}

		t.Run("removed", func(t *testing.T) {
			for _, tc := range []struct{ path, id string }{
				{"/v1/volume/host/pc-nonode-1/delete", "pc-nonode-1"},
				{"/v1/volume/host/pc-nonode-2/delete?force=false", "pc-nonode-2"},
				{"/v1/volume/host/reg-nonode-1/deregister", "reg-nonode-1"},
				{"/v1/volume/host/pc-down-1/delete?force=true", "pc-down-1"},
				{"/v1/volume/host/reg-down-1/deregister?force=true", "reg-down-1"},
			} {
				resp := serve(http.MethodDelete, tc.path)
				must.Eq(t, 200, resp.Code, must.Sprint(tc.path+": "+resp.Body.String()))
				must.Eq(t, "", resp.Body.String(), must.Sprint(tc.path))
				must.NotEq(t, "", resp.Header().Get("X-Nomad-Index"), must.Sprint(tc.path))
				must.Nil(t, volumeStored(t, s, tc.id), must.Sprint(tc.path))
			}
		})

		t.Run("invalid force", func(t *testing.T) {
			for _, q := range []string{"force=yes", "force=", "force=1", "force=TRUE", "force=t", "force=true&force=true", "force=false&force=true"} {
				for _, action := range []string{"delete", "deregister"} {
					resp := serve(http.MethodDelete, "/v1/volume/host/pc-keep/"+action+"?"+q)
					mustFixedError(t, resp, 400, removeInvalidForc)
					must.NotNil(t, volumeStored(t, s, "pc-keep"))
				}
			}
		})

		t.Run("coded errors", func(t *testing.T) {
			for _, tc := range []struct {
				name, path string
				code       int
				text       string
			}{
				{"cross kind delete", "/v1/volume/host/reg-keep/delete", 400, removeUseDereg},
				{"cross kind deregister", "/v1/volume/host/pc-keep/deregister", 400, removeUseDelete},
				{"wildcard namespace", "/v1/volume/host/pc-keep/delete?namespace=*", 400, "a volume write needs a single namespace"},
				{"force on a ready node", "/v1/volume/host/pc-ready/delete?force=true", 400,
					`force is only for a down, disconnected or unknown node, node status is "ready"`},
				{"unknown id", "/v1/volume/host/nope/delete", 404, "volume not found"},
				{"id prefix", "/v1/volume/host/pc-kee/delete", 404, "volume not found"},
				{"create word as id", "/v1/volume/host/create/delete", 404, "volume not found"},
				{"in use", "/v1/volume/host/pc-busy/delete?force=true", 409, "volume in use"},
				{"in use without force", "/v1/volume/host/pc-busy/delete", 409, "volume in use"},
				{"no connected client", "/v1/volume/host/pc-offline/delete", 502, `client call failed, volume state is "deleting"`},
			} {
				resp := serve(http.MethodDelete, tc.path)
				mustFixedError(t, resp, tc.code, tc.text)
			}
			must.NotNil(t, volumeStored(t, s, "pc-keep"))
			must.NotNil(t, volumeStored(t, s, "reg-keep"))
			must.NotNil(t, volumeStored(t, s, "pc-busy"))
			must.Eq(t, structs.DynamicHostVolumeStateDeleting, volumeStored(t, s, "pc-offline").State)
		})

		t.Run("routing", func(t *testing.T) {
			for _, tc := range []struct {
				method, path string
				code         int
			}{
				{http.MethodGet, "/v1/volume/host/pc-keep/delete", 405},
				{http.MethodPut, "/v1/volume/host/pc-keep/delete", 405},
				{http.MethodPost, "/v1/volume/host/pc-keep/deregister", 405},
				{http.MethodGet, "/v1/volume/host/pc-keep/deregister", 405},
				// The documented sample leaves out /delete; Wander serves only
				// the path the operation table names.
				{http.MethodDelete, "/v1/volume/host/pc-keep", 405},
				{http.MethodDelete, "/v1/volume/host/pc-keep/other", 404},
				{http.MethodDelete, "/v1/volume/host/pc-keep/delete/x", 404},
			} {
				resp := serve(tc.method, tc.path)
				must.Eq(t, tc.code, resp.Code, must.Sprint(tc.method+" "+tc.path))
			}
			must.NotNil(t, volumeStored(t, s, "pc-keep"))
		})

		t.Run("acl", func(t *testing.T) {
			resp := hostVolumeServe(t, s, http.MethodDelete, "/v1/volume/host/pc-keep/delete", "", nil)
			mustFixedError(t, resp, 403, "Permission denied")
			reader := mock.CreatePolicyAndToken(t, store, 900, "dhv-reader",
				mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
			resp = hostVolumeServe(t, s, http.MethodDelete, "/v1/volume/host/pc-keep/delete", "", reader)
			mustFixedError(t, resp, 403, "Permission denied")
			must.NotNil(t, volumeStored(t, s, "pc-keep"))
		})
	})
}

// A removal refused by the version gate returns 503 with the fixed text, from
// a second server that does not report a Wander version.
func TestHTTP_HostVolumeRemove_VersionGate(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		pc := structs.DynamicHostVolumeOriginPluginCreated
		seedRemoveVolumes(t, s, []removeFixture{{"pc-nonode", "", pc, structs.DynamicHostVolumeStateFailed}})

		old, cleanup := nomad.TestServer(t, func(c *nomad.Config) {
			c.BootstrapExpect = 0
			c.WanderVersion = ""
		})
		defer cleanup()
		nomad.TestJoin(t, s.Agent.Server(), old)
		testutil.WaitForResult(func() (bool, error) {
			n := len(s.Agent.Server().Members())
			return n == 2, fmt.Errorf("members: %d", n)
		}, func(err error) { must.NoError(t, err) })

		resp := hostVolumeServe(t, s, http.MethodDelete, "/v1/volume/host/pc-nonode/delete", "", nil)
		mustFixedError(t, resp, 503, "dynamic host volumes need every server to run Wander 0.1.2 or later")
	})
}

// claimFixture is one synthetic sticky claim.
type claimFixture struct {
	ns, id, job, group, volume string
	live                       bool
}

func seedHTTPClaims(t *testing.T, s *TestAgent, specs ...claimFixture) {
	t.Helper()
	store := s.Agent.Server().State()
	idx, err := store.LatestIndex()
	must.NoError(t, err)
	must.NoError(t, store.UpsertNamespaces(idx, []*structs.Namespace{{Name: "ns-b"}}))
	var vols []*structs.DynamicHostVolume
	var claims []*structs.DynamicHostVolumeClaim
	seen := map[string]bool{}
	for i, sp := range specs {
		if key := sp.ns + "/" + sp.volume; !seen[key] {
			seen[key] = true
			vols = append(vols, &structs.DynamicHostVolume{
				ID: "vol-" + sp.volume, Name: sp.volume, Namespace: sp.ns, Type: structs.VolumeTypeHost,
				NodeID: "node-" + sp.ns, NodePool: "default", HostPath: "/synthetic/" + sp.volume, PluginID: "plug",
				Origin: structs.DynamicHostVolumeOriginPluginCreated, State: structs.DynamicHostVolumeStateReady,
			})
		}
		claims = append(claims, &structs.DynamicHostVolumeClaim{ID: sp.id, Namespace: sp.ns, JobID: sp.job,
			TaskGroupName: sp.group, VolumeName: sp.volume, VolumeID: "vol-" + sp.volume,
			AllocationID: fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1), AllocationIndex: 7})
	}
	must.NoError(t, store.UpsertDynamicHostVolumes(idx, vols))
	for _, sp := range specs {
		if !sp.live {
			continue
		}
		job := mock.Job()
		job.ID, job.Namespace = sp.job, sp.ns
		job.TaskGroups[0].Name = sp.group
		job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
			"data": {Name: "data", Type: structs.VolumeTypeHost, Source: sp.volume}}
		must.NoError(t, store.UpsertJob(structs.MsgTypeTestSetup, idx, nil, job))
		alloc := mock.Alloc()
		alloc.Job, alloc.JobID, alloc.Namespace, alloc.TaskGroup = job, job.ID, job.Namespace, sp.group
		alloc.NodeID = "node-" + sp.ns
		alloc.ClientStatus, alloc.DesiredStatus = structs.AllocClientStatusPending, structs.AllocDesiredStatusRun
		must.NoError(t, store.UpsertAllocs(structs.MsgTypeTestSetup, idx, []*structs.Allocation{alloc}))
	}
	must.NoError(t, store.UpsertDynamicHostVolumeClaims(idx, claims))
}

var standardClaims = []claimFixture{
	{ns: "default", id: "c-aaa", job: "job-a", group: "g1", volume: "data-a"},
	{ns: "default", id: "c-aab", job: "job-a", group: "g2", volume: "data-b"},
	{ns: "default", id: "c-bbb", job: "job-b", group: "g1", volume: "data-a"},
	{ns: "ns-b", id: "n-ccc", job: "job-c", group: "g1", volume: "data-c"},
}

func claimListIDs(t *testing.T, resp *httptest.ResponseRecorder) []string {
	t.Helper()
	must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
	var rows []map[string]any
	must.NoError(t, json.Unmarshal(resp.Body.Bytes(), &rows))
	ids := []string{}
	for _, r := range rows {
		ids = append(ids, r["Namespace"].(string)+"/"+r["ID"].(string))
	}
	return ids
}

func TestHTTP_HostVolumeClaimList(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPClaims(t, s, standardClaims...)
		list := func(query string) *httptest.ResponseRecorder {
			return hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/claims?"+query, "", nil)
		}

		t.Run("filters", func(t *testing.T) {
			for _, tc := range []struct {
				query string
				want  []string
			}{
				{"", []string{"default/c-aaa", "default/c-aab", "default/c-bbb"}},
				{"namespace=*", []string{"default/c-aaa", "default/c-aab", "default/c-bbb", "ns-b/n-ccc"}},
				{"namespace=ns-b", []string{"ns-b/n-ccc"}},
				{"job_id=job-a", []string{"default/c-aaa", "default/c-aab"}},
				{"task_group=g1", []string{"default/c-aaa", "default/c-bbb"}},
				{"volume_name=data-b", []string{"default/c-aab"}},
				{"job_id=job-a&task_group=g1", []string{"default/c-aaa"}},
				{"claim_id=c-aa", []string{"default/c-aaa", "default/c-aab"}},
				{"claim_id=c-aa&prefix=c-aa", []string{"default/c-aaa", "default/c-aab"}},
				{"namespace=*&claim_id=n-", []string{"ns-b/n-ccc"}},
				{"claim_id=zzz", []string{}},
			} {
				must.Eq(t, tc.want, claimListIDs(t, list(tc.query)), must.Sprint(tc.query))
			}
		})

		t.Run("empty list is an array", func(t *testing.T) {
			resp := list("job_id=nobody")
			must.Eq(t, 200, resp.Code)
			must.Eq(t, "[]", strings.TrimSpace(resp.Body.String()))
		})

		t.Run("public keys", func(t *testing.T) {
			resp := list("claim_id=c-aaa")
			var rows []map[string]any
			must.NoError(t, json.Unmarshal(resp.Body.Bytes(), &rows))
			must.Len(t, 1, rows)
			keys := slices.Sorted(maps.Keys(rows[0]))
			must.Eq(t, []string{"AllocID", "CreateIndex", "ID", "JobID", "ModifyIndex", "Namespace",
				"TaskGroupName", "VolumeID", "VolumeName"}, keys)
			must.Eq(t, "00000000-0000-0000-0000-000000000001", rows[0]["AllocID"])
			must.Eq(t, "vol-data-a", rows[0]["VolumeID"])
			must.NotEq(t, "", resp.Header().Get("X-Nomad-Index"))
			must.NotEq(t, "", resp.Header().Get("X-Nomad-KnownLeader"))
		})

		t.Run("paging", func(t *testing.T) {
			var got []string
			token := ""
			for page := range 4 {
				q := "namespace=*&per_page=1"
				if token != "" {
					q += "&next_token=" + url.QueryEscape(token)
				}
				resp := list(q)
				ids := claimListIDs(t, resp)
				must.Len(t, 1, ids, must.Sprintf("page %d", page))
				got = append(got, ids...)
				token = resp.Header().Get("X-Nomad-NextToken")
				if page < 3 {
					must.NotEq(t, "", token, must.Sprintf("page %d token", page))
				} else {
					must.Eq(t, "", token)
				}
			}
			must.Eq(t, []string{"default/c-aaa", "default/c-aab", "default/c-bbb", "ns-b/n-ccc"}, got)
		})

		t.Run("refused queries", func(t *testing.T) {
			mustFixedError(t, list("claim_id=c-a&prefix=c-b"), 400, "claim_id and prefix differ")
			mustFixedError(t, list("filter="+url.QueryEscape(`JobID == "job-a"`)), 400, "filter is not supported for claims")
			must.Eq(t, 405, hostVolumeServe(t, s, http.MethodPost, "/v1/volumes/claims", "", nil).Code)
			must.Eq(t, 405, hostVolumeServe(t, s, http.MethodDelete, "/v1/volumes/claims", "", nil).Code)
			must.Eq(t, 404, hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/claims/", "", nil).Code)
		})
	})
}

func TestHTTP_HostVolumeClaimList_Blocking(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPClaims(t, s, standardClaims...)
		first := hostVolumeServe(t, s, http.MethodGet, "/v1/volumes/claims", "", nil)
		index, err := strconv.ParseUint(first.Header().Get("X-Nomad-Index"), 10, 64)
		must.NoError(t, err)

		type result struct{ resp *httptest.ResponseRecorder }
		done := make(chan result, 1)
		go func() {
			done <- result{hostVolumeServe(t, s, http.MethodGet,
				fmt.Sprintf("/v1/volumes/claims?index=%d&wait=20s", index), "", nil)}
		}()
		select {
		case <-done:
			t.Fatal("a blocking query returned before the claims changed")
		case <-time.After(300 * time.Millisecond):
		}

		del := hostVolumeServe(t, s, http.MethodDelete, "/v1/volumes/claim/c-bbb", "", nil)
		must.Eq(t, 200, del.Code, must.Sprint(del.Body.String()))

		select {
		case r := <-done:
			must.Eq(t, []string{"default/c-aaa", "default/c-aab"}, claimListIDs(t, r.resp))
			got, err := strconv.ParseUint(r.resp.Header().Get("X-Nomad-Index"), 10, 64)
			must.NoError(t, err)
			must.Greater(t, index, got)
		case <-time.After(10 * time.Second):
			t.Fatal("a blocking query did not wake on a claim delete")
		}
	})
}

func TestHTTP_HostVolumeClaimDelete(t *testing.T) {
	ci.Parallel(t)
	httpACLTest(t, nil, func(s *TestAgent) {
		held := claimFixture{ns: "default", id: "c-held", job: "job-h", group: "g", volume: "data-h", live: true}
		seedHTTPClaims(t, s, append(slices.Clone(standardClaims), held)...)
		store := s.Agent.Server().State()
		stored := func(ns, id string) bool {
			c, err := store.DynamicHostVolumeClaimByID(nil, ns, id)
			must.NoError(t, err)
			return c != nil
		}
		root := s.RootToken
		del := func(path string, tok *structs.ACLToken) *httptest.ResponseRecorder {
			return hostVolumeServe(t, s, http.MethodDelete, path, "", tok)
		}

		reader := mock.CreatePolicyAndToken(t, store, 900, "claim-reader",
			mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
		for _, tc := range []struct {
			name, path string
			tok        *structs.ACLToken
			code       int
			text       string
		}{
			{"no token", "/v1/volumes/claim/c-aaa", nil, 403, "Permission denied"},
			{"read only", "/v1/volumes/claim/c-aaa", reader, 403, "Permission denied"},
			{"unknown", "/v1/volumes/claim/nope", root, 404, "claim not found"},
			{"other namespace", "/v1/volumes/claim/n-ccc", root, 404, "claim not found"},
			{"held", "/v1/volumes/claim/c-held", root, 409, "claim in use"},
			{"wildcard", "/v1/volumes/claim/c-aaa?namespace=*", root, 400, "a claim delete needs a single namespace"},
		} {
			mustFixedError(t, del(tc.path, tc.tok), tc.code, tc.text)
			must.True(t, stored("default", "c-aaa"), must.Sprint(tc.name))
			must.True(t, stored("default", "c-held"), must.Sprint(tc.name))
		}

		resp := del("/v1/volumes/claim/c-aaa", root)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		must.Eq(t, "", resp.Body.String())
		must.NotEq(t, "", resp.Header().Get("X-Nomad-Index"))
		must.False(t, stored("default", "c-aaa"))
		resp = del("/v1/volumes/claim/n-ccc?namespace=ns-b", root)
		must.Eq(t, 200, resp.Code, must.Sprint(resp.Body.String()))
		must.False(t, stored("ns-b", "n-ccc"))

		for _, tc := range []struct {
			method, path string
			code         int
		}{
			{http.MethodGet, "/v1/volumes/claim/c-aab", 405},
			{http.MethodPut, "/v1/volumes/claim/c-aab", 405},
			{http.MethodPost, "/v1/volumes/claim/c-aab", 405},
			{http.MethodDelete, "/v1/volumes/claim/", 404},
			{http.MethodDelete, "/v1/volumes/claim/c-aab/x", 404},
			// Only the plural path is served: the official binary answered 404
			// for the singular sample path.
			{http.MethodDelete, "/v1/volume/claim/c-aab", 404},
		} {
			must.Eq(t, tc.code, hostVolumeServe(t, s, tc.method, tc.path, "", root).Code, must.Sprint(tc.method+" "+tc.path))
		}
		must.True(t, stored("default", "c-aab"))
	})
}

func TestHTTP_HostVolumeClaimDelete_VersionGate(t *testing.T) {
	ci.Parallel(t)
	httpTest(t, nil, func(s *TestAgent) {
		seedHTTPClaims(t, s, standardClaims...)
		old, cleanup := nomad.TestServer(t, func(c *nomad.Config) {
			c.BootstrapExpect = 0
			c.WanderVersion = ""
		})
		defer cleanup()
		nomad.TestJoin(t, s.Agent.Server(), old)
		testutil.WaitForResult(func() (bool, error) {
			n := len(s.Agent.Server().Members())
			return n == 2, fmt.Errorf("members: %d", n)
		}, func(err error) { must.NoError(t, err) })

		resp := hostVolumeServe(t, s, http.MethodDelete, "/v1/volumes/claim/c-aaa", "", nil)
		mustFixedError(t, resp, 503, "dynamic host volumes need every server to run Wander 0.1.2 or later")
	})
}

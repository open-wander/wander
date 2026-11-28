// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/open-wander/wander/api/internal/testutil"
	"github.com/shoenig/test/must"
)

// hostVolumeRecorded is one request as the test server saw it.
type hostVolumeRecorded struct {
	method string
	path   string
	query  url.Values
	body   []byte
}

// newHostVolumeServer starts a server that records every request and answers
// each with reply and the index headers of a real server. The client it
// returns has no region or namespace of its own, so only what a method sends
// shows up in the recording.
func newHostVolumeServer(t *testing.T, reply string) (*HostVolumes, func() []hostVolumeRecorded) {
	t.Helper()
	var mu sync.Mutex
	var seen []hostVolumeRecorded

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read", http.StatusInternalServerError)
			return
		}
		mu.Lock()
		seen = append(seen, hostVolumeRecorded{
			method: r.Method, path: r.URL.EscapedPath(), query: r.URL.Query(), body: body,
		})
		mu.Unlock()

		// A request takes measurable time, so RequestTime can be asserted.
		time.Sleep(2 * time.Millisecond)
		w.Header().Set("X-Nomad-Index", "42")
		w.Header().Set("X-Nomad-KnownLeader", "true")
		w.Header().Set("X-Nomad-LastContact", "7")
		w.Header().Set("X-Nomad-NextToken", "tok-2")
		if reply != "" {
			if _, err := w.Write([]byte(reply)); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	conf := DefaultConfig()
	conf.Address = srv.URL
	conf.Namespace = ""
	conf.Region = ""
	c, err := NewClient(conf)
	must.NoError(t, err)

	return c.HostVolumes(), func() []hostVolumeRecorded {
		mu.Lock()
		defer mu.Unlock()
		return append([]hostVolumeRecorded(nil), seen...)
	}
}

// hostVolumeBody is the JSON of a HostVolume with every field at its zero
// value except the overrides. Listing the fields here makes a dropped or
// renamed field of the api type fail the contract tests.
func hostVolumeBody(overrides map[string]any) map[string]any {
	m := map[string]any{
		"ID": "", "Name": "", "Namespace": "", "Type": "", "NodeID": "", "NodePool": "",
		"HostPath": "", "PluginID": "", "CapacityBytes": 0.0, "RequestedCapacityMinBytes": 0.0,
		"RequestedCapacityMaxBytes": 0.0, "RequestedCapabilities": nil, "Constraints": nil,
		"Parameters": nil, "State": "", "StatusDescription": "", "Origin": "",
		"Attachments": nil, "CreateTime": 0.0, "ModifyTime": 0.0, "CreateIndex": 0.0,
		"ModifyIndex": 0.0,
	}
	for k, v := range overrides {
		m[k] = v
	}
	return m
}

const hostVolumeReplyJSON = `{"Volume":{"ID":"vol-1","Name":"data","Namespace":"ns-a","State":"ready","CapacityBytes":1024,` +
	`"Attachments":[{"AllocationID":"alloc-1","NodeID":"node-a","AttachmentMode":"file-system","AccessMode":"single-node-writer","ReadOnly":true}]},` +
	`"Warnings":"synthetic warning"}`

func TestHostVolumes_Contract(t *testing.T) {
	testutil.Parallel(t)

	listReply := `[{"ID":"vol-1","Name":"data","Namespace":"ns-a","PluginID":"plug","NodeID":"node-a","NodePool":"pool",` +
		`"CapacityBytes":1024,"State":"ready","CreateIndex":3,"ModifyIndex":4,"CreateTime":5,"ModifyTime":6}]`
	claimReply := `[{"AllocID":"alloc-1","CreateIndex":3,"ID":"claim-1","JobID":"job-a","ModifyIndex":4,"Namespace":"ns-a",` +
		`"TaskGroupName":"grp","VolumeID":"vol-1","VolumeName":"data"}]`
	orphanReply := `[{"NodeID":"node-a","VolumeID":"vol-9","Name":"data","Namespace":"ns-a"}]`
	infoReply := `{"ID":"vol-1","Name":"data","Namespace":"ns-a","Type":"host","NodeID":"node-a","NodePool":"pool",` +
		`"HostPath":"/synthetic/vol-1","PluginID":"plug","CapacityBytes":1024,"RequestedCapacityMinBytes":1,` +
		`"RequestedCapacityMaxBytes":2,"RequestedCapabilities":[{"AttachmentMode":"file-system","AccessMode":"single-node-writer"}],` +
		`"Constraints":[{"LTarget":"${attr.a}","RTarget":"b","Operand":"="}],"Parameters":{"k":"v"},"State":"ready",` +
		`"StatusDescription":"fine","Origin":"registered","Attachments":[{"AllocationID":"alloc-1","NodeID":"node-a",` +
		`"AttachmentMode":"file-system","AccessMode":"single-node-writer","ReadOnly":true}],` +
		`"CreateTime":5,"ModifyTime":6,"CreateIndex":3,"ModifyIndex":4}`

	// queryMeta and writeMeta check the metadata every reply carries.
	queryMeta := func(t *testing.T, qm *QueryMeta) {
		t.Helper()
		must.NotNil(t, qm)
		must.Eq(t, 42, qm.LastIndex)
		must.Eq(t, 7*time.Millisecond, qm.LastContact)
		must.True(t, qm.KnownLeader)
		must.Eq(t, "tok-2", qm.NextToken)
		must.Positive(t, qm.RequestTime)
	}
	writeMeta := func(t *testing.T, wm *WriteMeta) {
		t.Helper()
		must.NotNil(t, wm)
		must.Eq(t, 42, wm.LastIndex)
		must.Positive(t, wm.RequestTime)
	}

	volume := &HostVolume{
		ID: "vol-1", Name: "data", Namespace: "ns-vol", NodeID: "node-a",
		RequestedCapabilities: []*HostVolumeCapability{{
			AttachmentMode: HostVolumeAttachmentModeFilesystem, AccessMode: HostVolumeAccessModeSingleNodeWriter,
		}},
	}
	volumeWire := hostVolumeBody(map[string]any{
		"ID": "vol-1", "Name": "data", "Namespace": "ns-vol", "NodeID": "node-a",
		"RequestedCapabilities": []any{map[string]any{"AttachmentMode": "file-system", "AccessMode": "single-node-writer"}},
	})
	checkWrite := func(t *testing.T, resp *HostVolumeWriteResponse) {
		t.Helper()
		must.NotNil(t, resp)
		must.NotNil(t, resp.Volume)
		must.Eq(t, "vol-1", resp.Volume.ID)
		must.Eq(t, HostVolumeStateReady, resp.Volume.State)
		must.Eq(t, "synthetic warning", resp.Warnings)
		must.Len(t, 1, resp.Volume.Attachments)
		must.Eq(t, "alloc-1", resp.Volume.Attachments[0].AllocationID)
		must.True(t, resp.Volume.Attachments[0].ReadOnly)
	}

	tests := []struct {
		name     string
		reply    string
		run      func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta)
		method   string
		path     string
		query    url.Values
		wantBody map[string]any
	}{
		{
			name: "list without filters", reply: listReply, method: "GET", path: "/v1/volumes",
			query: url.Values{"type": {"host"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				got, qm, err := hv.List(nil, nil)
				must.NoError(t, err)
				must.Len(t, 1, got)
				must.Eq(t, &HostVolumeStub{
					ID: "vol-1", Name: "data", Namespace: "ns-a", PluginID: "plug", NodeID: "node-a", NodePool: "pool",
					CapacityBytes: 1024, State: HostVolumeStateReady, CreateIndex: 3, ModifyIndex: 4, CreateTime: 5, ModifyTime: 6,
				}, got[0])
				return qm, nil
			},
		},
		{
			name: "list with filters and paging", reply: listReply, method: "GET", path: "/v1/volumes",
			query: url.Values{
				"type": {"host"}, "node_id": {"node-a"}, "node_pool": {"pool"}, "namespace": {"*"},
				"prefix": {"vol"}, "per_page": {"2"}, "next_token": {"tok-1"},
			},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				_, qm, err := hv.List(&HostVolumeListRequest{NodeID: "node-a", NodePool: "pool"},
					&QueryOptions{Namespace: "*", Prefix: "vol", PerPage: 2, NextToken: "tok-1"})
				must.NoError(t, err)
				return qm, nil
			},
		},
		{
			name: "info", reply: infoReply, method: "GET", path: "/v1/volume/host/vol-1", query: url.Values{},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				got, qm, err := hv.Info("vol-1", nil)
				must.NoError(t, err)
				must.Eq(t, "/synthetic/vol-1", got.HostPath)
				must.Eq(t, int64(2), got.RequestedCapacityMaxBytes)
				must.Eq(t, HostVolumeOriginRegistered, got.Origin)
				must.Eq(t, []*Constraint{{LTarget: "${attr.a}", RTarget: "b", Operand: "="}}, got.Constraints)
				must.Eq(t, map[string]string{"k": "v"}, got.Parameters)
				must.Eq(t, uint64(4), got.ModifyIndex)
				return qm, nil
			},
		},
		{
			name: "info escapes the ID", reply: infoReply, method: "GET", path: "/v1/volume/host/a%2Fb", query: url.Values{},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				_, qm, err := hv.Info("a/b", nil)
				must.NoError(t, err)
				return qm, nil
			},
		},
		{
			name: "create", reply: hostVolumeReplyJSON, method: "PUT", path: "/v1/volume/host/create", query: url.Values{},
			wantBody: map[string]any{"Volume": volumeWire, "PolicyOverride": false},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				got, wm, err := hv.Create(&HostVolumeCreateRequest{Volume: volume}, nil)
				must.NoError(t, err)
				checkWrite(t, got)
				return nil, wm
			},
		},
		{
			name: "register with policy override", reply: hostVolumeReplyJSON, method: "PUT", path: "/v1/volume/host/register",
			query:    url.Values{},
			wantBody: map[string]any{"Volume": volumeWire, "PolicyOverride": true},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				got, wm, err := hv.Register(&HostVolumeRegisterRequest{Volume: volume, PolicyOverride: true}, nil)
				must.NoError(t, err)
				checkWrite(t, got)
				return nil, wm
			},
		},
		{
			// The namespace of the options goes in the query and the namespace
			// of the volume stays in the body. Neither overrides the other; the
			// server refuses a pair that differs.
			name: "create sends both namespaces", reply: hostVolumeReplyJSON, method: "PUT", path: "/v1/volume/host/create",
			query:    url.Values{"namespace": {"ns-opt"}},
			wantBody: map[string]any{"Volume": volumeWire, "PolicyOverride": false},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				_, wm, err := hv.Create(&HostVolumeCreateRequest{Volume: volume}, &WriteOptions{Namespace: "ns-opt"})
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "delete", method: "DELETE", path: "/v1/volume/host/vol-1/delete", query: url.Values{},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.Delete("vol-1", false, nil)
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "delete with force", method: "DELETE", path: "/v1/volume/host/vol-1/delete",
			query: url.Values{"force": {"true"}, "namespace": {"ns-a"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.Delete("vol-1", true, &WriteOptions{Namespace: "ns-a"})
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "deregister", method: "DELETE", path: "/v1/volume/host/vol-1/deregister", query: url.Values{},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.Deregister("vol-1", false, nil)
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "deregister with force", method: "DELETE", path: "/v1/volume/host/vol-1/deregister",
			query: url.Values{"force": {"true"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.Deregister("vol-1", true, nil)
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "claims without filters", reply: claimReply, method: "GET", path: "/v1/volumes/claims", query: url.Values{},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				got, qm, err := hv.ListClaims(nil, nil)
				must.NoError(t, err)
				must.Eq(t, []*HostVolumeClaim{{
					AllocID: "alloc-1", CreateIndex: 3, ID: "claim-1", JobID: "job-a", ModifyIndex: 4,
					Namespace: "ns-a", TaskGroupName: "grp", VolumeID: "vol-1", VolumeName: "data",
				}}, got)
				return qm, nil
			},
		},
		{
			name: "claims with filters and paging", reply: claimReply, method: "GET", path: "/v1/volumes/claims",
			query: url.Values{
				"claim_id": {"claim"}, "job_id": {"job-a"}, "task_group": {"grp"}, "volume_name": {"data"},
				"namespace": {"*"}, "per_page": {"5"}, "next_token": {"tok-1"},
			},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				_, qm, err := hv.ListClaims(
					&HostVolumeClaimListRequest{ClaimID: "claim", JobID: "job-a", TaskGroup: "grp", VolumeName: "data"},
					&QueryOptions{Namespace: "*", PerPage: 5, NextToken: "tok-1"})
				must.NoError(t, err)
				return qm, nil
			},
		},
		{
			name: "claim delete", method: "DELETE", path: "/v1/volumes/claim/claim-1", query: url.Values{"namespace": {"ns-a"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.DeleteClaim("claim-1", &WriteOptions{Namespace: "ns-a"})
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "orphan list", reply: orphanReply, method: "GET", path: "/v1/volumes/host/orphans", query: url.Values{},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				got, qm, err := hv.ListOrphans("", nil)
				must.NoError(t, err)
				must.Eq(t, []*HostVolumeOrphan{{NodeID: "node-a", VolumeID: "vol-9", Name: "data", Namespace: "ns-a"}}, got)
				return qm, nil
			},
		},
		{
			name: "orphan list for a node in every namespace", reply: orphanReply, method: "GET", path: "/v1/volumes/host/orphans",
			query: url.Values{"node_id": {"node-a"}, "namespace": {"*"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				_, qm, err := hv.ListOrphans("node-a", &QueryOptions{Namespace: "*"})
				must.NoError(t, err)
				return qm, nil
			},
		},
		{
			name: "orphan remove in delete mode", method: "DELETE", path: "/v1/volume/host/orphan/node-a/vol-9",
			query: url.Values{"mode": {"delete"}, "namespace": {"ns-a"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.RemoveOrphan("node-a", "vol-9", HostVolumeOrphanModeDelete, &WriteOptions{Namespace: "ns-a"})
				must.NoError(t, err)
				return nil, wm
			},
		},
		{
			name: "orphan remove in forget mode", method: "DELETE", path: "/v1/volume/host/orphan/node-a/vol-9",
			query: url.Values{"mode": {"forget"}},
			run: func(t *testing.T, hv *HostVolumes) (*QueryMeta, *WriteMeta) {
				wm, err := hv.RemoveOrphan("node-a", "vol-9", HostVolumeOrphanModeForget, nil)
				must.NoError(t, err)
				return nil, wm
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hv, recorded := newHostVolumeServer(t, tc.reply)
			qm, wm := tc.run(t, hv)

			reqs := recorded()
			must.Len(t, 1, reqs)
			got := reqs[0]
			must.Eq(t, tc.method, got.method)
			must.Eq(t, tc.path, got.path)
			must.Eq(t, tc.query, got.query)
			if tc.wantBody == nil {
				must.Eq(t, "", string(got.body))
			} else {
				var body map[string]any
				must.NoError(t, json.Unmarshal(got.body, &body))
				must.Eq(t, tc.wantBody, body)
			}

			if qm != nil {
				queryMeta(t, qm)
			}
			if wm != nil {
				writeMeta(t, wm)
			}
			must.True(t, qm != nil || wm != nil)
		})
	}
}

// TestHostVolumes_Refusals checks the rules the client applies before it
// sends anything: the error text is fixed and no request is made.
func TestHostVolumes_Refusals(t *testing.T) {
	testutil.Parallel(t)

	tests := []struct {
		name string
		want string
		run  func(hv *HostVolumes) error
	}{
		{"info empty ID", "host volume ID is required", func(hv *HostVolumes) error {
			_, _, err := hv.Info("", nil)
			return err
		}},
		{"delete empty ID", "host volume ID is required", func(hv *HostVolumes) error {
			_, err := hv.Delete("", true, nil)
			return err
		}},
		{"deregister empty ID", "host volume ID is required", func(hv *HostVolumes) error {
			_, err := hv.Deregister("", false, nil)
			return err
		}},
		{"claim delete empty ID", "host volume claim ID is required", func(hv *HostVolumes) error {
			_, err := hv.DeleteClaim("", nil)
			return err
		}},
		{"create nil request", "host volume is required", func(hv *HostVolumes) error {
			_, _, err := hv.Create(nil, nil)
			return err
		}},
		{"create nil volume", "host volume is required", func(hv *HostVolumes) error {
			_, _, err := hv.Create(&HostVolumeCreateRequest{}, nil)
			return err
		}},
		{"register nil request", "host volume is required", func(hv *HostVolumes) error {
			_, _, err := hv.Register(nil, nil)
			return err
		}},
		{"register nil volume", "host volume is required", func(hv *HostVolumes) error {
			_, _, err := hv.Register(&HostVolumeRegisterRequest{}, nil)
			return err
		}},
		{"orphan remove empty node", "node ID is required", func(hv *HostVolumes) error {
			_, err := hv.RemoveOrphan("", "vol-9", HostVolumeOrphanModeDelete, nil)
			return err
		}},
		{"orphan remove empty volume", "host volume ID is required", func(hv *HostVolumes) error {
			_, err := hv.RemoveOrphan("node-a", "", HostVolumeOrphanModeForget, nil)
			return err
		}},
		{"orphan remove no mode", "orphan mode must be delete or forget", func(hv *HostVolumes) error {
			_, err := hv.RemoveOrphan("node-a", "vol-9", "", nil)
			return err
		}},
		{"orphan remove misspelt mode", "orphan mode must be delete or forget", func(hv *HostVolumes) error {
			_, err := hv.RemoveOrphan("node-a", "vol-9", "purge", nil)
			return err
		}},
		{"orphan remove wrong case mode", "orphan mode must be delete or forget", func(hv *HostVolumes) error {
			_, err := hv.RemoveOrphan("node-a", "vol-9", "Delete", nil)
			return err
		}},
		{"claims with a filter expression", "filter is not supported for claims", func(hv *HostVolumes) error {
			_, _, err := hv.ListClaims(nil, &QueryOptions{Filter: "JobID == \"x\""})
			return err
		}},
		{"claims with differing prefix and claim ID", "prefix and claim ID differ", func(hv *HostVolumes) error {
			_, _, err := hv.ListClaims(&HostVolumeClaimListRequest{ClaimID: "a"}, &QueryOptions{Prefix: "b"})
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hv, recorded := newHostVolumeServer(t, "")
			err := tc.run(hv)
			must.Error(t, err)
			must.EqError(t, err, tc.want)
			must.Len(t, 0, recorded())
		})
	}
}

// TestHostVolumes_ClaimPrefixSameValue shows that a prefix equal to the claim
// ID is not a conflict.
func TestHostVolumes_ClaimPrefixSameValue(t *testing.T) {
	testutil.Parallel(t)

	hv, recorded := newHostVolumeServer(t, "[]")
	_, _, err := hv.ListClaims(&HostVolumeClaimListRequest{ClaimID: "a"}, &QueryOptions{Prefix: "a"})
	must.NoError(t, err)
	reqs := recorded()
	must.Len(t, 1, reqs)
	must.Eq(t, url.Values{"claim_id": {"a"}, "prefix": {"a"}}, reqs[0].query)
}

// TestHostVolumes_CallerOptionsUntouched checks that the methods never write
// into the Params of the caller's options.
func TestHostVolumes_CallerOptionsUntouched(t *testing.T) {
	testutil.Parallel(t)

	hv, _ := newHostVolumeServer(t, "[]")
	q := &QueryOptions{Namespace: "ns-a"}
	_, _, err := hv.List(&HostVolumeListRequest{NodeID: "node-a"}, q)
	must.NoError(t, err)
	_, _, err = hv.ListClaims(&HostVolumeClaimListRequest{JobID: "job-a"}, q)
	must.NoError(t, err)
	_, _, err = hv.ListOrphans("node-a", q)
	must.NoError(t, err)
	must.Nil(t, q.Params)

	w := &WriteOptions{Namespace: "ns-a"}
	_, err = hv.RemoveOrphan("node-a", "vol-9", HostVolumeOrphanModeForget, w)
	must.NoError(t, err)
	_, err = hv.Delete("vol-1", true, w)
	must.NoError(t, err)
	must.Eq(t, &WriteOptions{Namespace: "ns-a"}, w)
}

// TestHostVolumes_ServerError checks that a refusal from the server comes back
// as an error with its status.
func TestHostVolumes_ServerError(t *testing.T) {
	testutil.Parallel(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "volume not found", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	conf := DefaultConfig()
	conf.Address = srv.URL
	c, err := NewClient(conf)
	must.NoError(t, err)

	_, _, err = c.HostVolumes().Info("vol-1", nil)
	must.Error(t, err)
	must.StrContains(t, err.Error(), "404")
	must.StrContains(t, err.Error(), "volume not found")
}

// TestHostVolumeWarningsNull checks that a reply without warnings, which the
// server writes as null, decodes to an empty string.
func TestHostVolumeWarningsNull(t *testing.T) {
	testutil.Parallel(t)

	hv, _ := newHostVolumeServer(t, `{"Volume":{"ID":"vol-1"},"Warnings":null}`)
	got, _, err := hv.Register(&HostVolumeRegisterRequest{Volume: &HostVolume{Name: "data"}}, nil)
	must.NoError(t, err)
	must.Eq(t, "", got.Warnings)
	must.Eq(t, "vol-1", got.Volume.ID)
}

// TestHostVolumeInfo_NodeFields checks that the node entry of a dynamic volume
// carries its ID and namespace and that a static entry leaves them empty.
func TestHostVolumeInfo_NodeFields(t *testing.T) {
	testutil.Parallel(t)

	var got map[string]*HostVolumeInfo
	must.NoError(t, json.Unmarshal([]byte(
		`{"dyn":{"Path":"/synthetic/dyn","ReadOnly":false,"ID":"vol-1","Namespace":"ns-a"},`+
			`"static":{"Path":"/synthetic/static","ReadOnly":true}}`), &got))
	must.Eq(t, "vol-1", got["dyn"].ID)
	must.Eq(t, "ns-a", got["dyn"].Namespace)
	must.Eq(t, "", got["static"].ID)
	must.Eq(t, "", got["static"].Namespace)
}

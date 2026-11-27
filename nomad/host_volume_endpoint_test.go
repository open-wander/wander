// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

func testHostVolume(ns, id, node, pool string) *structs.DynamicHostVolume {
	return &structs.DynamicHostVolume{
		ID: id, Name: "name-" + id, Namespace: ns, Type: structs.VolumeTypeHost,
		NodeID: node, NodePool: pool, HostPath: "/synthetic/" + id,
		PluginID: "plug", State: structs.DynamicHostVolumeStateReady,
	}
}

func upsertHostVolumeNamespaces(t *testing.T, s *Server, index uint64, names ...string) {
	t.Helper()
	nss := make([]*structs.Namespace, 0, len(names))
	for _, n := range names {
		nss = append(nss, &structs.Namespace{Name: n})
	}
	must.NoError(t, s.State().UpsertNamespaces(index, nss))
}

func hostVolumeIDs(vols []*structs.DynamicHostVolumeStub) []string {
	ids := []string{}
	for _, v := range vols {
		ids = append(ids, v.Namespace+"/"+v.ID)
	}
	return ids
}

func TestHostVolumeEndpoint_ACL(t *testing.T) {
	ci.Parallel(t)
	s, root, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	ns := structs.DefaultNamespace
	must.NoError(t, s.State().UpsertDynamicHostVolumes(100,
		[]*structs.DynamicHostVolume{testHostVolume(ns, "vol-1", "node-1", "default")}))

	capPolicy := func(caps ...string) string { return mock.NamespacePolicy(ns, "", caps) }
	cases := []struct {
		name   string
		policy string // empty: use no token or special token
		token  string // "none" or "bogus" for special cases
		allow  bool
	}{
		{name: "no token", token: "none"},
		{name: "unknown token", token: "bogus"},
		{name: "no host volume capability", policy: capPolicy(acl.NamespaceCapabilityReadJob)},
		{name: "read", policy: capPolicy(acl.NamespaceCapabilityHostVolumeRead), allow: true},
		{name: "create implies read", policy: capPolicy(acl.NamespaceCapabilityHostVolumeCreate), allow: true},
		{name: "register implies read", policy: capPolicy(acl.NamespaceCapabilityHostVolumeRegister), allow: true},
		{name: "write implies read", policy: capPolicy(acl.NamespaceCapabilityHostVolumeWrite), allow: true},
		{name: "delete does not imply read", policy: capPolicy(acl.NamespaceCapabilityHostVolumeDelete)},
		{name: "namespace read shorthand", policy: mock.NamespacePolicy(ns, acl.PolicyRead, nil), allow: true},
		{name: "namespace write shorthand", policy: mock.NamespacePolicy(ns, acl.PolicyWrite, nil), allow: true},
		{name: "deny", policy: mock.NamespacePolicy(ns, acl.PolicyDeny, []string{acl.NamespaceCapabilityHostVolumeRead})},
		{name: "management", token: "root", allow: true},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var secret string
			switch tc.token {
			case "none":
			case "bogus":
				secret = "00000000-0000-0000-0000-000000000000"
			case "root":
				secret = root.SecretID
			default:
				tok := mock.CreatePolicyAndToken(t, s.State(), uint64(1000+i*2), fmt.Sprintf("p%d", i), tc.policy)
				secret = tok.SecretID
			}
			qo := structs.QueryOptions{Region: "global", Namespace: ns, AuthToken: secret}

			var list structs.HostVolumeListResponse
			err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", &structs.HostVolumeListRequest{QueryOptions: qo}, &list)
			var get structs.HostVolumeGetResponse
			gerr := msgpackrpc.CallWithCodec(codec, "HostVolume.Get", &structs.HostVolumeGetRequest{ID: "vol-1", QueryOptions: qo}, &get)
			if tc.allow {
				must.NoError(t, err)
				must.Eq(t, []string{ns + "/vol-1"}, hostVolumeIDs(list.Volumes))
				must.NoError(t, gerr)
				must.NotNil(t, get.Volume)
				must.Eq(t, "/synthetic/vol-1", get.Volume.HostPath)
				return
			}
			must.EqError(t, err, structs.ErrPermissionDenied.Error())
			must.EqError(t, gerr, structs.ErrPermissionDenied.Error())
		})
	}
}

func TestHostVolumeEndpoint_ACLDisabled(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	ns := structs.DefaultNamespace
	must.NoError(t, s.State().UpsertDynamicHostVolumes(100,
		[]*structs.DynamicHostVolume{testHostVolume(ns, "vol-1", "node-1", "default")}))
	qo := structs.QueryOptions{Region: "global", Namespace: ns}

	var list structs.HostVolumeListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", &structs.HostVolumeListRequest{QueryOptions: qo}, &list))
	must.Len(t, 1, list.Volumes)
	must.Eq(t, uint64(100), list.Index)

	var get structs.HostVolumeGetResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.Get", &structs.HostVolumeGetRequest{ID: "vol-1", QueryOptions: qo}, &get))
	must.NotNil(t, get.Volume)
	must.Eq(t, uint64(100), get.Index)
}

func TestHostVolumeEndpoint_List_Wildcard(t *testing.T) {
	ci.Parallel(t)
	s, root, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	upsertHostVolumeNamespaces(t, s, 50, "ns-a", "ns-b")
	must.NoError(t, s.State().UpsertDynamicHostVolumes(100, []*structs.DynamicHostVolume{
		testHostVolume("ns-a", "vol-1", "node-1", "default"),
		testHostVolume("ns-a", "vol-2", "node-1", "default"),
		testHostVolume("ns-a", "vol-3", "node-2", "default"),
		testHostVolume("ns-b", "vol-1", "node-1", "default"),
		testHostVolume("ns-b", "vol-secret", "node-1", "default"),
	}))

	tok := mock.CreatePolicyAndToken(t, s.State(), 200, "reader-a",
		mock.NamespacePolicy("ns-a", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
	list := func(req *structs.HostVolumeListRequest) (*structs.HostVolumeListResponse, error) {
		req.Region = "global"
		req.Namespace = structs.AllNamespacesSentinel
		req.AuthToken = tok.SecretID
		var resp structs.HostVolumeListResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp)
		return &resp, err
	}

	t.Run("only readable namespace", func(t *testing.T) {
		resp, err := list(&structs.HostVolumeListRequest{})
		must.NoError(t, err)
		must.Eq(t, []string{"ns-a/vol-1", "ns-a/vol-2", "ns-a/vol-3"}, hostVolumeIDs(resp.Volumes))
	})

	t.Run("prefix", func(t *testing.T) {
		req := &structs.HostVolumeListRequest{}
		req.Prefix = "vol-secret"
		resp, err := list(req)
		must.NoError(t, err)
		must.Len(t, 0, resp.Volumes)
	})

	t.Run("filter expression cannot reach unreadable namespace", func(t *testing.T) {
		req := &structs.HostVolumeListRequest{}
		req.Filter = `Namespace == "ns-b"`
		resp, err := list(req)
		must.NoError(t, err)
		must.Len(t, 0, resp.Volumes)
	})

	t.Run("pagination tokens never point at unreadable namespace", func(t *testing.T) {
		var seen []string
		token := ""
		for range 10 {
			req := &structs.HostVolumeListRequest{}
			req.PerPage = 1
			req.NextToken = token
			resp, err := list(req)
			must.NoError(t, err)
			seen = append(seen, hostVolumeIDs(resp.Volumes)...)
			if resp.NextToken == "" {
				break
			}
			must.StrContains(t, resp.NextToken, "ns-a")
			must.StrNotContains(t, resp.NextToken, "ns-b")
			token = resp.NextToken
		}
		must.Eq(t, []string{"ns-a/vol-1", "ns-a/vol-2", "ns-a/vol-3"}, seen)
	})

	t.Run("no readable namespace is permission denied", func(t *testing.T) {
		other := mock.CreatePolicyAndToken(t, s.State(), 201, "none",
			mock.NamespacePolicy("ns-a", "", []string{acl.NamespaceCapabilityReadJob}))
		req := &structs.HostVolumeListRequest{}
		req.Region, req.Namespace, req.AuthToken = "global", structs.AllNamespacesSentinel, other.SecretID
		var resp structs.HostVolumeListResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})

	t.Run("readable namespace without volumes is empty", func(t *testing.T) {
		upsertHostVolumeNamespaces(t, s, 60, "ns-empty")
		empty := mock.CreatePolicyAndToken(t, s.State(), 202, "empty",
			mock.NamespacePolicy("ns-empty", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
		req := &structs.HostVolumeListRequest{}
		req.Region, req.Namespace, req.AuthToken = "global", structs.AllNamespacesSentinel, empty.SecretID
		var resp structs.HostVolumeListResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp))
		must.NotNil(t, resp.Volumes)
		must.Len(t, 0, resp.Volumes)
		must.Eq(t, uint64(100), resp.Index)
	})

	t.Run("management sees every namespace", func(t *testing.T) {
		req := &structs.HostVolumeListRequest{}
		req.Region, req.Namespace, req.AuthToken = "global", structs.AllNamespacesSentinel, root.SecretID
		var resp structs.HostVolumeListResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp))
		must.Len(t, 5, resp.Volumes)
	})
}

func TestHostVolumeEndpoint_List_Filters(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	upsertHostVolumeNamespaces(t, s, 50, "ns-b")
	must.NoError(t, s.State().UpsertDynamicHostVolumes(100, []*structs.DynamicHostVolume{
		testHostVolume("default", "alpha-1", "node-1", "pool-x"),
		testHostVolume("default", "alpha-2", "node-2", "pool-y"),
		testHostVolume("default", "beta-1", "node-1", "pool-y"),
		testHostVolume("ns-b", "alpha-3", "node-1", "pool-x"),
		testHostVolume("ns-b", "beta-2", "node-2", "pool-y"),
	}))

	cases := []struct {
		name      string
		namespace string
		prefix    string
		node      string
		pool      string
		filter    string
		expect    []string
	}{
		{name: "single namespace all", namespace: "default",
			expect: []string{"default/alpha-1", "default/alpha-2", "default/beta-1"}},
		{name: "single namespace prefix", namespace: "default", prefix: "alpha",
			expect: []string{"default/alpha-1", "default/alpha-2"}},
		{name: "wildcard prefix", namespace: "*", prefix: "alpha",
			expect: []string{"default/alpha-1", "default/alpha-2", "ns-b/alpha-3"}},
		{name: "single namespace node", namespace: "default", node: "node-1",
			expect: []string{"default/alpha-1", "default/beta-1"}},
		{name: "wildcard node", namespace: "*", node: "node-1",
			expect: []string{"default/alpha-1", "default/beta-1", "ns-b/alpha-3"}},
		{name: "single namespace pool", namespace: "default", pool: "pool-y",
			expect: []string{"default/alpha-2", "default/beta-1"}},
		{name: "wildcard pool", namespace: "*", pool: "pool-x",
			expect: []string{"default/alpha-1", "ns-b/alpha-3"}},
		{name: "combined", namespace: "*", prefix: "alpha", node: "node-1", pool: "pool-x",
			expect: []string{"default/alpha-1", "ns-b/alpha-3"}},
		{name: "filter expression", namespace: "*", filter: `Name == "name-beta-2"`,
			expect: []string{"ns-b/beta-2"}},
		{name: "no match", namespace: "default", node: "node-9", expect: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.HostVolumeListRequest{NodeID: tc.node, NodePool: tc.pool}
			req.Region, req.Namespace, req.Prefix, req.Filter = "global", tc.namespace, tc.prefix, tc.filter
			var resp structs.HostVolumeListResponse
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp))
			must.Eq(t, tc.expect, hostVolumeIDs(resp.Volumes))
		})
	}

	t.Run("stub omits host path", func(t *testing.T) {
		// The listing must not leak the host path on the wire, whatever the
		// stub's Go type looks like.
		req := &structs.HostVolumeListRequest{}
		req.Region, req.Namespace = "global", "default"
		var resp structs.HostVolumeListResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp))
		must.Len(t, 3, resp.Volumes)

		raw, err := json.Marshal(resp.Volumes)
		must.NoError(t, err)
		must.StrNotContains(t, string(raw), "HostPath")
		must.StrNotContains(t, string(raw), "/synthetic/")
	})

	t.Run("pagination", func(t *testing.T) {
		var seen []string
		token := ""
		for range 10 {
			req := &structs.HostVolumeListRequest{}
			req.Region, req.Namespace, req.PerPage, req.NextToken = "global", "*", 2, token
			var resp structs.HostVolumeListResponse
			must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp))
			must.True(t, len(resp.Volumes) <= 2)
			seen = append(seen, hostVolumeIDs(resp.Volumes)...)
			if resp.NextToken == "" {
				break
			}
			token = resp.NextToken
		}
		must.Eq(t, []string{"default/alpha-1", "default/alpha-2", "default/beta-1", "ns-b/alpha-3", "ns-b/beta-2"}, seen)
	})

	t.Run("bad filter expression", func(t *testing.T) {
		req := &structs.HostVolumeListRequest{}
		req.Region, req.Namespace, req.Filter = "global", "default", "not a ( filter"
		var resp structs.HostVolumeListResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp)
		must.Error(t, err)
		must.StrContains(t, err.Error(), "failed to create result paginator")
	})
}

func TestHostVolumeEndpoint_Get(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	must.NoError(t, s.State().UpsertDynamicHostVolumes(100,
		[]*structs.DynamicHostVolume{testHostVolume("default", "vol-1", "node-1", "default")}))

	call := func(ns, id string) (*structs.HostVolumeGetResponse, error) {
		req := &structs.HostVolumeGetRequest{ID: id}
		req.Region, req.Namespace = "global", ns
		var resp structs.HostVolumeGetResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.Get", req, &resp)
		return &resp, err
	}

	t.Run("found", func(t *testing.T) {
		resp, err := call("default", "vol-1")
		must.NoError(t, err)
		must.Eq(t, "vol-1", resp.Volume.ID)
	})
	t.Run("unknown", func(t *testing.T) {
		resp, err := call("default", "nope")
		must.NoError(t, err)
		must.Nil(t, resp.Volume)
		must.Eq(t, uint64(100), resp.Index)
	})
	t.Run("other namespace", func(t *testing.T) {
		upsertHostVolumeNamespaces(t, s, 60, "ns-b")
		resp, err := call("ns-b", "vol-1")
		must.NoError(t, err)
		must.Nil(t, resp.Volume)
	})
	t.Run("missing id", func(t *testing.T) {
		_, err := call("default", "")
		must.ErrorContains(t, err, "missing volume ID")
		must.ErrorContains(t, err, "400")
	})
	t.Run("wildcard namespace", func(t *testing.T) {
		_, err := call("*", "vol-1")
		must.ErrorContains(t, err, "400")
	})
	t.Run("NUL in id", func(t *testing.T) {
		_, err := call("default", "a\x00b")
		must.ErrorContains(t, err, "400")
		must.ErrorContains(t, err, "invalid volume ID")
	})
}

func TestHostVolumeEndpoint_Blocking(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	vol := testHostVolume("default", "vol-1", "node-1", "default")
	must.NoError(t, s.State().UpsertDynamicHostVolumes(100, []*structs.DynamicHostVolume{vol}))

	qo := structs.QueryOptions{Region: "global", Namespace: "default", MinQueryIndex: 100, MaxQueryTime: 10 * time.Second}
	delay := 300 * time.Millisecond

	// Writes run on a timer goroutine, so their errors travel back over a
	// channel and are checked on the test goroutine.
	write := func(f func() error) chan error {
		errCh := make(chan error, 1)
		time.AfterFunc(delay, func() { errCh <- f() })
		return errCh
	}

	t.Run("list", func(t *testing.T) {
		errCh := write(func() error {
			next := testHostVolume("default", "vol-2", "node-1", "default")
			return s.State().UpsertDynamicHostVolumes(110, []*structs.DynamicHostVolume{next})
		})
		start := time.Now()
		var resp structs.HostVolumeListResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", &structs.HostVolumeListRequest{QueryOptions: qo}, &resp))
		must.True(t, time.Since(start) >= delay)
		must.NoError(t, <-errCh)
		must.True(t, resp.Index >= 110)
		must.Len(t, 2, resp.Volumes)
	})

	t.Run("get", func(t *testing.T) {
		q := qo
		q.MinQueryIndex = 110
		errCh := write(func() error {
			v := vol.Copy()
			v.CreateIndex, v.ModifyIndex = 100, 100
			v.CapacityBytes = 7
			return s.State().UpsertDynamicHostVolumes(120, []*structs.DynamicHostVolume{v})
		})
		start := time.Now()
		var resp structs.HostVolumeGetResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.Get", &structs.HostVolumeGetRequest{ID: "vol-1", QueryOptions: q}, &resp))
		must.True(t, time.Since(start) >= delay)
		must.NoError(t, <-errCh)
		must.True(t, resp.Index >= 120)
		must.Eq(t, int64(7), resp.Volume.CapacityBytes)
	})

	t.Run("claims write does not move index and times out", func(t *testing.T) {
		q := qo
		q.MinQueryIndex = 120
		q.MaxQueryTime = 500 * time.Millisecond
		errCh := make(chan error, 1)
		time.AfterFunc(100*time.Millisecond, func() {
			errCh <- s.State().UpsertDynamicHostVolumeClaims(130, []*structs.DynamicHostVolumeClaim{{
				ID: "claim-1", Namespace: "default", JobID: "job", TaskGroupName: "g",
				VolumeName: "name-vol-1", VolumeID: "vol-1", AllocationID: "alloc", AllocationIndex: 1,
			}})
		})
		start := time.Now()
		var resp structs.HostVolumeListResponse
		must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", &structs.HostVolumeListRequest{QueryOptions: q}, &resp))
		must.NoError(t, <-errCh)
		must.True(t, time.Since(start) >= 400*time.Millisecond)
		must.Eq(t, uint64(120), resp.Index)
	})
}

// A token whose only host-volume-read grant is on a namespace that does not
// exist passes the wildcard check but has nothing readable, so it gets an
// empty list rather than an error. A token with no grant at all is denied.
func TestHostVolumeEndpoint_List_NoExistingReadableNamespace(t *testing.T) {
	ci.Parallel(t)
	s, _, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	must.NoError(t, s.State().UpsertDynamicHostVolumes(100,
		[]*structs.DynamicHostVolume{testHostVolume("default", "vol-1", "node-1", "default")}))

	list := func(token string) (*structs.HostVolumeListResponse, error) {
		req := &structs.HostVolumeListRequest{}
		req.Region, req.Namespace, req.AuthToken = "global", structs.AllNamespacesSentinel, token
		var resp structs.HostVolumeListResponse
		err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp)
		return &resp, err
	}

	t.Run("exact missing namespace", func(t *testing.T) {
		tok := mock.CreatePolicyAndToken(t, s.State(), 200, "ghost",
			mock.NamespacePolicy("ns-ghost", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
		resp, err := list(tok.SecretID)
		must.NoError(t, err)
		must.NotNil(t, resp.Volumes)
		must.Len(t, 0, resp.Volumes)
		must.Eq(t, uint64(100), resp.Index)
	})

	t.Run("glob matching no existing namespace", func(t *testing.T) {
		tok := mock.CreatePolicyAndToken(t, s.State(), 202, "glob",
			mock.NamespacePolicy("ghost-*", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
		resp, err := list(tok.SecretID)
		must.NoError(t, err)
		must.NotNil(t, resp.Volumes)
		must.Len(t, 0, resp.Volumes)
		must.Eq(t, uint64(100), resp.Index)
	})

	t.Run("no host volume read anywhere", func(t *testing.T) {
		tok := mock.CreatePolicyAndToken(t, s.State(), 204, "nothing",
			mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityReadJob}))
		_, err := list(tok.SecretID)
		must.EqError(t, err, structs.ErrPermissionDenied.Error())
	})
}

// A table that has never been written reports index 0 internally; the
// endpoints report 1 so that blocking clients do not spin.
func TestHostVolumeEndpoint_IndexNeverBelowOne(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	qo := structs.QueryOptions{Region: "global", Namespace: "default"}

	var list structs.HostVolumeListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.List", &structs.HostVolumeListRequest{QueryOptions: qo}, &list))
	must.Len(t, 0, list.Volumes)
	must.Eq(t, uint64(1), list.Index)

	var get structs.HostVolumeGetResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "HostVolume.Get", &structs.HostVolumeGetRequest{ID: "vol-1", QueryOptions: qo}, &get))
	must.Nil(t, get.Volume)
	must.Eq(t, uint64(1), get.Index)
}

// A NUL byte is never valid in a namespace or ID prefix; the state store
// rejects it and the endpoint answers with a fixed coded 400.
func TestHostVolumeEndpoint_List_NULInKey(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)

	must.NoError(t, s.State().UpsertDynamicHostVolumes(100,
		[]*structs.DynamicHostVolume{testHostVolume("default", "vol-1", "node-1", "default")}))

	cases := []struct{ name, namespace, prefix string }{
		{name: "prefix", namespace: "default", prefix: "a\x00b"},
		{name: "namespace", namespace: "a\x00b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.HostVolumeListRequest{}
			req.Region, req.Namespace, req.Prefix = "global", tc.namespace, tc.prefix
			var resp structs.HostVolumeListResponse
			err := msgpackrpc.CallWithCodec(codec, "HostVolume.List", req, &resp)
			must.ErrorContains(t, err, "400")
			must.ErrorContains(t, err, "invalid namespace or prefix")
		})
	}
}

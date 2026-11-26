// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"net/rpc"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// claimSpec describes one synthetic claim, its volume and, when live is set, a
// pending allocation of the claim's group that uses the volume.
type claimSpec struct {
	ns, id, job, group, volume string
	live                       bool
}

// seedClaims writes the fixtures straight into the state store at index 1, so
// that a later write through Raft is above every table index. It runs once per
// server, because the claims table index must move forward.
func seedClaims(t *testing.T, s *Server, specs ...claimSpec) []*structs.DynamicHostVolumeClaim {
	t.Helper()
	must.Less(t, s.raft.LastIndex(), uint64(2), must.Sprint("the seed index must be below the Raft log"))
	nsSeen, volSeen := map[string]bool{}, map[string]bool{}
	var vols []*structs.DynamicHostVolume
	var claims []*structs.DynamicHostVolumeClaim
	for _, sp := range specs {
		if !nsSeen[sp.ns] && sp.ns != structs.DefaultNamespace {
			nsSeen[sp.ns] = true
			upsertHostVolumeNamespaces(t, s, 1, sp.ns)
		}
		// Claims of one volume name share a volume. Each namespace has its
		// own node, because a volume name is unique on a node.
		volKey := sp.ns + "/" + sp.volume
		if !volSeen[volKey] {
			volSeen[volKey] = true
			vol := testHostVolume(sp.ns, "vol-"+sp.volume, "node-"+sp.ns, "default")
			vol.Name = sp.volume
			vols = append(vols, vol)
		}
		claims = append(claims, &structs.DynamicHostVolumeClaim{ID: sp.id, Namespace: sp.ns, JobID: sp.job,
			TaskGroupName: sp.group, VolumeName: sp.volume, VolumeID: "vol-" + sp.volume,
			AllocationID: "00000000-0000-0000-0000-00000000" + fmt.Sprintf("%04d", len(claims)+1), AllocationIndex: 1})
	}
	must.NoError(t, s.State().UpsertDynamicHostVolumes(1, vols))
	for _, sp := range specs {
		if !sp.live {
			continue
		}
		job := mock.Job()
		job.ID, job.Namespace = sp.job, sp.ns
		job.TaskGroups[0].Name = sp.group
		job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
			"data": {Name: "data", Type: structs.VolumeTypeHost, Source: sp.volume}}
		must.NoError(t, s.State().UpsertJob(structs.MsgTypeTestSetup, 1, nil, job))
		alloc := mock.Alloc()
		alloc.Job, alloc.JobID, alloc.Namespace, alloc.TaskGroup = job, job.ID, job.Namespace, sp.group
		alloc.NodeID = "node-" + sp.ns
		alloc.ClientStatus, alloc.DesiredStatus = structs.AllocClientStatusPending, structs.AllocDesiredStatusRun
		must.NoError(t, s.State().UpsertAllocs(structs.MsgTypeTestSetup, 1, []*structs.Allocation{alloc}))
	}
	must.NoError(t, s.State().UpsertDynamicHostVolumeClaims(1, claims))
	return claims
}

func claimIDs(claims []*structs.DynamicHostVolumeClaim) []string {
	ids := []string{}
	for _, c := range claims {
		ids = append(ids, c.Namespace+"/"+c.ID)
	}
	return ids
}

func listClaims(codec rpc.ClientCodec, req *structs.HostVolumeClaimListRequest) (*structs.HostVolumeClaimListResponse, error) {
	if req.Region == "" {
		req.Region = "global"
	}
	if req.Namespace == "" {
		req.Namespace = structs.DefaultNamespace
	}
	var resp structs.HostVolumeClaimListResponse
	err := msgpackrpc.CallWithCodec(codec, "HostVolume.ListClaims", req, &resp)
	return &resp, err
}

func deleteClaim(codec rpc.ClientCodec, ns, id, token string) (*structs.HostVolumeClaimDeleteResponse, error) {
	req := &structs.HostVolumeClaimDeleteRequest{ID: id}
	req.Region, req.Namespace, req.AuthToken = "global", ns, token
	var resp structs.HostVolumeClaimDeleteResponse
	err := msgpackrpc.CallWithCodec(codec, "HostVolume.DeleteClaim", req, &resp)
	return &resp, err
}

func storedClaimIDs(t *testing.T, s *Server) []string {
	t.Helper()
	iter, err := s.State().DynamicHostVolumeClaims(nil)
	must.NoError(t, err)
	ids := []string{}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		c := raw.(*structs.DynamicHostVolumeClaim)
		ids = append(ids, c.Namespace+"/"+c.ID)
	}
	return ids
}

func TestHostVolumeClaim_ACL(t *testing.T) {
	ci.Parallel(t)
	s, root, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	ns := structs.DefaultNamespace
	seedClaims(t, s,
		claimSpec{ns: ns, id: "claim-1", job: "job-1", group: "g", volume: "data-1"},
		claimSpec{ns: ns, id: "claim-2", job: "job-2", group: "g", volume: "data-2"},
		claimSpec{ns: ns, id: "claim-3", job: "job-3", group: "g", volume: "data-3"},
		claimSpec{ns: ns, id: "claim-4", job: "job-4", group: "g", volume: "data-4"})

	pol := func(caps ...string) string { return mock.NamespacePolicy(ns, "", caps) }
	cases := []struct {
		name         string
		policy       string
		token        string
		list, delete bool
	}{
		{name: "no token", token: "none"},
		{name: "unknown token", token: "bogus"},
		{name: "no host volume capability", policy: pol(acl.NamespaceCapabilityReadJob)},
		{name: "read lists but cannot delete", policy: pol(acl.NamespaceCapabilityHostVolumeRead), list: true},
		{name: "create lists but cannot delete", policy: pol(acl.NamespaceCapabilityHostVolumeCreate), list: true},
		{name: "delete capability alone neither lists nor deletes", policy: pol(acl.NamespaceCapabilityHostVolumeDelete)},
		{name: "write lists and deletes", policy: pol(acl.NamespaceCapabilityHostVolumeWrite), list: true, delete: true},
		{name: "deny", policy: mock.NamespacePolicy(ns, acl.PolicyDeny, []string{acl.NamespaceCapabilityHostVolumeWrite})},
		{name: "management", token: "root", list: true, delete: true},
	}
	next := 0
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
				secret = mock.CreatePolicyAndToken(t, s.State(), uint64(1000+i*2), fmt.Sprintf("p%d", i), tc.policy).SecretID
			}
			req := &structs.HostVolumeClaimListRequest{}
			req.AuthToken = secret
			resp, err := listClaims(codec, req)
			if tc.list {
				must.NoError(t, err)
				must.Len(t, len(storedClaimIDs(t, s)), resp.Claims)
			} else {
				must.EqError(t, err, structs.ErrPermissionDenied.Error())
			}

			before := storedClaimIDs(t, s)
			target := "claim-" + fmt.Sprint(next+1)
			_, err = deleteClaim(codec, ns, target, secret)
			if tc.delete {
				next++
				must.NoError(t, err)
				must.Eq(t, len(before)-1, len(storedClaimIDs(t, s)))
				return
			}
			must.EqError(t, err, structs.ErrPermissionDenied.Error())
			must.Eq(t, before, storedClaimIDs(t, s))
		})
	}
}

func TestHostVolumeClaim_List_Wildcard(t *testing.T) {
	ci.Parallel(t)
	s, root, cleanup := TestACLServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	seedClaims(t, s,
		claimSpec{ns: "ns-a", id: "claim-1", job: "job", group: "g", volume: "d1"},
		claimSpec{ns: "ns-a", id: "claim-2", job: "job", group: "g", volume: "d2"},
		claimSpec{ns: "ns-a", id: "claim-3", job: "job", group: "g", volume: "d3"},
		claimSpec{ns: "ns-b", id: "claim-1", job: "job", group: "g", volume: "d1"},
		claimSpec{ns: "ns-b", id: "claim-secret", job: "job", group: "g", volume: "d2"})
	tok := mock.CreatePolicyAndToken(t, s.State(), 200, "reader-a",
		mock.NamespacePolicy("ns-a", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
	list := func(mutate func(*structs.HostVolumeClaimListRequest)) (*structs.HostVolumeClaimListResponse, error) {
		req := &structs.HostVolumeClaimListRequest{}
		req.Namespace, req.AuthToken = structs.AllNamespacesSentinel, tok.SecretID
		mutate(req)
		return listClaims(codec, req)
	}
	all := []string{"ns-a/claim-1", "ns-a/claim-2", "ns-a/claim-3"}

	resp, err := list(func(*structs.HostVolumeClaimListRequest) {})
	must.NoError(t, err)
	must.Eq(t, all, claimIDs(resp.Claims))

	resp, err = list(func(r *structs.HostVolumeClaimListRequest) { r.Prefix = "claim-secret" })
	must.NoError(t, err)
	must.Len(t, 0, resp.Claims)

	resp, err = list(func(r *structs.HostVolumeClaimListRequest) { r.Prefix = "claim-2" })
	must.NoError(t, err)
	must.Eq(t, []string{"ns-a/claim-2"}, claimIDs(resp.Claims))

	var seen []string
	token := ""
	for range 10 {
		resp, err := list(func(r *structs.HostVolumeClaimListRequest) { r.PerPage, r.NextToken = 1, token })
		must.NoError(t, err)
		seen = append(seen, claimIDs(resp.Claims)...)
		if resp.NextToken == "" {
			break
		}
		must.StrNotContains(t, resp.NextToken, "ns-b")
		token = resp.NextToken
	}
	must.Eq(t, all, seen)

	other := mock.CreatePolicyAndToken(t, s.State(), 201, "none",
		mock.NamespacePolicy("ns-a", "", []string{acl.NamespaceCapabilityReadJob}))
	_, err = list(func(r *structs.HostVolumeClaimListRequest) { r.AuthToken = other.SecretID })
	must.EqError(t, err, structs.ErrPermissionDenied.Error())

	resp, err = list(func(r *structs.HostVolumeClaimListRequest) { r.AuthToken = root.SecretID })
	must.NoError(t, err)
	must.Len(t, 5, resp.Claims)

	// The ID is the pagination key, so claims with one ID in two namespaces
	// are both returned.
	resp, err = list(func(r *structs.HostVolumeClaimListRequest) { r.AuthToken, r.Prefix = root.SecretID, "claim-1" })
	must.NoError(t, err)
	must.Eq(t, []string{"ns-a/claim-1", "ns-b/claim-1"}, claimIDs(resp.Claims))
}

func TestHostVolumeClaim_List_Filters(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	seedClaims(t, s,
		claimSpec{ns: "default", id: "abc-1", job: "job-a", group: "web", volume: "data"},
		claimSpec{ns: "default", id: "abc-2", job: "job-a", group: "db", volume: "data"},
		claimSpec{ns: "default", id: "abd-3", job: "job-b", group: "web", volume: "logs"},
		claimSpec{ns: "other", id: "abc-9", job: "job-a", group: "web", volume: "data"})

	cases := []struct {
		name   string
		mutate func(*structs.HostVolumeClaimListRequest)
		want   []string
	}{
		{"namespace only", func(*structs.HostVolumeClaimListRequest) {}, []string{"default/abc-1", "default/abc-2", "default/abd-3"}},
		{"other namespace", func(r *structs.HostVolumeClaimListRequest) { r.Namespace = "other" }, []string{"other/abc-9"}},
		{"claim prefix", func(r *structs.HostVolumeClaimListRequest) { r.Prefix = "abc" }, []string{"default/abc-1", "default/abc-2"}},
		{"job", func(r *structs.HostVolumeClaimListRequest) { r.JobID = "job-b" }, []string{"default/abd-3"}},
		{"task group", func(r *structs.HostVolumeClaimListRequest) { r.TaskGroup = "db" }, []string{"default/abc-2"}},
		{"volume name", func(r *structs.HostVolumeClaimListRequest) { r.VolumeName = "logs" }, []string{"default/abd-3"}},
		{"combined", func(r *structs.HostVolumeClaimListRequest) {
			r.Prefix, r.JobID, r.TaskGroup, r.VolumeName = "ab", "job-a", "web", "data"
		}, []string{"default/abc-1"}},
		{"no match", func(r *structs.HostVolumeClaimListRequest) { r.JobID = "job-z" }, []string{}},
		{"wildcard with filter", func(r *structs.HostVolumeClaimListRequest) {
			r.Namespace, r.JobID, r.TaskGroup = structs.AllNamespacesSentinel, "job-a", "web"
		}, []string{"default/abc-1", "other/abc-9"}},
		{"wildcard with prefix", func(r *structs.HostVolumeClaimListRequest) {
			r.Namespace, r.Prefix = structs.AllNamespacesSentinel, "abd"
		}, []string{"default/abd-3"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.HostVolumeClaimListRequest{}
			tc.mutate(req)
			resp, err := listClaims(codec, req)
			must.NoError(t, err)
			must.Eq(t, tc.want, claimIDs(resp.Claims))
		})
	}

	t.Run("pagination", func(t *testing.T) {
		var seen []string
		token := ""
		for range 10 {
			req := &structs.HostVolumeClaimListRequest{}
			req.PerPage, req.NextToken = 2, token
			resp, err := listClaims(codec, req)
			must.NoError(t, err)
			must.True(t, len(resp.Claims) <= 2)
			seen = append(seen, claimIDs(resp.Claims)...)
			if resp.NextToken == "" {
				break
			}
			token = resp.NextToken
		}
		must.Eq(t, []string{"default/abc-1", "default/abc-2", "default/abd-3"}, seen)
	})
}

func TestHostVolumeClaim_Delete(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	seedClaims(t, s,
		claimSpec{ns: "default", id: "free", job: "job-a", group: "g", volume: "data-a"},
		claimSpec{ns: "default", id: "held", job: "job-b", group: "g", volume: "data-b", live: true},
		claimSpec{ns: "other", id: "free", job: "job-a", group: "g", volume: "data-a"})

	t.Run("a claim in use is refused with 409 and stays", func(t *testing.T) {
		before := s.raft.LastIndex()
		_, err := deleteClaim(codec, "default", "held", "")
		mustCoded(t, err, 409, "claim in use")
		must.SliceContains(t, storedClaimIDs(t, s), "default/held")
		// The pre-check refuses without a log entry.
		must.Eq(t, before, s.raft.LastIndex())
	})

	t.Run("the apply refuses a held claim too", func(t *testing.T) {
		// The RPC pre-check is bypassed, as a racing or replayed entry would.
		claim, err := s.State().DynamicHostVolumeClaimByID(nil, "default", "held")
		must.NoError(t, err)
		_, _, err = s.dynamicHostVolumeRaftApply(structs.DynamicHostVolumeClaimDeleteRequestType,
			&structs.DynamicHostVolumeClaimDeleteRequest{
				Claims:       []structs.DynamicHostVolumeClaimKey{{Namespace: "default", ID: "held", ExpectedModifyIndex: claim.ModifyIndex}},
				WriteRequest: structs.WriteRequest{Region: "global"}})
		// The apply returns the state error itself, so the refusal is the
		// in-use rule and not some other failure such as a stale index.
		must.ErrorIs(t, err, state.ErrDynamicHostVolumeStateInUse)
		must.SliceContains(t, storedClaimIDs(t, s), "default/held")
	})

	t.Run("unknown claim is 404", func(t *testing.T) {
		_, err := deleteClaim(codec, "default", "missing", "")
		mustCoded(t, err, 404, "claim not found")

		// The lookup is keyed by the request namespace, so a token that can
		// write "default" cannot reach a claim in "other" by its ID. The ID
		// exists only in "other", on a server of its own with ACLs enabled.
		as, _, cleanup := TestACLServer(t, nil)
		defer cleanup()
		acodec := rpcClient(t, as)
		testutil.WaitForLeader(t, as.RPC)
		seedClaims(t, as,
			claimSpec{ns: "default", id: "local", job: "job-a", group: "g", volume: "data-a"},
			claimSpec{ns: "other", id: "elsewhere", job: "job-a", group: "g", volume: "data-a"})
		writer := mock.CreatePolicyAndToken(t, as.State(), 100, "writer-default",
			mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeWrite}))

		_, err = deleteClaim(acodec, "default", "elsewhere", writer.SecretID)
		mustCoded(t, err, 404, "claim not found")
		_, err = deleteClaim(acodec, "default", "missing", writer.SecretID)
		mustCoded(t, err, 404, "claim not found")
		must.Eq(t, []string{"default/local", "other/elsewhere"}, storedClaimIDs(t, as))

		// The claim is still reachable in its own namespace.
		other, err := as.State().DynamicHostVolumeClaimByID(nil, "other", "elsewhere")
		must.NoError(t, err)
		must.NotNil(t, other)
	})

	t.Run("wildcard namespace and bad IDs are 400", func(t *testing.T) {
		_, err := deleteClaim(codec, structs.AllNamespacesSentinel, "free", "")
		mustCoded(t, err, 400, "a claim delete needs a single namespace")
		_, err = deleteClaim(codec, "default", "", "")
		mustCoded(t, err, 400, "missing claim ID")
		_, err = deleteClaim(codec, "default", "bad\x00id", "")
		mustCoded(t, err, 400, "invalid claim ID")
		must.Eq(t, 3, len(storedClaimIDs(t, s)))
	})

	t.Run("a free claim is removed from its namespace only", func(t *testing.T) {
		resp, err := deleteClaim(codec, "default", "free", "")
		must.NoError(t, err)
		must.Eq(t, []string{"default/held", "other/free"}, storedClaimIDs(t, s))
		idx, err := s.State().Index("dynamic_host_volume_claims")
		must.NoError(t, err)
		must.Eq(t, idx, resp.Index)
	})
}

func TestHostVolumeClaim_Blocking(t *testing.T) {
	ci.Parallel(t)
	s, cleanup := TestServer(t, nil)
	defer cleanup()
	codec := rpcClient(t, s)
	testutil.WaitForLeader(t, s.RPC)
	seedClaims(t, s,
		claimSpec{ns: "default", id: "claim-1", job: "job-a", group: "g", volume: "data-a"},
		claimSpec{ns: "default", id: "claim-2", job: "job-b", group: "g", volume: "data-b"})

	// A claims table that was written at index 1 reports 1 and never 0.
	first, err := listClaims(codec, &structs.HostVolumeClaimListRequest{})
	must.NoError(t, err)
	must.Eq(t, uint64(1), first.Index)

	errCh := make(chan error, 1)
	time.AfterFunc(300*time.Millisecond, func() {
		_, err := deleteClaim(codec2(t, s), "default", "claim-1", "")
		errCh <- err
	})
	req := &structs.HostVolumeClaimListRequest{}
	req.MinQueryIndex, req.MaxQueryTime = 1, 10*time.Second
	start := time.Now()
	resp, err := listClaims(codec, req)
	must.NoError(t, err)
	must.True(t, time.Since(start) >= 300*time.Millisecond, must.Sprint("the query returned before the delete"))
	must.NoError(t, <-errCh)
	must.True(t, resp.Index > 1)
	must.Eq(t, []string{"default/claim-2"}, claimIDs(resp.Claims))
}

func codec2(t *testing.T, s *Server) rpc.ClientCodec {
	t.Helper()
	return rpcClient(t, s)
}

func TestHostVolumeClaim_Delete_VersionGate(t *testing.T) {
	ci.Parallel(t)
	s1, cleanup1 := TestServer(t, nil)
	defer cleanup1()
	s2, cleanup2 := TestServer(t, func(c *Config) {
		c.BootstrapExpect = 0
		c.WanderVersion = ""
	})
	defer cleanup2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForResult(func() (bool, error) {
		return len(s1.serf.Members()) == 2, fmt.Errorf("members: %d", len(s1.serf.Members()))
	}, func(err error) { must.NoError(t, err) })
	seedClaims(t, s1, claimSpec{ns: "default", id: "claim-1", job: "job-a", group: "g", volume: "data-a"})

	before := s1.raft.LastIndex()
	_, err := deleteClaim(rpcClient(t, s1), "default", "claim-1", "")
	mustCoded(t, err, 503, hostVolumeErrVersionGate)
	must.Eq(t, []string{"default/claim-1"}, storedClaimIDs(t, s1))
	must.Eq(t, before, s1.raft.LastIndex())
}

// A call made to a follower reaches the leader with its code intact.
func TestHostVolumeClaim_Forwarded(t *testing.T) {
	ci.Parallel(t)
	s1, cleanup1 := TestServer(t, nil)
	defer cleanup1()
	s2, cleanup2 := TestServer(t, func(c *Config) { c.BootstrapExpect = 0 })
	defer cleanup2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForResult(func() (bool, error) {
		return len(s1.serf.Members()) == 2 && len(s1.raft.GetConfiguration().Configuration().Servers) == 2, fmt.Errorf("not joined")
	}, func(err error) { must.NoError(t, err) })
	follower := s2
	if s2.IsLeader() {
		follower = s1
	}
	leader := s1
	if follower == s1 {
		leader = s2
	}
	seedClaims(t, leader, claimSpec{ns: "default", id: "held", job: "job-a", group: "g", volume: "data-a", live: true})

	codec := rpcClient(t, follower)
	_, err := deleteClaim(codec, "default", "held", "")
	mustCoded(t, err, 409, "claim in use")
	_, err = deleteClaim(codec, "default", "missing", "")
	mustCoded(t, err, 404, "claim not found")
	resp, err := listClaims(codec, &structs.HostVolumeClaimListRequest{})
	must.NoError(t, err)
	must.Eq(t, []string{"default/held"}, claimIDs(resp.Claims))
}

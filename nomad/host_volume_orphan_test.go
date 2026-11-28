// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

const (
	orphanErrMode = "mode must be delete or forget"
	orphanErrGone = "orphaned volume not found"
	orphanErrHas  = "volume has a record, not an orphan"
	orphanErrReg  = "volume was registered, use forget"
	orphanErrNS   = "namespace is not the namespace of the volume on the node"
)

// advertised builds the entry a node advertises for a dynamic volume.
func advertised(id, name, ns string) *structs.ClientHostVolumeConfig {
	return &structs.ClientHostVolumeConfig{Name: name, Path: "/synthetic/path/" + id, ID: id, Namespace: ns}
}

// advertise replaces the host volumes a node advertises. The write is made
// straight into the state store, at an index above the Raft log.
func (e *dhvEnv) advertise(nodeID string, vols ...*structs.ClientHostVolumeConfig) {
	e.t.Helper()
	n, err := e.s.State().NodeByID(nil, nodeID)
	must.NoError(e.t, err)
	must.NotNil(e.t, n)
	n = n.Copy()
	n.HostVolumes = map[string]*structs.ClientHostVolumeConfig{}
	for _, v := range vols {
		n.HostVolumes[v.Name] = v
	}
	must.NoError(e.t, e.s.State().UpsertNode(structs.MsgTypeTestSetup, nextDHVIndex(), n))
}

func (e *dhvEnv) secretFor(secret string) string {
	switch secret {
	case "":
		return e.root
	case "-":
		return ""
	}
	return secret
}

func (e *dhvEnv) listOrphans(secret, ns, node string) (*structs.HostVolumeOrphanListResponse, error) {
	e.t.Helper()
	var resp structs.HostVolumeOrphanListResponse
	err := e.rpc("HostVolume.ListOrphans", &structs.HostVolumeOrphanListRequest{
		NodeID:       node,
		QueryOptions: structs.QueryOptions{Region: "global", Namespace: ns, AuthToken: e.secretFor(secret)},
	}, &resp)
	return &resp, err
}

func (e *dhvEnv) removeOrphan(secret, ns, node, id, mode string) error {
	e.t.Helper()
	var resp structs.HostVolumeOrphanRemoveResponse
	return e.rpc("HostVolume.RemoveOrphan", &structs.HostVolumeOrphanRemoveRequest{
		NodeID: node, VolumeID: id, Mode: mode,
		WriteRequest: structs.WriteRequest{Region: "global", Namespace: ns, AuthToken: e.secretFor(secret)},
	}, &resp)
}

func orphanKeys(resp *structs.HostVolumeOrphanListResponse) []string {
	out := []string{}
	for _, o := range resp.Orphans {
		out = append(out, o.NodeID+"/"+o.VolumeID)
	}
	return out
}

func TestHostVolumeListOrphans(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, true)
	must.NoError(t, e.s.State().UpsertNamespaces(50, []*structs.Namespace{{Name: "other"}, {Name: "third"}}))

	// Both nodes are ones no client touches.
	nodeA := e.stateNode(structs.NodeStatusReady)
	nodeB := e.stateNode(structs.NodeStatusReady)
	nodeC := e.stateNode(structs.NodeStatusReady)
	static := &structs.ClientHostVolumeConfig{Name: "static", Path: "/synthetic/static"}
	e.advertise(nodeA,
		advertised("vol-def", "d1", "default"),
		advertised("vol-oth", "o1", "other"),
		advertised("vol-rec", "r1", "default"),
		advertised("vol-old", "old1", ""),
		advertised("vol-else", "e1", "default"),
		static)
	e.advertise(nodeB, advertised("vol-b", "b1", "default"))
	e.advertise(nodeC, static)

	// vol-rec has a record on its node. vol-else has one on another node:
	// the listing only looks at the node that advertises the volume.
	rec := dhvSeedVolume("vol-rec", "r1", nodeA, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	elsewhere := dhvSeedVolume("vol-else", "e1", nodeB, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	e.seedAll(rec, elsewhere)

	pol := func(i int, rules ...string) string {
		return mock.CreatePolicyAndToken(t, e.s.State(), uint64(2000+i*2), fmt.Sprintf("orph%d", i), strings.Join(rules, "\n")).SecretID
	}
	read := func(ns string) string {
		return mock.NamespacePolicy(ns, "", []string{acl.NamespaceCapabilityHostVolumeRead})
	}
	readDefault := pol(1, read("default"))
	readOther := pol(2, read("other"))
	readDefaultNode := pol(3, read("default"), mock.NodePolicy(acl.PolicyRead))
	readAll := pol(4, read("default"), read("other"))
	jobsOnly := pol(5, mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityReadJob}))
	policyRead := pol(6, mock.NamespacePolicy("default", acl.PolicyRead, nil))

	a := func(id string) string { return nodeA + "/" + id }
	b := func(id string) string { return nodeB + "/" + id }
	cases := []struct {
		name, secret, ns, node string
		want                   []string
		denied                 bool
	}{
		{name: "management, every namespace", ns: "*", node: "",
			want: sortedKeys(a("vol-def"), a("vol-oth"), a("vol-old"), a("vol-else"), b("vol-b"))},
		{name: "management, default only", ns: "default", want: sortedKeys(a("vol-def"), a("vol-else"), b("vol-b"))},
		{name: "management, other only", ns: "other", want: sortedKeys(a("vol-oth"))},
		{name: "management, third has none", ns: "third", want: []string{}},
		{name: "node filter", ns: "*", node: nodeA, want: sortedKeys(a("vol-def"), a("vol-oth"), a("vol-old"), a("vol-else"))},
		{name: "node filter and namespace", ns: "default", node: nodeB, want: sortedKeys(b("vol-b"))},
		{name: "node with only static volumes", ns: "*", node: nodeC, want: []string{}},
		{name: "malformed node ID", ns: "*", node: "node", want: []string{}},
		{name: "unknown node", ns: "*", node: "5b0c0a5e-0000-4000-8000-000000000001", want: []string{}},
		{name: "read default, wildcard", secret: readDefault, ns: "*", want: sortedKeys(a("vol-def"), a("vol-else"), b("vol-b"))},
		{name: "read default, own namespace", secret: readDefault, ns: "default", want: sortedKeys(a("vol-def"), a("vol-else"), b("vol-b"))},
		{name: "read default, other namespace", secret: readDefault, ns: "other", denied: true},
		{name: "read other, wildcard", secret: readOther, ns: "*", want: sortedKeys(a("vol-oth"))},
		{name: "policy read", secret: policyRead, ns: "*", want: sortedKeys(a("vol-def"), a("vol-else"), b("vol-b"))},
		{name: "node read adds the empty namespace to the wildcard", secret: readDefaultNode, ns: "*",
			want: sortedKeys(a("vol-def"), a("vol-old"), a("vol-else"), b("vol-b"))},
		{name: "empty namespace never matches a named one", secret: readDefaultNode, ns: "default",
			want: sortedKeys(a("vol-def"), a("vol-else"), b("vol-b"))},
		{name: "two namespaces", secret: readAll, ns: "*", want: sortedKeys(a("vol-def"), a("vol-oth"), a("vol-else"), b("vol-b"))},
		{name: "no host volume capability", secret: jobsOnly, ns: "default", denied: true},
		{name: "no token", secret: "-", ns: "default", denied: true},
		{name: "unknown token", secret: "00000000-0000-0000-0000-000000000000", ns: "default", denied: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := e.with(t)
			resp, err := e.listOrphans(tc.secret, tc.ns, tc.node)
			if tc.denied {
				must.EqError(t, err, structs.ErrPermissionDenied.Error())
				return
			}
			must.NoError(t, err)
			must.Eq(t, tc.want, orphanKeys(resp))
			must.GreaterEq(t, 1, resp.Index)
		})
	}

	t.Run("fields and no host path", func(t *testing.T) {
		resp, err := e.listOrphans("", "other", nodeA)
		must.NoError(t, err)
		must.Eq(t, []*structs.HostVolumeOrphan{{NodeID: nodeA, VolumeID: "vol-oth", Name: "o1", Namespace: "other"}}, resp.Orphans)
		_, hasPath := reflect.TypeOf(structs.HostVolumeOrphan{}).FieldByName("Path")
		must.False(t, hasPath)
		all, err := e.listOrphans("", "*", "")
		must.NoError(t, err)
		raw, err := json.Marshal(all)
		must.NoError(t, err)
		must.StrNotContains(t, string(raw), "/synthetic/path")
	})

	t.Run("a volume with a record is never listed", func(t *testing.T) {
		resp, err := e.listOrphans("", "*", nodeA)
		must.NoError(t, err)
		for _, o := range resp.Orphans {
			must.NotEq(t, "vol-rec", o.VolumeID)
		}
	})
}

func sortedKeys(keys ...string) []string {
	out := append([]string{}, keys...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// A blocked listing wakes when a record for an advertised volume appears, and
// then no longer lists it.
func TestHostVolumeListOrphans_Blocking(t *testing.T) {
	ci.Parallel(t)
	e := newDHVEnv(t, false)
	node := e.stateNode(structs.NodeStatusReady)
	e.advertise(node, advertised("vol-x", "x1", "default"))

	first, err := e.listOrphans("", "default", node)
	must.NoError(t, err)
	must.Eq(t, []string{node + "/vol-x"}, orphanKeys(first))

	type result struct {
		resp *structs.HostVolumeOrphanListResponse
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var resp structs.HostVolumeOrphanListResponse
		err := e.rpc("HostVolume.ListOrphans", &structs.HostVolumeOrphanListRequest{
			NodeID: node,
			QueryOptions: structs.QueryOptions{
				Region: "global", Namespace: "default", MinQueryIndex: first.Index, MaxQueryTime: 10 * time.Second,
			},
		}, &resp)
		done <- result{&resp, err}
	}()
	select {
	case <-done:
		t.Fatal("the listing returned before anything changed")
	case <-time.After(300 * time.Millisecond):
	}

	e.seedVolumesAt(nextDHVIndex(), dhvSeedVolume("vol-x", "x1", node, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated))
	select {
	case r := <-done:
		must.NoError(t, r.err)
		must.Eq(t, []string{}, orphanKeys(r.resp))
		must.Greater(t, first.Index, r.resp.Index)
	case <-time.After(10 * time.Second):
		t.Fatal("the listing did not wake")
	}
}

// orphanEnv is a server with ACLs, the client node advertising one orphan of
// each kind and tokens for each capability.
type orphanEnv struct {
	*dhvEnv
	node string
}

func newOrphanEnv(t *testing.T) *orphanEnv {
	t.Helper()
	e := newDHVEnv(t, true)
	must.NoError(t, e.s.State().UpsertNamespaces(50, []*structs.Namespace{{Name: "other"}}))
	return &orphanEnv{dhvEnv: e, node: e.nodeID}
}

func (o *orphanEnv) counts() (deletes, deregs, forgets int) {
	d, r := o.mock.removalCounts()
	return d, r, o.mock.forgetCount()
}

func (o *orphanEnv) mustNoClientCall() {
	o.t.Helper()
	d, r, f := o.counts()
	must.Eq(o.t, 0, d+r+f, must.Sprint("the client was called"))
}

func TestHostVolumeRemoveOrphan_Refusals(t *testing.T) {
	ci.Parallel(t)
	o := newOrphanEnv(t)
	e := o.dhvEnv

	other := e.stateNode(structs.NodeStatusReady)
	e.seedAll(
		dhvSeedVolume("vol-rec", "r1", o.node, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated),
		dhvSeedVolume("vol-rec-b", "rb", other, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated),
	)
	otherNS := dhvSeedVolume("vol-rec-o", "ro", o.node, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	otherNS.Namespace = "other"
	e.seedAll(otherNS)

	adv := func() {
		e.advertise(o.node,
			advertised("vol-orph", "orph", "default"),
			advertised("vol-rec", "r1", "default"),
			advertised("vol-rec-b", "rb", "default"),
			advertised("vol-rec-o", "ro", "default"),
			advertised("vol-other-ns", "ons", "other"),
			advertised("vol-busy", "busy", "default"),
		)
	}
	adv()
	e.use(o.node, "busy", structs.AllocClientStatusRunning)
	adv()

	cases := []struct {
		name         string
		ns, node, id string
		mode         string
		code         int
		msg          string
	}{
		{"mode missing", "default", o.node, "vol-orph", "", 400, orphanErrMode},
		{"mode unknown", "default", o.node, "vol-orph", "remove", 400, orphanErrMode},
		{"mode in upper case", "default", o.node, "vol-orph", "Delete", 400, orphanErrMode},
		{"mode all caps", "default", o.node, "vol-orph", "FORGET", 400, orphanErrMode},
		{"mode with a space", "default", o.node, "vol-orph", "delete ", 400, orphanErrMode},
		{"wildcard namespace", "*", o.node, "vol-orph", "forget", 400, hostVolumeErrSingleNamespace},
		{"empty volume ID", "default", o.node, "", "forget", 400, hostVolumeErrMissingID},
		{"empty node ID", "default", "", "vol-orph", "forget", 400, "missing node ID"},
		{"malformed node ID", "default", "node", "vol-orph", "forget", 404, orphanErrGone},
		{"unknown node", "default", "5b0c0a5e-0000-4000-8000-000000000001", "vol-orph", "forget", 404, orphanErrGone},
		{"not advertised", "default", o.node, "vol-nothing", "forget", 404, orphanErrGone},
		{"namespace differs from the advertised one", "default", o.node, "vol-other-ns", "forget", 404, orphanErrGone},
		{"record in the namespace", "default", o.node, "vol-rec", "forget", 409, orphanErrHas},
		{"record in the namespace, delete", "default", o.node, "vol-rec", "delete", 409, orphanErrHas},
		{"record on another node", "default", o.node, "vol-rec-b", "forget", 409, orphanErrHas},
		{"record in another namespace", "default", o.node, "vol-rec-o", "delete", 409, orphanErrHas},
		{"a live allocation mounts it", "default", o.node, "vol-busy", "delete", 409, hostVolumeErrInUse},
		{"a live allocation mounts it, forget", "default", o.node, "vol-busy", "forget", 409, hostVolumeErrInUse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &orphanEnv{dhvEnv: e.with(t), node: o.node}
			raftBefore := o.s.raft.LastIndex()
			volIdx, volumes := o.snapshotTable()

			err := o.removeOrphan("", tc.ns, tc.node, tc.id, tc.mode)
			mustCoded(t, err, tc.code, tc.msg)
			o.mustNoClientCall()
			must.Eq(t, raftBefore, o.s.raft.LastIndex(), must.Sprint("a refusal wrote to Raft"))
			afterIdx, after := o.snapshotTable()
			must.Eq(t, volIdx, afterIdx)
			must.Eq(t, volumes, after)
			must.False(t, o.s.dhvCoordinator.isInFlight(tc.ns+"/"+tc.id))
		})
	}

	t.Run("a stopped allocation does not hold the volume", func(t *testing.T) {
		e := e.with(t)
		e.advertise(o.node, advertised("vol-done", "done", "default"))
		e.use(o.node, "done", structs.AllocClientStatusComplete)
		must.NoError(t, e.removeOrphan("", "default", o.node, "vol-done", "forget"))
		must.Eq(t, 1, e.mock.forgetCount())
	})
}

func TestHostVolumeRemoveOrphan_Modes(t *testing.T) {
	ci.Parallel(t)

	t.Run("delete runs the plugin delete once", func(t *testing.T) {
		o := newOrphanEnv(t)
		e := o.dhvEnv
		e.advertise(o.node, advertised("vol-orph", "orph", "default"))
		raftBefore := e.s.raft.LastIndex()
		volIdx, volumes := e.snapshotTable()

		must.NoError(t, e.removeOrphan("", "default", o.node, "vol-orph", "delete"))
		d, r, f := o.counts()
		must.Eq(t, []int{1, 0, 0}, []int{d, r, f})
		first, _ := e.mock.firstRemovals()
		must.Eq(t, cstructs.HostVolumeDeleteRequest{VolumeID: "vol-orph", Namespace: "default", NodeID: o.node}, first)
		must.Eq(t, raftBefore, e.s.raft.LastIndex(), must.Sprint("an orphan removal wrote to Raft"))
		afterIdx, after := e.snapshotTable()
		must.Eq(t, volIdx, afterIdx)
		must.Eq(t, volumes, after)
		must.False(t, e.s.dhvCoordinator.isInFlight("default/vol-orph"))
	})

	t.Run("forget runs no plugin", func(t *testing.T) {
		o := newOrphanEnv(t)
		e := o.dhvEnv
		e.advertise(o.node, advertised("vol-orph", "orph", "default"))
		raftBefore := e.s.raft.LastIndex()

		must.NoError(t, e.removeOrphan("", "default", o.node, "vol-orph", "forget"))
		d, r, f := o.counts()
		must.Eq(t, []int{0, 0, 1}, []int{d, r, f})
		e.mock.mu.Lock()
		got := e.mock.forgets[0]
		e.mock.mu.Unlock()
		must.Eq(t, cstructs.HostVolumeForgetRequest{VolumeID: "vol-orph", Namespace: "default", NodeID: o.node}, got)
		must.Eq(t, raftBefore, e.s.raft.LastIndex())
	})

	t.Run("the namespace of an old client's orphan is the request namespace", func(t *testing.T) {
		o := newOrphanEnv(t)
		e := o.dhvEnv
		e.advertise(o.node, advertised("vol-old", "old", ""))
		must.NoError(t, e.removeOrphan("", "other", o.node, "vol-old", "forget"))
		e.mock.mu.Lock()
		got := e.mock.forgets[0]
		e.mock.mu.Unlock()
		must.Eq(t, "other", got.Namespace)
	})

	t.Run("a caller that may not remove is 403 whatever the mode", func(t *testing.T) {
		o := newOrphanEnv(t)
		e := o.dhvEnv
		e.advertise(o.node, advertised("vol-orph", "orph", "default"))
		for _, mode := range []string{"", "remove", "Delete"} {
			for _, secret := range []string{"-", "00000000-0000-0000-0000-000000000000"} {
				must.EqError(t, e.removeOrphan(secret, "default", o.node, "vol-orph", mode), structs.ErrPermissionDenied.Error())
			}
		}
		o.mustNoClientCall()
	})

	t.Run("an old client without the in-use check is covered by the server's own", func(t *testing.T) {
		o := newOrphanEnv(t)
		e := o.dhvEnv
		e.advertise(o.node, advertised("vol-old", "old", ""))
		e.use(o.node, "old", structs.AllocClientStatusRunning)
		e.advertise(o.node, advertised("vol-old", "old", ""))
		mustCoded(t, e.removeOrphan("", "default", o.node, "vol-old", "delete"), 409, hostVolumeErrInUse)
		o.mustNoClientCall()
	})
}

func TestHostVolumeRemoveOrphan_ClientAnswers(t *testing.T) {
	ci.Parallel(t)

	reject := func(reason string) func(m *mockClientHostVolume) {
		return func(m *mockClientHostVolume) {
			m.deleteReply = func(req *cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
				return cstructs.HostVolumeDeleteResponse{VolumeID: req.VolumeID, Reason: reason, Message: "/synthetic/secret/path"}
			}
			m.forgetReply = func(req *cstructs.HostVolumeForgetRequest) cstructs.HostVolumeForgetResponse {
				return cstructs.HostVolumeForgetResponse{VolumeID: req.VolumeID, Reason: reason, Message: "/synthetic/secret/path"}
			}
		}
	}
	cases := []struct {
		name, mode string
		configure  func(*mockClientHostVolume)
		code       int
		msg        string
	}{
		{"registered volume in delete mode", "delete", reject("volume_kind_conflict"), 400, orphanErrReg},
		{"volume in use on the client, delete", "delete", reject("volume_in_use"), 409, hostVolumeErrInUse},
		{"volume in use on the client, forget", "forget", reject("volume_in_use"), 409, hostVolumeErrInUse},
		{"wrong namespace for an old client's orphan", "delete", reject("delete_mismatch"), 400, orphanErrNS},
		{"wrong namespace, forget", "forget", reject("delete_mismatch"), 400, orphanErrNS},
		{"plugin failure", "delete", reject("nonzero_exit"), 502, "client rejected: nonzero_exit"},
		{"store failure, forget", "forget", reject("invalid_record"), 502, "client rejected: invalid_record"},
		{"unlisted reason", "delete", reject("/synthetic/secret/path reason"), 502, "client rejected: client_rejected"},
		{"transport failure, delete", "delete", func(m *mockClientHostVolume) { m.err = errors.New("dial tcp 192.0.2.1: /synthetic/secret/path") }, 502, hostVolumeErrClientCall},
		{"transport failure, forget", "forget", func(m *mockClientHostVolume) { m.err = errors.New("Unknown rpc method: HostVolume.Forget") }, 502, hostVolumeErrClientCall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := newOrphanEnv(t)
			e := o.dhvEnv
			e.advertise(o.node, advertised("vol-orph", "orph", "default"))
			e.mock.set(tc.configure)
			raftBefore := e.s.raft.LastIndex()

			err := e.removeOrphan("", "default", o.node, "vol-orph", tc.mode)
			mustCoded(t, err, tc.code, tc.msg)
			must.Eq(t, raftBefore, e.s.raft.LastIndex())
			must.False(t, e.s.dhvCoordinator.isInFlight("default/vol-orph"))
			// A repeat is accepted once the client is well again.
			e.mock.set(func(m *mockClientHostVolume) { m.err, m.deleteReply, m.forgetReply = nil, nil, nil })
			must.NoError(t, e.removeOrphan("", "default", o.node, "vol-orph", tc.mode))
		})
	}
}

func TestHostVolumeRemoveOrphan_ACL(t *testing.T) {
	ci.Parallel(t)
	o := newOrphanEnv(t)
	e := o.dhvEnv

	pol := func(i int, rules ...string) string {
		return mock.CreatePolicyAndToken(t, e.s.State(), uint64(3000+i*2), fmt.Sprintf("orm%d", i), strings.Join(rules, "\n")).SecretID
	}
	ns := func(name string, caps ...string) string { return mock.NamespacePolicy(name, "", caps) }
	cases := []struct {
		name   string
		secret string
		allow  bool
	}{
		{"management", "", true},
		{"delete capability", pol(1, ns("default", acl.NamespaceCapabilityHostVolumeDelete)), true},
		{"write capability", pol(2, ns("default", acl.NamespaceCapabilityHostVolumeWrite)), true},
		{"policy write", pol(3, mock.NamespacePolicy("default", acl.PolicyWrite, nil)), true},
		{"read capability", pol(4, ns("default", acl.NamespaceCapabilityHostVolumeRead)), false},
		{"create and register", pol(5, ns("default", acl.NamespaceCapabilityHostVolumeCreate, acl.NamespaceCapabilityHostVolumeRegister)), false},
		{"delete in another namespace", pol(6, ns("other", acl.NamespaceCapabilityHostVolumeDelete)), false},
		{"node write alone", pol(7, mock.NodePolicy(acl.PolicyWrite)), false},
		{"deny overrides an allow", pol(8, mock.NamespacePolicy("default", acl.PolicyDeny, []string{acl.NamespaceCapabilityHostVolumeDelete})), false},
		{"no token", "-", false},
		{"unknown token", "00000000-0000-0000-0000-000000000000", false},
	}
	for _, tc := range cases {
		for _, mode := range []string{"delete", "forget"} {
			t.Run(tc.name+" "+mode, func(t *testing.T) {
				o := &orphanEnv{dhvEnv: e.with(t), node: o.node}
				o.advertise(o.node, advertised("vol-orph", "orph", "default"))
				d0, r0, f0 := o.counts()

				err := o.removeOrphan(tc.secret, "default", o.node, "vol-orph", mode)
				d1, r1, f1 := o.counts()
				if tc.allow {
					must.NoError(t, err)
					must.Eq(t, d0+r0+f0+1, d1+r1+f1)
					return
				}
				must.EqError(t, err, structs.ErrPermissionDenied.Error())
				must.Eq(t, d0+r0+f0, d1+r1+f1, must.Sprint("a denied call reached the client"))
			})
		}
	}
}

func TestHostVolumeRemoveOrphan_Audit(t *testing.T) {
	ci.Parallel(t)
	a := newAuditEnv(t, true)
	e := a.dhvEnv
	must.NoError(t, e.s.State().UpsertNamespaces(50, []*structs.Namespace{{Name: "other"}}))
	e.seedAll(dhvSeedVolume("vol-rec", "r1", e.nodeID, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated))
	reader := mock.CreatePolicyAndToken(t, e.s.State(), 5000, "orphan-audit-read",
		mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeRead}))

	adv := func() {
		e.advertise(e.nodeID,
			advertised("vol-orph", "orph", "default"),
			advertised("vol-rec", "r1", "default"),
			advertised("vol-busy", "busy", "default"))
	}
	adv()
	e.use(e.nodeID, "busy", structs.AllocClientStatusRunning)
	adv()
	root := a.accessor

	rejectDelete := func(reason string) func(m *mockClientHostVolume) {
		return func(m *mockClientHostVolume) {
			m.deleteReply = func(req *cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
				return cstructs.HostVolumeDeleteResponse{VolumeID: req.VolumeID, Reason: reason}
			}
		}
	}
	reset := func(m *mockClientHostVolume) { m.deleteReply, m.forgetReply, m.err = nil, nil, nil }

	cases := []struct {
		name      string
		secret    string
		ns, id    string
		mode      string
		configure func(*mockClientHostVolume)
		want      auditWant
	}{
		{"delete", "", "default", "vol-orph", "delete", nil,
			auditWant{actor: root, ns: "default", op: "orphan_delete", outcome: "ok", volumeID: "vol-orph"}},
		{"forget", "", "default", "vol-orph", "forget", nil,
			auditWant{actor: root, ns: "default", op: "orphan_forget", outcome: "ok", volumeID: "vol-orph"}},
		{"denied", reader.SecretID, "default", "vol-orph", "forget", nil,
			auditWant{actor: reader.AccessorID, ns: "default", op: "orphan_forget", outcome: "denied", volumeID: "vol-orph"}},
		{"not advertised", "", "default", "vol-nothing", "delete", nil,
			auditWant{actor: root, ns: "default", op: "orphan_delete", outcome: "not_found", volumeID: "vol-nothing"}},
		{"has a record", "", "default", "vol-rec", "forget", nil,
			auditWant{actor: root, ns: "default", op: "orphan_forget", outcome: "conflict", volumeID: "vol-rec"}},
		{"in use", "", "default", "vol-busy", "delete", nil,
			auditWant{actor: root, ns: "default", op: "orphan_delete", outcome: "in_use", volumeID: "vol-busy"}},
		{"registered in delete mode", "", "default", "vol-orph", "delete", rejectDelete("volume_kind_conflict"),
			auditWant{actor: root, ns: "default", op: "orphan_delete", outcome: "invalid", volumeID: "vol-orph"}},
		{"client rejected", "", "default", "vol-orph", "delete", rejectDelete("nonzero_exit"),
			auditWant{actor: root, ns: "default", op: "orphan_delete", outcome: "client_rejected", reason: "nonzero_exit", volumeID: "vol-orph"}},
		{"client unreachable", "", "default", "vol-orph", "forget", func(m *mockClientHostVolume) { m.err = errors.New("synthetic") },
			auditWant{actor: root, ns: "default", op: "orphan_forget", outcome: "client_unreachable", volumeID: "vol-orph"}},
		{"wildcard namespace", "", "*", "vol-orph", "forget", nil,
			auditWant{actor: root, ns: "*", op: "orphan_forget", outcome: "invalid", volumeID: "vol-orph"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e.mock.set(reset)
			if tc.configure != nil {
				e.mock.set(tc.configure)
			}
			line := a.one(t, func() {
				_ = e.removeOrphan(tc.secret, tc.ns, e.nodeID, tc.id, tc.mode)
			})
			mustAuditLine(t, line, tc.want)
		})
	}

	// A call that names no valid mode cannot name its operation. It removes
	// nothing and is refused before the audit line is set up, so it writes
	// none.
	for _, mode := range []string{"", "remove", "Delete"} {
		t.Run("no line for mode "+mode, func(t *testing.T) {
			e.mock.set(reset)
			before := len(auditLines(t, a.logs))
			mustCoded(t, e.removeOrphan("", "default", e.nodeID, "vol-orph", mode), 400, orphanErrMode)
			must.Eq(t, before, len(auditLines(t, a.logs)))
		})
	}
}

// A record that appears after the first check and before the client call makes
// the volume no orphan: the removal is refused and the client is not called.
func TestHostVolumeRemoveOrphan_RecordAfterTheCheck(t *testing.T) {
	ci.Parallel(t)
	o := newOrphanEnv(t)
	e := o.dhvEnv
	e.advertise(o.node, advertised("vol-orph", "orph", "default"))

	e.s.dhvCoordinator.setHook(func(point string) {
		if point == "orphan-checked" {
			e.seedVolumesAt(nextDHVIndex(), dhvSeedVolume("vol-orph", "orph", o.node,
				structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated))
		}
	})
	mustCoded(t, e.removeOrphan("", "default", o.node, "vol-orph", "delete"), 409, orphanErrHas)
	o.mustNoClientCall()
}

// The volume is marked in flight for the whole call, so a second removal of
// the same ID is refused while the first one waits on the client.
func TestHostVolumeRemoveOrphan_InFlight(t *testing.T) {
	ci.Parallel(t)
	o := newOrphanEnv(t)
	e := o.dhvEnv
	e.advertise(o.node, advertised("vol-orph", "orph", "default"))

	entered, release := make(chan struct{}), make(chan struct{})
	e.mock.set(func(m *mockClientHostVolume) {
		m.hook = func() {
			select {
			case entered <- struct{}{}:
				<-release
			default:
			}
		}
	})
	first := make(chan error, 1)
	go func() { first <- e.removeOrphan("", "default", o.node, "vol-orph", "forget") }()
	<-entered

	mustCoded(t, e.removeOrphan("", "default", o.node, "vol-orph", "delete"), 409, hostVolumeErrInProgress)
	close(release)
	must.NoError(t, <-first)
	must.False(t, e.s.dhvCoordinator.isInFlight("default/vol-orph"))
}

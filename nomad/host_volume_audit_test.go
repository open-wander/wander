// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/open-wander/wander/acl"
	"github.com/open-wander/wander/ci"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// The audit lines of the volume operations are written under this logger name.
// The capture logger is named "nomad", as the agent's server logger is.
const dhvAuditModule = "nomad.dynamic_host_volume.audit"

// auditLogger returns a server config callback that sends the server log to
// the buffer as JSON, one object per line.
func auditLogger(logs *lockedBuffer) func(*Config) {
	return func(c *Config) {
		c.Logger = hclog.NewInterceptLogger(&hclog.LoggerOptions{
			Name: "nomad", Output: logs, Level: hclog.Trace, JSONFormat: true,
		})
	}
}

// auditLines returns the audit lines in the buffer, in order, and fails the
// test on a line that is not JSON.
func auditLines(t *testing.T, logs *lockedBuffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, raw := range strings.Split(logs.String(), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var line map[string]any
		must.NoError(t, json.Unmarshal([]byte(raw), &line), must.Sprintf("line is not JSON: %q", raw))
		if line["@module"] == dhvAuditModule {
			out = append(out, line)
		}
	}
	return out
}

// auditWant is the content of one audit line. volumeID "*" accepts any
// non-empty ID; outcome accepts alternatives separated by "|".
type auditWant struct {
	actor, ns, op, outcome string
	volumeID, claimID      string
	reason                 string
	force                  bool
}

// auditClosedOutcomes is the closed set of outcome codes.
var auditClosedOutcomes = []string{
	"ok", "ready", "failed", "removed", "denied", "invalid", "not_found", "in_use", "in_progress",
	"conflict", "version_gate", "client_rejected", "client_unreachable", "outcome_not_recorded", "error",
}

func mustAuditLine(t *testing.T, line map[string]any, w auditWant) {
	t.Helper()
	must.Eq(t, "info", line["@level"])
	got := map[string]any{}
	for k, v := range line {
		switch k {
		case "@timestamp", "@level", "@message", "@module":
		default:
			got[k] = v
		}
	}
	outcome, _ := got["outcome"].(string)
	must.SliceContains(t, auditClosedOutcomes, outcome, must.Sprint("outcome is not in the closed set"))
	must.SliceContains(t, strings.Split(w.outcome, "|"), outcome)

	want := map[string]any{"actor": w.actor, "namespace": w.ns, "operation": w.op, "outcome": outcome}
	if w.op == "delete_claim" {
		want["claim_id"] = w.claimID
	} else {
		want["volume_id"] = w.volumeID
		if w.volumeID == "*" {
			must.NotEq(t, "", got["volume_id"])
			want["volume_id"] = got["volume_id"]
		}
	}
	if w.reason != "" {
		want["reason"] = w.reason
	}
	if w.force {
		want["force"] = true
	}
	must.Eq(t, want, got)
}

type auditEnv struct {
	*dhvEnv
	logs     *lockedBuffer
	accessor string // accessor ID of the management token
}

// newAuditEnv is a server with a client node, whose log is captured. The
// leader's reconciler is stopped so that only the calls of the test write.
func newAuditEnv(t *testing.T, withACL bool) *auditEnv {
	t.Helper()
	logs := &lockedBuffer{}
	e := &dhvEnv{t: t, mock: &mockClientHostVolume{}, seed: 1}
	a := &auditEnv{dhvEnv: e, logs: logs}
	var cleanup func()
	if withACL {
		var tok *structs.ACLToken
		e.s, tok, cleanup = TestACLServer(t, auditLogger(logs))
		e.root, a.accessor = tok.SecretID, tok.AccessorID
	} else {
		e.s, cleanup = TestServer(t, auditLogger(logs))
	}
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, e.s.RPC)
	e.s.stopDynamicHostVolumeReconciler()
	c := startMockHostVolumeClient(t, e.s, e.mock)
	e.nodeID = c.NodeID()
	e.advertisePlugin()
	return a
}

// one runs the call and returns the single audit line it wrote.
func (a *auditEnv) one(t *testing.T, do func()) map[string]any {
	t.Helper()
	before := len(auditLines(t, a.logs))
	do()
	lines := auditLines(t, a.logs)
	must.Eq(t, before+1, len(lines), must.Sprint("a call must write exactly one audit line"))
	return lines[len(lines)-1]
}

func TestHostVolumeAudit_Operations(t *testing.T) {
	ci.Parallel(t)
	a := newAuditEnv(t, true)
	e := a.dhvEnv
	must.NoError(t, e.s.State().UpsertNamespaces(50, []*structs.Namespace{{Name: "other"}}))

	// Fixtures first: the table index only moves forward. The volume the
	// update changes sits on a node no client touches.
	ghost := uuid.Generate()
	down := e.stateNode(structs.NodeStatusDown)
	mk := func(id string, node string, origin structs.DynamicHostVolumeOrigin) *structs.DynamicHostVolume {
		return dhvSeedVolume(id, "n-"+id, node, structs.DynamicHostVolumeStateReady, origin)
	}
	created, reg := structs.DynamicHostVolumeOriginPluginCreated, structs.DynamicHostVolumeOriginRegistered
	e.seedAll(
		mk("upd", ghost, created), mk("del-ok", e.nodeID, created), mk("dereg-ok", e.nodeID, reg),
		mk("wrong-kind", e.nodeID, reg), mk("in-use", e.nodeID, created), mk("force-ok", down, created),
		mk("force-dereg", down, reg), mk("force-ready", e.nodeID, created), mk("rejected", e.nodeID, created),
		mk("unreachable", e.nodeID, created), mk("busy", e.nodeID, created),
	)

	readTok := mock.CreatePolicyAndToken(t, e.s.State(), 5000, "audit-read",
		mock.NamespacePolicy("default", "", []string{acl.NamespaceCapabilityHostVolumeRead}))
	const bogus = "00000000-0000-0000-0000-000000000000"

	type step struct {
		name string
		do   func(t *testing.T)
		want auditWant
	}
	root := a.accessor
	steps := []step{
		{"create ready", func(t *testing.T) {
			_, _, err := e.create(dhvReqVolume("c1"))
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "create", outcome: "ready", volumeID: "*"}},
		{"create rejected by the client is failed", func(t *testing.T) {
			e.mock.set(func(m *mockClientHostVolume) {
				m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
					return cstructs.HostVolumeCreateResponse{Reason: "plugin_failed", Message: "synthetic message"}
				}
			})
			defer e.mock.set(func(m *mockClientHostVolume) { m.createReply = nil })
			vol, _, err := e.create(dhvReqVolume("c2"))
			must.NoError(t, err)
			must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
		}, auditWant{actor: root, ns: "default", op: "create", outcome: "failed", volumeID: "*"}},
		{"create with the client unreachable", func(t *testing.T) {
			e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
			defer e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
			_, _, err := e.create(dhvReqVolume("c3"))
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "create", outcome: "client_unreachable", volumeID: "*"}},
		{"create with no eligible node is failed", func(t *testing.T) {
			req := dhvReqVolume("c7")
			req.PluginID = "absent"
			vol, _, err := e.create(req)
			must.NoError(t, err)
			must.Eq(t, structs.DynamicHostVolumeStateFailed, vol.State)
		}, auditWant{actor: root, ns: "default", op: "create", outcome: "failed", volumeID: "*"}},
		{"create invalid", func(t *testing.T) {
			_, _, err := e.create(dhvReqVolume(""))
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "create", outcome: "invalid", volumeID: ""}},
		{"create without the capability", func(t *testing.T) {
			_, _, err := e.call("HostVolume.Create", readTok.SecretID, "default", dhvReqVolume("c4"))
			must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())
		}, auditWant{actor: readTok.AccessorID, ns: "default", op: "create", outcome: "denied", volumeID: ""}},
		{"create with an unknown token", func(t *testing.T) {
			_, _, err := e.call("HostVolume.Create", bogus, "default", dhvReqVolume("c5"))
			must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())
		}, auditWant{actor: "unauthenticated", ns: "default", op: "create", outcome: "denied", volumeID: ""}},
		{"create without a token", func(t *testing.T) {
			var resp structs.HostVolumeCreateResponse
			err := e.rpc("HostVolume.Create", &structs.HostVolumeCreateRequest{
				Volume: dhvReqVolume("c6"), WriteRequest: structs.WriteRequest{Region: "global", Namespace: "other"}}, &resp)
			must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())
		}, auditWant{actor: "anonymous", ns: "other", op: "create", outcome: "denied", volumeID: ""}},
		{"register ready", func(t *testing.T) {
			_, _, err := e.register(dhvRegVolume("r1", e.nodeID, "/synthetic/r1"))
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "register", outcome: "ready", volumeID: "*"}},
		{"register invalid", func(t *testing.T) {
			_, _, err := e.register(dhvRegVolume("r2", e.nodeID, ""))
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "register", outcome: "invalid", volumeID: ""}},
		{"update", func(t *testing.T) {
			v := dhvReqVolume("n-upd")
			v.ID = "upd"
			v.Parameters = map[string]string{"k": "new"}
			_, _, err := e.create(v)
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "update", outcome: "ready", volumeID: "upd"}},
		{"update of an unknown volume", func(t *testing.T) {
			v := dhvReqVolume("n-none")
			v.ID = "none"
			_, _, err := e.create(v)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "update", outcome: "not_found", volumeID: "none"}},
		{"delete", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, "", "default", "del-ok", false)
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "ok", volumeID: "del-ok"}},
		{"deregister", func(t *testing.T) {
			_, err := e.remove(dhvMethodDeregister, "", "default", "dereg-ok", false)
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "deregister", outcome: "ok", volumeID: "dereg-ok"}},
		{"delete of an unknown volume", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, "", "default", "missing", false)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "not_found", volumeID: "missing"}},
		{"delete of a registered volume", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, "", "default", "wrong-kind", false)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "invalid", volumeID: "wrong-kind"}},
		{"delete of a volume in use", func(t *testing.T) {
			// The allocation is written right before the call: a live client
			// can move an earlier one out of pending.
			e.use(e.nodeID, "n-in-use", structs.AllocClientStatusPending)
			_, err := e.remove(dhvMethodDelete, "", "default", "in-use", false)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "in_use", volumeID: "in-use"}},
		{"delete of a volume with an operation running", func(t *testing.T) {
			must.True(t, e.s.dhvCoordinator.begin("default/busy"))
			defer e.s.dhvCoordinator.end("default/busy")
			_, err := e.remove(dhvMethodDelete, "", "default", "busy", false)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "in_progress", volumeID: "busy"}},
		{"forced delete", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, "", "default", "force-ok", true)
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "ok", volumeID: "force-ok", force: true}},
		{"forced deregister", func(t *testing.T) {
			_, err := e.remove(dhvMethodDeregister, "", "default", "force-dereg", true)
			must.NoError(t, err)
		}, auditWant{actor: root, ns: "default", op: "deregister", outcome: "ok", volumeID: "force-dereg", force: true}},
		{"forced delete refused on a live node", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, "", "default", "force-ready", true)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "invalid", volumeID: "force-ready", force: true}},
		{"delete rejected by the client", func(t *testing.T) {
			e.mock.set(func(m *mockClientHostVolume) {
				m.deleteReply = func(*cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
					return cstructs.HostVolumeDeleteResponse{Reason: "nonzero_exit"}
				}
			})
			defer e.mock.set(func(m *mockClientHostVolume) { m.deleteReply = nil })
			_, err := e.remove(dhvMethodDelete, "", "default", "rejected", false)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "client_rejected", reason: "nonzero_exit", volumeID: "rejected"}},
		{"delete with the client unreachable", func(t *testing.T) {
			e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
			defer e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
			_, err := e.remove(dhvMethodDelete, "", "default", "unreachable", false)
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "delete", outcome: "client_unreachable", volumeID: "unreachable"}},
		{"delete without the capability", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, readTok.SecretID, "default", "busy", false)
			must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())
		}, auditWant{actor: readTok.AccessorID, ns: "default", op: "delete", outcome: "denied", volumeID: "busy"}},
		{"forced delete without the capability", func(t *testing.T) {
			_, err := e.remove(dhvMethodDelete, "-", "default", "busy", true)
			must.ErrorContains(t, err, structs.ErrPermissionDenied.Error())
		}, auditWant{actor: "anonymous", ns: "default", op: "delete", outcome: "denied", volumeID: "busy", force: true}},
		{"create refused by the version gate", func(t *testing.T) {
			advertiseWanderVersion(t, e.s, "0.1.1")
			defer advertiseWanderVersion(t, e.s, e.s.config.WanderVersion)
			_, _, err := e.create(dhvReqVolume("gated"))
			must.Error(t, err)
		}, auditWant{actor: root, ns: "default", op: "create", outcome: "version_gate", volumeID: ""}},
	}
	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			line := a.one(t, func() { st.do(t) })
			mustAuditLine(t, line, st.want)
		})
	}
}

func TestHostVolumeAudit_ClaimDelete(t *testing.T) {
	ci.Parallel(t)
	logs := &lockedBuffer{}
	s, cleanup := TestServer(t, auditLogger(logs))
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)
	s.stopDynamicHostVolumeReconciler()
	seedClaims(t, s,
		claimSpec{ns: "default", id: "held", job: "job-a", group: "g", volume: "data-a", live: true},
		claimSpec{ns: "default", id: "free", job: "job-b", group: "g", volume: "data-b"},
	)
	codec := rpcClient(t, s)

	for _, tc := range []struct {
		id      string
		wantErr bool
		outcome string
	}{
		{"free", false, "ok"},
		{"held", true, "in_use"},
		{"missing", true, "not_found"},
		{"", true, "invalid"},
	} {
		t.Run("claim "+tc.id, func(t *testing.T) {
			before := len(auditLines(t, logs))
			_, err := deleteClaim(codec, "default", tc.id, "")
			must.Eq(t, tc.wantErr, err != nil)
			lines := auditLines(t, logs)
			must.Eq(t, before+1, len(lines))
			// ACLs are off, so the caller is the management identity.
			mustAuditLine(t, lines[len(lines)-1], auditWant{
				actor: "management", ns: "default", op: "delete_claim", outcome: tc.outcome, claimID: tc.id})
		})
	}
}

// A call that a follower forwards is logged once, by the server that runs it.
func TestHostVolumeAudit_ForwardedLogsOnce(t *testing.T) {
	ci.Parallel(t)
	logs1, logs2 := &lockedBuffer{}, &lockedBuffer{}
	s1, cleanup1 := TestServer(t, auditLogger(logs1))
	defer cleanup1()
	s2, cleanup2 := TestServer(t, func(c *Config) {
		auditLogger(logs2)(c)
		c.BootstrapExpect = 0
	})
	defer cleanup2()
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForResult(func() (bool, error) {
		return len(s1.serf.Members()) == 2 && len(s1.raft.GetConfiguration().Configuration().Servers) == 2, fmt.Errorf("not joined")
	}, func(err error) { must.NoError(t, err) })

	leader, follower, leaderLogs, followerLogs := s1, s2, logs1, logs2
	if s2.IsLeader() {
		leader, follower, leaderLogs, followerLogs = s2, s1, logs2, logs1
	}
	leader.stopDynamicHostVolumeReconciler()
	follower.stopDynamicHostVolumeReconciler()
	seedClaims(t, leader, claimSpec{ns: "default", id: "free", job: "job-b", group: "g", volume: "data-b"})

	codec := rpcClient(t, follower)
	_, err := deleteClaim(codec, "default", "free", "")
	must.NoError(t, err)
	_, err = deleteClaim(codec, "default", "missing", "")
	must.Error(t, err)
	var resp structs.HostVolumeDeleteResponse
	err = msgpackrpc.CallWithCodec(codec, "HostVolume.Delete", &structs.HostVolumeDeleteRequest{
		VolumeID: "missing", WriteRequest: structs.WriteRequest{Region: "global", Namespace: "default"}}, &resp)
	must.Error(t, err)

	must.Len(t, 0, auditLines(t, followerLogs), must.Sprint("the follower must not log a forwarded call"))
	lines := auditLines(t, leaderLogs)
	must.Len(t, 3, lines)
	mustAuditLine(t, lines[0], auditWant{actor: "management", ns: "default", op: "delete_claim", outcome: "ok", claimID: "free"})
	mustAuditLine(t, lines[1], auditWant{actor: "management", ns: "default", op: "delete_claim", outcome: "not_found", claimID: "missing"})
	mustAuditLine(t, lines[2], auditWant{actor: "management", ns: "default", op: "delete", outcome: "not_found", volumeID: "missing"})
}

// A user-supplied ID is a value of the line: hclog quotes it, so it cannot end
// the line or start another.
func TestHostVolumeAudit_IDCannotForgeALine(t *testing.T) {
	ci.Parallel(t)
	logs := &lockedBuffer{}
	s, cleanup := TestServer(t, func(c *Config) {
		c.Logger = hclog.NewInterceptLogger(&hclog.LoggerOptions{Name: "nomad", Output: logs, Level: hclog.Trace})
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)
	s.stopDynamicHostVolumeReconciler()
	codec := rpcClient(t, s)

	forged := "x\n2026-01-01T00:00:00.000Z [INFO]  nomad.dynamic_host_volume.audit: volume operation: actor=forged"
	_, err := deleteClaim(codec, "default", forged, "")
	must.Error(t, err)

	// hclog writes a value with a newline as continuation lines, each led by
	// "  | ", so the forged text never starts a line of its own.
	var audit, forged2 []string
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, "actor=forged") {
			forged2 = append(forged2, l)
		}
		if strings.Contains(l, dhvAuditModule+":") && !strings.HasPrefix(l, "  | ") {
			audit = append(audit, l)
		}
	}
	must.Len(t, 1, audit, must.Sprint("the forged text must not start a second line"))
	must.StrContains(t, audit[0], "actor=management")
	must.Len(t, 1, forged2)
	must.StrHasPrefix(t, "  | ", forged2[0])
}

// The audit lines carry the fixed keys and nothing else: no token secret, no
// host path, no parameter value, no plugin output and no client message.
func TestHostVolumeAudit_LeaksNothing(t *testing.T) {
	ci.Parallel(t)
	a := newAuditEnv(t, true)
	e := a.dhvEnv
	const (
		path    = "/synthetic/secret-path"
		param   = "synthetic-param-value"
		output  = "synthetic-plugin-output"
		message = "synthetic-client-message"
	)
	down := e.stateNode(structs.NodeStatusDown)
	forceVol := dhvSeedVolume("force", "force", down, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	forceVol.HostPath = path
	delVol := dhvSeedVolume("del", "del", e.nodeID, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginPluginCreated)
	delVol.HostPath, delVol.Parameters = path, map[string]string{"k": param}
	regVol := dhvSeedVolume("dereg", "dereg", e.nodeID, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginRegistered)
	regVol.HostPath = path
	forceReg := dhvSeedVolume("force-dereg", "force-dereg", down, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeOriginRegistered)
	forceReg.HostPath = path
	e.seedAll(forceVol, delVol, regVol, forceReg)
	must.NoError(t, e.s.State().UpsertDynamicHostVolumeClaims(60, []*structs.DynamicHostVolumeClaim{{
		ID: "claim-leak", Namespace: "default", JobID: "job-leak", TaskGroupName: "g", VolumeName: "del",
		VolumeID: "del", AllocationID: uuid.Generate(), AllocationIndex: 1}}))

	e.mock.set(func(m *mockClientHostVolume) {
		m.createReply = func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse {
			return cstructs.HostVolumeCreateResponse{Reason: "plugin_failed", Message: message + " " + output + " " + path}
		}
		m.deleteReply = func(*cstructs.HostVolumeDeleteRequest) cstructs.HostVolumeDeleteResponse {
			return cstructs.HostVolumeDeleteResponse{Reason: message + " " + output + " " + path}
		}
		m.deregReply = func(*cstructs.HostVolumeDeregisterRequest) cstructs.HostVolumeDeregisterResponse {
			return cstructs.HostVolumeDeregisterResponse{Reason: message + " " + output + " " + path}
		}
	})

	cv := dhvReqVolume("leak")
	cv.Parameters = map[string]string{"k": param}
	_, _, _ = e.create(cv)
	e.mock.set(func(m *mockClientHostVolume) { m.createReply = nil })
	rv := dhvRegVolume("leak-reg", e.nodeID, path)
	rv.Parameters = map[string]string{"k": param}
	_, _, _ = e.register(rv)
	uv := dhvReqVolume("n-leak")
	uv.ID, uv.Parameters = "none", map[string]string{"k": param}
	_, _, _ = e.create(uv)
	_, _ = e.remove(dhvMethodDelete, "", "default", "del", false)
	_, _ = e.remove(dhvMethodDeregister, "", "default", "dereg", false)
	_, _ = e.remove(dhvMethodDelete, "", "default", "force", true)
	_, _ = e.remove(dhvMethodDeregister, "", "default", "force-dereg", true)
	_, _ = deleteClaim(rpcClient(t, e.s), "default", "claim-leak", e.root)
	_, _ = e.remove(dhvMethodDelete, e.root+"-bad", "default", "del", false)
	_, _ = e.remove(dhvMethodDelete, "00000000-0000-0000-0000-000000000000", "default", "del", false)

	lines := auditLines(t, a.logs)
	must.Len(t, 10, lines)
	allowed := []string{"@timestamp", "@level", "@message", "@module", "actor", "namespace", "volume_id",
		"claim_id", "operation", "outcome", "reason", "force"}
	for _, l := range lines {
		for k := range l {
			must.SliceContains(t, allowed, k, must.Sprintf("unexpected key %q in an audit line", k))
		}
		if r, ok := l["reason"]; ok {
			must.SliceContains(t, append(hostVolumeClientReasons, hostVolumeReasonOther), fmt.Sprint(r))
		}
	}
	// The whole captured log is searched, not only the audit lines.
	out := a.logs.String()
	for _, secret := range []string{e.root, path, "secret-path", param, output, message} {
		must.StrNotContains(t, out, secret)
	}
}

// The leader's reconciler logs once when it settles a volume, with the fixed
// actor, and never on an attempt that settles nothing.
func TestHostVolumeAudit_ReconcilerLogsOncePerSettlement(t *testing.T) {
	ci.Parallel(t)
	type kase struct {
		name    string
		state   structs.DynamicHostVolumeState
		origin  structs.DynamicHostVolumeOrigin
		op      string
		outcome string
	}
	for _, tc := range []kase{
		{"create settles ready", structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated, "create", "ready"},
		{"register settles ready", structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginRegistered, "register", "ready"},
		{"delete settles removed", structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginPluginCreated, "delete", "removed"},
		{"deregister settles removed", structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeOriginRegistered, "deregister", "removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAuditEnv(t, false)
			e := a.dhvEnv
			vol := dhvSeedVolume("vol-1", "data", e.nodeID, tc.state, tc.origin)
			vol.ModifyTime = time.Now().UnixNano()
			e.seedVolumes(vol)
			e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
			passes := e.reconcile(nil)
			waitPasses(t, passes, 6)
			must.Len(t, 0, auditLines(t, a.logs), must.Sprint("an attempt that settles nothing must not log"))

			e.mock.set(func(m *mockClientHostVolume) { m.err = nil })
			if tc.state == structs.DynamicHostVolumeStateDeleting {
				e.waitGone("vol-1")
			} else {
				e.waitState("vol-1", structs.DynamicHostVolumeStateReady)
			}
			waitPasses(t, passes, 6)
			lines := auditLines(t, a.logs)
			must.Len(t, 1, lines)
			mustAuditLine(t, lines[0], auditWant{actor: "leader", ns: "default", op: tc.op, outcome: tc.outcome, volumeID: "vol-1"})
		})
	}

	t.Run("a volume that keeps failing settles failed once", func(t *testing.T) {
		a := newAuditEnv(t, false)
		e := a.dhvEnv
		vol := dhvSeedVolume("vol-1", "data", e.nodeID, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeOriginPluginCreated)
		vol.ModifyTime = time.Now().UnixNano()
		e.seedVolumes(vol)
		e.mock.set(func(m *mockClientHostVolume) { m.err = errors.New("synthetic transport failure") })
		passes := e.reconcile(func(s *dynamicHostVolumeReconcileSettings) { s.Bound = 100 * time.Millisecond })
		e.waitState("vol-1", structs.DynamicHostVolumeStateFailed)
		waitPasses(t, passes, 6)
		lines := auditLines(t, a.logs)
		must.Len(t, 1, lines)
		mustAuditLine(t, lines[0], auditWant{actor: "leader", ns: "default", op: "create", outcome: "failed", volumeID: "vol-1"})
	})
}

func TestHostVolumeAuditOutcome(t *testing.T) {
	ci.Parallel(t)
	vol := func(st structs.DynamicHostVolumeState) *structs.DynamicHostVolume {
		return &structs.DynamicHostVolume{State: st}
	}
	for _, tc := range []struct {
		name    string
		err     error
		vol     *structs.DynamicHostVolume
		outcome string
		reason  string
	}{
		{"success", nil, nil, "ok", ""},
		{"ready volume", nil, vol(structs.DynamicHostVolumeStateReady), "ready", ""},
		{"failed volume", nil, vol(structs.DynamicHostVolumeStateFailed), "failed", ""},
		{"unavailable volume", nil, vol(structs.DynamicHostVolumeStateUnavailable), "ok", ""},
		{"denied", structs.ErrPermissionDenied, nil, "denied", ""},
		{"invalid", hostVolumeCoded(400, "invalid volume: Name: required"), nil, "invalid", ""},
		{"not found", hostVolumeCoded(404, hostVolumeErrNotFound), nil, "not_found", ""},
		{"in use", hostVolumeCoded(409, hostVolumeErrInUse), nil, "in_use", ""},
		{"claim in use", hostVolumeCoded(409, hostVolumeErrClaimInUse), nil, "in_use", ""},
		{"in progress", hostVolumeCoded(409, hostVolumeErrInProgress), nil, "in_progress", ""},
		{"concurrent change", hostVolumeCoded(409, hostVolumeErrConcurrent), nil, "conflict", ""},
		{"state conflict", hostVolumeStateError(409, "", structs.DynamicHostVolumeStateDeleting), nil, "conflict", ""},
		{"version gate", hostVolumeCoded(503, hostVolumeErrVersionGate), nil, "version_gate", ""},
		{"outcome not recorded", hostVolumeStateError(500, hostVolumeErrOutcome, structs.DynamicHostVolumeStatePending), nil, "outcome_not_recorded", ""},
		{"client unreachable", hostVolumeStateError(502, hostVolumeErrClientCall, structs.DynamicHostVolumeStateDeleting), nil, "client_unreachable", ""},
		{"client rejected with a listed reason", hostVolumeStateError(502, hostVolumeErrRejected+": nonzero_exit", structs.DynamicHostVolumeStateDeleting), nil, "client_rejected", "nonzero_exit"},
		{"client rejected with another reason", hostVolumeStateError(502, hostVolumeErrRejected+": /synthetic/path", structs.DynamicHostVolumeStateDeleting), nil, "client_rejected", hostVolumeReasonOther},
		{"uncoded error", errors.New("synthetic /synthetic/path"), nil, "error", ""},
		{"other code", hostVolumeCoded(418, "synthetic"), nil, "error", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome, reason := hostVolumeAuditOutcome(tc.err, tc.vol)
			must.Eq(t, tc.outcome, outcome)
			must.Eq(t, tc.reason, reason)
		})
	}
}

func TestHostVolumeAuditActor(t *testing.T) {
	ci.Parallel(t)
	for _, tc := range []struct {
		name    string
		id      *structs.AuthenticatedIdentity
		authErr error
		want    string
	}{
		{"token", &structs.AuthenticatedIdentity{ACLToken: &structs.ACLToken{AccessorID: "acc", SecretID: "sec"}}, nil, "acc"},
		{"anonymous", &structs.AuthenticatedIdentity{ACLToken: structs.AnonymousACLToken}, nil, "anonymous"},
		{"leader", &structs.AuthenticatedIdentity{ACLToken: structs.LeaderACLToken}, nil, hostVolumeActorLeader},
		{"acls disabled", &structs.AuthenticatedIdentity{}, nil, "management"},
		{"workload", &structs.AuthenticatedIdentity{Claims: &structs.IdentityClaims{AllocationID: "alloc-1"}}, nil, "workload:alloc-1"},
		{"node", &structs.AuthenticatedIdentity{ClientID: "node-1"}, nil, "client:node-1"},
		{"failed authentication", &structs.AuthenticatedIdentity{}, structs.ErrPermissionDenied, "unauthenticated"},
		{"no identity", nil, nil, "unauthenticated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			must.Eq(t, tc.want, hostVolumeAuditActor(tc.id, tc.authErr))
		})
	}
	must.Eq(t, structs.LeaderACLToken.AccessorID, hostVolumeActorLeader)
}

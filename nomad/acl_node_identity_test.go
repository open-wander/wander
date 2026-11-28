// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"net"
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

// TestResolveACL_NoNilWhenEnabled checks that, with ACLs enabled, an identity
// that carries neither a token nor claims resolves to a deny-all ACL rather
// than the nil ACL that means "ACLs are disabled".
func TestResolveACL_NoNilWhenEnabled(t *testing.T) {
	ci.Parallel(t)

	srv, mgmt, cleanup := TestACLServer(t, nil)
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)

	alloc := mock.Alloc()
	must.NoError(t, srv.State().UpsertAllocs(structs.MsgTypeTestSetup, 50, []*structs.Allocation{alloc}))

	disabled, cleanupDisabled := TestServer(t, nil)
	defer cleanupDisabled()
	testutil.WaitForLeader(t, disabled.RPC)

	cases := []struct {
		name         string
		identity     *structs.AuthenticatedIdentity
		enabled      bool
		wantNil      bool
		wantMgmt     bool
		wantClientOK bool // ResolveClientOrACL returns nil
	}{
		{name: "client id only", identity: &structs.AuthenticatedIdentity{ClientID: "node-1"}, enabled: true, wantClientOK: true},
		{name: "nil identity", identity: nil, enabled: true},
		{name: "remote ip only", identity: &structs.AuthenticatedIdentity{RemoteIP: net.ParseIP("10.0.0.1")}, enabled: true},
		{name: "empty identity", identity: &structs.AuthenticatedIdentity{}, enabled: true},
		{name: "management token", identity: &structs.AuthenticatedIdentity{ACLToken: mgmt}, enabled: true, wantMgmt: true},
		{name: "claims", identity: &structs.AuthenticatedIdentity{Claims: alloc.ToTaskIdentityClaims(nil, "web")}, enabled: true},
		{name: "disabled client id", identity: &structs.AuthenticatedIdentity{ClientID: "node-1"}, enabled: false, wantNil: true, wantClientOK: true},
		{name: "disabled nil identity", identity: nil, enabled: false, wantNil: true, wantClientOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := srv
			if !tc.enabled {
				s = disabled
			}
			args := &structs.JobListRequest{}
			args.SetIdentity(tc.identity)

			aclObj, err := s.ResolveACL(args)
			must.NoError(t, err)
			if tc.wantNil {
				must.Nil(t, aclObj)
			} else {
				must.NotNil(t, aclObj, must.Sprint("ACLs enabled must never resolve to a nil ACL"))
				must.Eq(t, tc.wantMgmt, aclObj.IsManagement())
				if !tc.wantMgmt {
					must.False(t, aclObj.AllowNsOp("default", acl.NamespaceCapabilityReadJob))
					must.False(t, aclObj.AllowNodeWrite())
					must.False(t, aclObj.AllowOperatorRead())
				}
			}

			clientObj, err := s.ResolveClientOrACL(args)
			must.NoError(t, err)
			if tc.wantClientOK || tc.wantNil {
				must.Nil(t, clientObj)
			} else {
				must.NotNil(t, clientObj)
			}
		})
	}
}

// TestACL_NodeSecretRefused checks that a client node's SecretID is refused by
// endpoints that authorise against an ACL, while ACLs are enabled.
func TestACL_NodeSecretRefused(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanup := TestACLServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	node := mock.Node()
	other := mock.Node()
	must.NoError(t, srv.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))
	must.NoError(t, srv.State().UpsertNode(structs.MsgTypeTestSetup, 101, other))

	write := structs.WriteRequest{Region: "global", Namespace: "default", AuthToken: node.SecretID}
	query := structs.QueryOptions{Region: "global", Namespace: "default", AuthToken: node.SecretID}

	job := mock.Job()
	must.NoError(t, srv.State().UpsertJob(structs.MsgTypeTestSetup, 102, nil, job))

	sv := mock.VariableEncrypted()
	sv.Namespace = "default"
	sv.Path = "a/b"
	must.NoError(t, srv.State().VarSet(103, &structs.VarApplyStateRequest{Op: structs.VarOpSet, Var: sv}).Error)

	cases := []struct {
		name   string
		method string
		args   any
		reply  any
	}{
		// This case guards the token check in Job.Register, which refuses a
		// node secret before any ACL check, not the deny-all ACL. The
		// write cases below (job deregister, variables apply, drain) are the
		// ones that depend on the ACL.
		{"job register", "Job.Register", &structs.JobRegisterRequest{Job: mock.Job(), WriteRequest: write}, &structs.JobRegisterResponse{}},
		{"job deregister", "Job.Deregister", &structs.JobDeregisterRequest{JobID: job.ID, WriteRequest: write}, &structs.JobDeregisterResponse{}},
		{"job get", "Job.GetJob", &structs.JobSpecificRequest{JobID: job.ID, QueryOptions: query}, &structs.SingleJobResponse{}},
		{"job list", "Job.List", &structs.JobListRequest{QueryOptions: query}, &structs.JobListResponse{}},
		{"scheduler config", "Operator.SchedulerGetConfiguration", &structs.GenericRequest{QueryOptions: query}, &structs.SchedulerConfigurationResponse{}},
		{"variables read", "Variables.Read", &structs.VariablesReadRequest{Path: "a/b", QueryOptions: query}, &structs.VariablesReadResponse{}},
		{"variables apply", "Variables.Apply", &structs.VariablesApplyRequest{Op: structs.VarOpSet, Var: mock.Variable(), WriteRequest: write}, &structs.VariablesApplyResponse{}},
		{"drain other node", "Node.UpdateDrain", &structs.NodeUpdateDrainRequest{
			NodeID:        other.ID,
			DrainStrategy: &structs.DrainStrategy{DrainSpec: structs.DrainSpec{Deadline: time.Hour}},
			WriteRequest:  write,
		}, &structs.NodeDrainUpdateResponse{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := msgpackrpc.CallWithCodec(codec, tc.method, tc.args, tc.reply)
			must.Error(t, err)
			if tc.method != "Job.Register" {
				must.EqError(t, err, structs.ErrPermissionDenied.Error())
			}
		})
	}

	// Variables.List filters per variable, so the node sees nothing rather
	// than an error.
	var list structs.VariablesListResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Variables.List",
		&structs.VariablesListRequest{QueryOptions: query}, &list))
	must.SliceEmpty(t, list.Data)

	// The refused drain must not have changed the other node.
	out, err := srv.State().NodeByID(nil, other.ID)
	must.NoError(t, err)
	must.Nil(t, out.DrainStrategy)
}

// TestNode_UpdateDrain_OwnNodeSecret checks that a client can drain itself
// with its own secret while ACLs are enabled.
func TestNode_UpdateDrain_OwnNodeSecret(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanup := TestACLServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	node := mock.Node()
	must.NoError(t, srv.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))

	req := &structs.NodeUpdateDrainRequest{
		NodeID:        node.ID,
		DrainStrategy: &structs.DrainStrategy{DrainSpec: structs.DrainSpec{Deadline: time.Hour}},
		WriteRequest:  structs.WriteRequest{Region: "global", AuthToken: node.SecretID},
	}
	var resp structs.NodeDrainUpdateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", req, &resp))

	out, err := srv.State().NodeByID(nil, node.ID)
	must.NoError(t, err)
	must.NotNil(t, out.DrainStrategy)
}

// TestServiceRegistration_DeleteByID_NodeSecret checks that a node may delete
// only the service registrations that belong to it.
func TestServiceRegistration_DeleteByID_NodeSecret(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanup := TestACLServer(t, nil)
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	owner := mock.Node()
	stranger := mock.Node()
	must.NoError(t, srv.State().UpsertNode(structs.MsgTypeTestSetup, 100, owner))
	must.NoError(t, srv.State().UpsertNode(structs.MsgTypeTestSetup, 101, stranger))

	services := mock.ServiceRegistrations()
	services[0].NodeID = owner.ID
	must.NoError(t, srv.State().UpsertServiceRegistrations(structs.MsgTypeTestSetup, 102, services))

	del := func(secret string) error {
		req := &structs.ServiceRegistrationDeleteByIDRequest{
			ID: services[0].ID,
			WriteRequest: structs.WriteRequest{
				Region: DefaultRegion, Namespace: services[0].Namespace, AuthToken: secret,
			},
		}
		var resp structs.ServiceRegistrationDeleteByIDResponse
		return msgpackrpc.CallWithCodec(codec, structs.ServiceRegistrationDeleteByIDRPCMethod, req, &resp)
	}

	must.EqError(t, del(stranger.SecretID), structs.ErrPermissionDenied.Error())
	out, err := srv.State().GetServiceRegistrationByID(nil, services[0].Namespace, services[0].ID)
	must.NoError(t, err)
	must.NotNil(t, out)

	must.NoError(t, del(owner.SecretID))
	out, err = srv.State().GetServiceRegistrationByID(nil, services[0].Namespace, services[0].ID)
	must.NoError(t, err)
	must.Nil(t, out)
}

// TestNode_UpdateDrain_OwnNodeRestricted checks that a client node may only
// start a drain on itself: it may not clear a drain (which would undo an
// operator's drain) or mark itself eligible.
func TestNode_UpdateDrain_OwnNodeRestricted(t *testing.T) {
	ci.Parallel(t)

	srv, _, cleanup := TestACLServer(t, func(c *Config) { c.NumSchedulers = 0 })
	defer cleanup()
	testutil.WaitForLeader(t, srv.RPC)
	codec := rpcClient(t, srv)

	node := mock.Node()
	must.NoError(t, srv.State().UpsertNode(structs.MsgTypeTestSetup, 100, node))

	strategy := &structs.DrainStrategy{DrainSpec: structs.DrainSpec{Deadline: time.Hour}}
	must.NoError(t, srv.State().UpdateNodeDrain(structs.MsgTypeTestSetup, 101, node.ID,
		strategy, false, time.Now().Unix(), nil, nil, "operator"))

	// The node drainer may complete the drain at once, so compare the stored
	// node before and after rather than expecting the strategy to remain.
	testutil.WaitForResult(func() (bool, error) {
		n, err := srv.State().NodeByID(nil, node.ID)
		if err != nil {
			return false, err
		}
		return n.DrainStrategy == nil, nil
	}, func(err error) { must.NoError(t, err) })
	before, err := srv.State().NodeByID(nil, node.ID)
	must.NoError(t, err)
	must.Eq(t, structs.NodeSchedulingIneligible, before.SchedulingEligibility)

	cases := []struct {
		name         string
		strategy     *structs.DrainStrategy
		markEligible bool
	}{
		{name: "clear drain"},
		{name: "clear drain and mark eligible", markEligible: true},
		{name: "start drain and mark eligible", strategy: strategy, markEligible: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &structs.NodeUpdateDrainRequest{
				NodeID:        node.ID,
				DrainStrategy: tc.strategy,
				MarkEligible:  tc.markEligible,
				WriteRequest:  structs.WriteRequest{Region: "global", AuthToken: node.SecretID},
			}
			var resp structs.NodeDrainUpdateResponse
			err := msgpackrpc.CallWithCodec(codec, "Node.UpdateDrain", req, &resp)
			must.EqError(t, err, structs.ErrPermissionDenied.Error())

			out, err := srv.State().NodeByID(nil, node.ID)
			must.NoError(t, err)
			must.Eq(t, before.ModifyIndex, out.ModifyIndex)
			must.Eq(t, structs.NodeSchedulingIneligible, out.SchedulingEligibility)
		})
	}
}

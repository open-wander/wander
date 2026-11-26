// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"sync"
	"testing"
	"time"

	msgpackrpc "github.com/hashicorp/net-rpc-msgpackrpc"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client"
	"github.com/open-wander/wander/client/config"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/nomad/structs"
	sconfig "github.com/open-wander/wander/nomad/structs/config"
	"github.com/open-wander/wander/testutil"
	"github.com/shoenig/test/must"
)

// mockClientHostVolume stands in for the client HostVolume endpoint. It
// records the requests it receives and answers with fixed replies.
type mockClientHostVolume struct {
	mu       sync.Mutex
	creates  []cstructs.HostVolumeCreateRequest
	registry []cstructs.HostVolumeRegisterRequest
	deletes  []cstructs.HostVolumeDeleteRequest
	deregs   []cstructs.HostVolumeDeregisterRequest
	err      error

	// The fields below configure the mock for the server endpoint tests and
	// are left nil by the handshake tests. A reply function replaces the
	// default reply, and hook runs after the request is recorded with no lock
	// held, so a test can block a call or observe its timing.
	createReply   func(*cstructs.HostVolumeCreateRequest) cstructs.HostVolumeCreateResponse
	registerReply func(*cstructs.HostVolumeRegisterRequest) cstructs.HostVolumeRegisterResponse
	hook          func()
}

func (m *mockClientHostVolume) Create(req *cstructs.HostVolumeCreateRequest, resp *cstructs.HostVolumeCreateResponse) error {
	m.mu.Lock()
	m.creates = append(m.creates, *req)
	*resp = cstructs.HostVolumeCreateResponse{
		VolumeID: req.VolumeID, HostPath: "/synthetic/" + req.VolumeID, CapacityBytes: 4096, Status: "ready",
	}
	if m.createReply != nil {
		*resp = m.createReply(req)
	}
	hook, err := m.hook, m.err
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

func (m *mockClientHostVolume) Register(req *cstructs.HostVolumeRegisterRequest, resp *cstructs.HostVolumeRegisterResponse) error {
	m.mu.Lock()
	m.registry = append(m.registry, *req)
	*resp = cstructs.HostVolumeRegisterResponse{VolumeID: req.VolumeID, HostPath: req.HostPath, Status: "ready"}
	if m.registerReply != nil {
		*resp = m.registerReply(req)
	}
	hook, err := m.hook, m.err
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

func (m *mockClientHostVolume) Delete(req *cstructs.HostVolumeDeleteRequest, resp *cstructs.HostVolumeDeleteResponse) error {
	m.mu.Lock()
	m.deletes = append(m.deletes, *req)
	*resp = cstructs.HostVolumeDeleteResponse{VolumeID: req.VolumeID}
	hook, err := m.hook, m.err
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

func (m *mockClientHostVolume) Deregister(req *cstructs.HostVolumeDeregisterRequest, resp *cstructs.HostVolumeDeregisterResponse) error {
	m.mu.Lock()
	m.deregs = append(m.deregs, *req)
	*resp = cstructs.HostVolumeDeregisterResponse{VolumeID: req.VolumeID}
	hook, err := m.hook, m.err
	m.mu.Unlock()
	if hook != nil {
		hook()
	}
	return err
}

// removalCounts returns how many delete and deregister calls arrived.
func (m *mockClientHostVolume) removalCounts() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deletes), len(m.deregs)
}

// set changes the mock's configuration while the client may be serving calls.
func (m *mockClientHostVolume) set(f func(*mockClientHostVolume)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f(m)
}

func (m *mockClientHostVolume) counts() (int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.creates), len(m.registry)
}

func startMockHostVolumeClient(t *testing.T, s *Server, mock *mockClientHostVolume) *client.Client {
	t.Helper()
	c, cleanup := client.TestClientWithRPCs(t, func(c *config.Config) {
		c.Servers = []string{s.config.RPCAddr.String()}
	}, map[string]any{"HostVolume": mock})
	t.Cleanup(func() { _ = cleanup() })
	select {
	case <-c.Ready():
	case <-time.After(10 * time.Second):
		t.Fatal("client timed out on initialize")
	}
	return c
}

func waitForNodeInState(t *testing.T, s *Server, nodeID string) {
	t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		n, err := s.State().NodeByID(nil, nodeID)
		if err != nil {
			return false, err
		}
		if n == nil {
			return false, errors.New("node is not in the state store yet")
		}
		return true, nil
	}, func(err error) { must.NoError(t, err) })
}

func TestClientHostVolume_Local(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, func(c *Config) { c.BootstrapExpect = 1 })
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)
	mock := &mockClientHostVolume{}
	c := startMockHostVolumeClient(t, s, mock)
	waitForNodeInState(t, s, c.NodeID())
	codec := rpcClient(t, s)

	createReq := &cstructs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: c.NodeID(), PluginID: "plug",
	}
	var createResp cstructs.HostVolumeCreateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Create", createReq, &createResp))
	must.Eq(t, cstructs.HostVolumeCreateResponse{
		VolumeID: "vol-a", HostPath: "/synthetic/vol-a", CapacityBytes: 4096, Status: "ready",
	}, createResp)

	regReq := &cstructs.HostVolumeRegisterRequest{
		VolumeID: "vol-b", Namespace: "default", Name: "ext", NodeID: c.NodeID(), HostPath: "/synthetic/ext",
	}
	var regResp cstructs.HostVolumeRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Register", regReq, &regResp))
	must.Eq(t, "ready", regResp.Status)

	nc, nr := mock.counts()
	must.Eq(t, 1, nc)
	must.Eq(t, 1, nr)
	must.Eq(t, *createReq, mock.creates[0])
	must.Eq(t, *regReq, mock.registry[0])

	delReq := &cstructs.HostVolumeDeleteRequest{VolumeID: "vol-a", Namespace: "default", NodeID: c.NodeID()}
	var delResp cstructs.HostVolumeDeleteResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Delete", delReq, &delResp))
	must.Eq(t, cstructs.HostVolumeDeleteResponse{VolumeID: "vol-a"}, delResp)
	deregReq := &cstructs.HostVolumeDeregisterRequest{VolumeID: "vol-b", Namespace: "default", NodeID: c.NodeID()}
	var deregResp cstructs.HostVolumeDeregisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Deregister", deregReq, &deregResp))
	must.Eq(t, cstructs.HostVolumeDeregisterResponse{VolumeID: "vol-b"}, deregResp)
	nd, ng := mock.removalCounts()
	must.Eq(t, 1, nd)
	must.Eq(t, 1, ng)
	must.Eq(t, *delReq, mock.deletes[0])
	must.Eq(t, *deregReq, mock.deregs[0])

	t.Run("unknown node", func(t *testing.T) {
		req := *createReq
		req.NodeID = "5b0c0a5e-0000-4000-8000-000000000001"
		var r cstructs.HostVolumeCreateResponse
		err := msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Create", &req, &r)
		must.ErrorContains(t, err, "Unknown node")
	})

	t.Run("unknown node for delete and deregister", func(t *testing.T) {
		const missing = "5b0c0a5e-0000-4000-8000-000000000001"
		var d cstructs.HostVolumeDeleteResponse
		must.ErrorContains(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Delete",
			&cstructs.HostVolumeDeleteRequest{VolumeID: "vol-a", Namespace: "default", NodeID: missing}, &d), "Unknown node")
		var g cstructs.HostVolumeDeregisterResponse
		must.ErrorContains(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Deregister",
			&cstructs.HostVolumeDeregisterRequest{VolumeID: "vol-b", Namespace: "default", NodeID: missing}, &g), "Unknown node")
	})

	t.Run("client error", func(t *testing.T) {
		mock.mu.Lock()
		mock.err = errors.New("synthetic client failure")
		mock.mu.Unlock()
		var r cstructs.HostVolumeCreateResponse
		err := msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Create", createReq, &r)
		must.ErrorContains(t, err, "synthetic client failure")
		var d cstructs.HostVolumeDeleteResponse
		err = msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Delete", delReq, &d)
		must.ErrorContains(t, err, "synthetic client failure")
		var g cstructs.HostVolumeDeregisterResponse
		err = msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Deregister", deregReq, &g)
		must.ErrorContains(t, err, "synthetic client failure")
	})
}

// A client connected to another server is reached by forwarding.
func TestClientHostVolume_Forwarded(t *testing.T) {
	ci.Parallel(t)

	s1, cleanup1 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	t.Cleanup(cleanup1)
	s2, cleanup2 := TestServer(t, func(c *Config) { c.BootstrapExpect = 2 })
	t.Cleanup(cleanup2)
	TestJoin(t, s1, s2)
	testutil.WaitForLeader(t, s1.RPC)
	testutil.WaitForLeader(t, s2.RPC)

	mock := &mockClientHostVolume{}
	c := startMockHostVolumeClient(t, s2, mock)
	waitForNodeInState(t, s1, c.NodeID())
	waitForNodeInState(t, s2, c.NodeID())

	// Only s2 holds the connection.
	s1.nodeConnsLock.Lock()
	delete(s1.nodeConns, c.NodeID())
	s1.nodeConnsLock.Unlock()
	_, onS1 := s1.getNodeConn(c.NodeID())
	must.False(t, onS1)
	_, onS2 := s2.getNodeConn(c.NodeID())
	must.True(t, onS2)

	codec := rpcClient(t, s1)
	req := &cstructs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: c.NodeID(), PluginID: "plug",
	}
	var resp cstructs.HostVolumeCreateResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Create", req, &resp))
	must.Eq(t, "ready", resp.Status)
	must.Eq(t, "/synthetic/vol-a", resp.HostPath)

	regReq := &cstructs.HostVolumeRegisterRequest{
		VolumeID: "vol-b", Namespace: "default", Name: "ext", NodeID: c.NodeID(), HostPath: "/synthetic/ext",
	}
	var regResp cstructs.HostVolumeRegisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Register", regReq, &regResp))
	must.Eq(t, "ready", regResp.Status)

	nc, nr := mock.counts()
	must.Eq(t, 1, nc)
	must.Eq(t, 1, nr)

	var delResp cstructs.HostVolumeDeleteResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Delete",
		&cstructs.HostVolumeDeleteRequest{VolumeID: "vol-a", Namespace: "default", NodeID: c.NodeID()}, &delResp))
	must.Eq(t, "vol-a", delResp.VolumeID)
	var deregResp cstructs.HostVolumeDeregisterResponse
	must.NoError(t, msgpackrpc.CallWithCodec(codec, "ClientHostVolume.Deregister",
		&cstructs.HostVolumeDeregisterRequest{VolumeID: "vol-b", Namespace: "default", NodeID: c.NodeID()}, &deregResp))
	must.Eq(t, "vol-b", deregResp.VolumeID)
	nd, ng := mock.removalCounts()
	must.Eq(t, 1, nd)
	must.Eq(t, 1, ng)
}

// Only a caller presenting a server certificate for the local region may
// route calls to a client, because the endpoint does no ACL check of its
// own. A client certificate from the same region is refused before the node
// is looked up; a server certificate gets past the check and fails on the
// unknown node instead.
func TestClientHostVolume_RequiresServerCertificate(t *testing.T) {
	ci.Parallel(t)

	const (
		cafile     = "../helper/tlsutil/testdata/nomad-agent-ca.pem"
		serverCert = "../helper/tlsutil/testdata/regionFoo-server-nomad.pem"
		serverKey  = "../helper/tlsutil/testdata/regionFoo-server-nomad-key.pem"
		clientCert = "../helper/tlsutil/testdata/regionFoo-client-nomad.pem"
		clientKey  = "../helper/tlsutil/testdata/regionFoo-client-nomad-key.pem"
	)
	serverTLS := &sconfig.TLSConfig{
		EnableRPC:            true,
		VerifyServerHostname: true,
		CAFile:               cafile,
		CertFile:             serverCert,
		KeyFile:              serverKey,
	}
	clientTLS := serverTLS.Copy()
	clientTLS.CertFile = clientCert
	clientTLS.KeyFile = clientKey

	s, cleanup := TestServer(t, func(c *Config) {
		c.Region = "regionFoo"
		c.BootstrapExpect = 1
		c.NumSchedulers = 0
		c.TLSConfig = serverTLS
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	const nodeID = "5b0c0a5e-0000-4000-8000-000000000002"
	calls := []struct {
		method string
		args   any
		reply  any
	}{
		{
			method: "ClientHostVolume.Create",
			args: &cstructs.HostVolumeCreateRequest{
				VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: nodeID, PluginID: "plug",
			},
			reply: &cstructs.HostVolumeCreateResponse{},
		},
		{
			method: "ClientHostVolume.Register",
			args: &cstructs.HostVolumeRegisterRequest{
				VolumeID: "vol-b", Namespace: "default", Name: "ext", NodeID: nodeID, HostPath: "/synthetic/ext",
			},
			reply: &cstructs.HostVolumeRegisterResponse{},
		},
		{
			method: "ClientHostVolume.Delete",
			args:   &cstructs.HostVolumeDeleteRequest{VolumeID: "vol-a", Namespace: "default", NodeID: nodeID},
			reply:  &cstructs.HostVolumeDeleteResponse{},
		},
		{
			method: "ClientHostVolume.Deregister",
			args:   &cstructs.HostVolumeDeregisterRequest{VolumeID: "vol-b", Namespace: "default", NodeID: nodeID},
			reply:  &cstructs.HostVolumeDeregisterResponse{},
		},
	}

	for _, call := range calls {
		t.Run(call.method+"/client certificate", func(t *testing.T) {
			codec := rpcClientWithTLS(t, s, clientTLS)
			err := msgpackrpc.CallWithCodec(codec, call.method, call.args, call.reply)
			must.EqError(t, err, structs.ErrPermissionDenied.Error())
		})
		t.Run(call.method+"/server certificate", func(t *testing.T) {
			codec := rpcClientWithTLS(t, s, serverTLS)
			err := msgpackrpc.CallWithCodec(codec, call.method, call.args, call.reply)
			must.ErrorContains(t, err, "Unknown node")
		})
	}
}

// A request whose caller cannot be authenticated is refused before the node
// is looked up. An RPC context with no connection has no remote address, so
// authentication fails for the anonymous caller when ACLs are enabled.
func TestClientHostVolume_AuthenticateFailure(t *testing.T) {
	ci.Parallel(t)

	s, cleanup := TestServer(t, func(c *Config) {
		c.ACLEnabled = true
		c.BootstrapExpect = 1
		c.NumSchedulers = 0
	})
	t.Cleanup(cleanup)
	testutil.WaitForLeader(t, s.RPC)

	const nodeID = "5b0c0a5e-0000-4000-8000-000000000003"
	ep := NewClientHostVolumeEndpoint(s, &RPCContext{})

	err := ep.Create(&cstructs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: nodeID, PluginID: "plug",
	}, &cstructs.HostVolumeCreateResponse{})
	must.ErrorIs(t, err, structs.ErrPermissionDenied)

	err = ep.Register(&cstructs.HostVolumeRegisterRequest{
		VolumeID: "vol-b", Namespace: "default", Name: "ext", NodeID: nodeID, HostPath: "/synthetic/ext",
	}, &cstructs.HostVolumeRegisterResponse{})
	must.ErrorIs(t, err, structs.ErrPermissionDenied)

	err = ep.Delete(&cstructs.HostVolumeDeleteRequest{
		VolumeID: "vol-a", Namespace: "default", NodeID: nodeID,
	}, &cstructs.HostVolumeDeleteResponse{})
	must.ErrorIs(t, err, structs.ErrPermissionDenied)

	err = ep.Deregister(&cstructs.HostVolumeDeregisterRequest{
		VolumeID: "vol-b", Namespace: "default", NodeID: nodeID,
	}, &cstructs.HostVolumeDeregisterResponse{})
	must.ErrorIs(t, err, structs.ErrPermissionDenied)
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	log "github.com/hashicorp/go-hclog"

	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/nomad/structs"
)

// ClientHostVolume routes dynamic host volume calls from a server to the
// client that owns the volume. It is Wander policy.
//
// Like ClientCSI it does no namespace ACL check: only servers may call it,
// which the TLS certificate level enforces, and the HostVolume server
// endpoints authorize the caller before they send a request here. A node
// connected to another server is reached by forwarding the call to that
// server.
type ClientHostVolume struct {
	srv    *Server
	ctx    *RPCContext
	logger log.Logger
}

func NewClientHostVolumeEndpoint(srv *Server, ctx *RPCContext) *ClientHostVolume {
	return &ClientHostVolume{srv: srv, ctx: ctx, logger: srv.logger.Named("client_host_volume")}
}

// Create asks the node to provision a volume. A rejection by the client is
// in the reply, not in the error.
func (a *ClientHostVolume) Create(args *cstructs.HostVolumeCreateRequest, reply *cstructs.HostVolumeCreateResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_host_volume", "create"}, time.Now())
	return a.sendNodeRPC(args.NodeID, "HostVolume.Create", "ClientHostVolume.Create", args, reply)
}

// Register asks the node to adopt an existing directory as a volume.
func (a *ClientHostVolume) Register(args *cstructs.HostVolumeRegisterRequest, reply *cstructs.HostVolumeRegisterResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_host_volume", "register"}, time.Now())
	return a.sendNodeRPC(args.NodeID, "HostVolume.Register", "ClientHostVolume.Register", args, reply)
}

// Delete asks the node to delete a plugin-created volume. A rejection by the
// client is in the reply, not in the error.
func (a *ClientHostVolume) Delete(args *cstructs.HostVolumeDeleteRequest, reply *cstructs.HostVolumeDeleteResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_host_volume", "delete"}, time.Now())
	return a.sendNodeRPC(args.NodeID, "HostVolume.Delete", "ClientHostVolume.Delete", args, reply)
}

// Deregister asks the node to drop the record of a registered volume. The
// node never runs a plugin or touches the directory.
func (a *ClientHostVolume) Deregister(args *cstructs.HostVolumeDeregisterRequest, reply *cstructs.HostVolumeDeregisterResponse) error {
	defer metrics.MeasureSince([]string{"nomad", "client_host_volume", "deregister"}, time.Now())
	return a.sendNodeRPC(args.NodeID, "HostVolume.Deregister", "ClientHostVolume.Deregister", args, reply)
}

func (a *ClientHostVolume) sendNodeRPC(nodeID, method, fwdMethod string, args, reply any) error {
	// client requests aren't RequestWithIdentity, so a placeholder fills in
	// the identity data for metrics
	identityReq := &structs.GenericRequest{}
	authErr := a.srv.Authenticate(a.ctx, identityReq)
	a.srv.MeasureRPCRate("client_host_volume", structs.RateMetricWrite, identityReq)

	// only servers can send these client RPCs
	err := validateTLSCertificateLevel(a.srv, a.ctx, tlsCertificateLevelServer)
	if authErr != nil || err != nil {
		return structs.ErrPermissionDenied
	}

	snap, err := a.srv.State().Snapshot()
	if err != nil {
		return err
	}
	if _, err = getNodeForRpc(snap, nodeID); err != nil {
		return err
	}

	state, ok := a.srv.getNodeConn(nodeID)
	if !ok {
		return findNodeConnAndForward(a.srv, nodeID, fwdMethod, args, reply)
	}
	if err := NodeRpc(state.Session, method, args, reply); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}

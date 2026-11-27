// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/armon/go-metrics"

	"github.com/open-wander/wander/acl"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
)

// Fixed texts of the coded errors of HostVolume.Delete and
// HostVolume.Deregister, in addition to those of create and register.
const (
	hostVolumeErrMissingID   = "missing volume ID"
	hostVolumeErrInUse       = "volume in use"
	hostVolumeErrUseDelete   = "volume was created by a plugin, use delete"
	hostVolumeErrUseDereg    = "volume was registered, use deregister"
	hostVolumeErrOriginKnown = "volume origin is unknown"
	hostVolumeErrRejected    = "client rejected"
)

// hostVolumeClientReasons lists the reason codes of a client rejection that
// are passed on. A reason comes from the client, so any other text is replaced
// by a fixed code: it could carry a path or a node name.
var hostVolumeClientReasons = []string{
	"delete_mismatch", "volume_kind_conflict", "invalid_input", "invalid_path", "filesystem_failed",
	"not_executable", "start_failed", "timeout", "output_too_large", "nonzero_exit", "invalid_output",
	"invalid_version", "unsupported_platform", "node_mismatch", "plugin_unavailable", "invalid_record",
}

const hostVolumeReasonOther = "client_rejected"

// Delete deletes a plugin-created dynamic host volume: it marks the volume
// deleting, asks the node to run the plugin delete, and removes the record.
// Delete needs the host-volume-delete capability, which host-volume-write
// implies.
func (v *HostVolume) Delete(args *structs.HostVolumeDeleteRequest, reply *structs.HostVolumeDeleteResponse) (retErr error) {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Delete", args, args, reply); done {
		return err
	}
	audit := &hostVolumeAudit{op: hostVolumeAuditDelete, ns: args.RequestNamespace(), volumeID: args.VolumeID, force: args.Force}
	defer func() { v.srv.auditHostVolumeCall(args, authErr, audit, retErr, nil) }()
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeDelete)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "delete"}, time.Now())

	index, err := v.remove(false, ns, args.VolumeID, args.Force)
	if err != nil {
		return err
	}
	reply.Index = index
	return nil
}

// Deregister drops a registered dynamic host volume: it marks the volume
// deleting, asks the node to drop its record, and removes the record. No
// plugin runs and the directory is never touched. There is no deregister
// capability, so Deregister needs host-volume-delete too.
func (v *HostVolume) Deregister(args *structs.HostVolumeDeregisterRequest, reply *structs.HostVolumeDeregisterResponse) (retErr error) {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Deregister", args, args, reply); done {
		return err
	}
	audit := &hostVolumeAudit{op: hostVolumeAuditDeregister, ns: args.RequestNamespace(), volumeID: args.VolumeID, force: args.Force}
	defer func() { v.srv.auditHostVolumeCall(args, authErr, audit, retErr, nil) }()
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeDelete)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "deregister"}, time.Now())

	index, err := v.remove(true, ns, args.VolumeID, args.Force)
	if err != nil {
		return err
	}
	reply.Index = index
	return nil
}

// remove runs a delete or a deregistration. The volume stays marked in flight
// for the whole call, the client call included, so a second request, the
// leader's retry loop and a create cannot race it. Every refusal that can be
// made from the stored record is made before the first write. The client call
// runs with no lock held other than that mark.
//
// A repeat of a volume that is already deleting is accepted and drives the
// removal again; the mark is a no-op for it.
func (v *HostVolume) remove(deregister bool, ns, id string, force bool) (uint64, error) {
	if ns == structs.AllNamespacesSentinel {
		return 0, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrSingleNamespace)
	}
	if id == "" {
		return 0, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrMissingID)
	}
	coord := v.srv.dhvCoordinator
	key := ns + "/" + id
	if !coord.begin(key) {
		return 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrInProgress)
	}
	defer coord.end(key)

	callClient, err := v.checkRemoval(deregister, ns, id, force)
	if err != nil {
		return 0, err
	}
	coord.at("delete-checked")

	// The apply checks again that no live allocation uses the volume, which
	// closes the window between the check above and this write.
	vol := structs.DynamicHostVolumeKey{Namespace: ns, ID: id}
	_, err = v.apply(structs.DynamicHostVolumeMarkDeletingRequestType, &structs.DynamicHostVolumeMarkDeletingRequest{
		Keys: []structs.DynamicHostVolumeKey{vol}, ModifyTime: time.Now().UnixNano(), WriteRequest: v.writeRequest(),
	})
	switch {
	case err == nil:
	case errors.Is(err, state.ErrDynamicHostVolumeStateInUse):
		return 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrInUse)
	case errors.Is(err, state.ErrDynamicHostVolumeStateTransition):
		return 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrConcurrent)
	default:
		return 0, hostVolumeApplyError(err)
	}
	coord.at("delete-marked")

	if callClient != nil {
		if err := v.callRemoval(deregister, callClient); err != nil {
			return 0, err
		}
	}

	index, err := v.apply(structs.DynamicHostVolumeRemoveRequestType, &structs.DynamicHostVolumeRemoveRequest{
		Keys: []structs.DynamicHostVolumeKey{vol}, WriteRequest: v.writeRequest(),
	})
	switch {
	case err == nil:
		return index, nil
	case errors.Is(err, ErrDynamicHostVolumeVersion):
		return 0, hostVolumeStateError(http.StatusServiceUnavailable, hostVolumeErrVersionGate, structs.DynamicHostVolumeStateDeleting)
	case errors.Is(err, state.ErrDynamicHostVolumeStateInUse):
		return 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrInUse)
	}
	return 0, hostVolumeStateError(http.StatusInternalServerError, hostVolumeErrOutcome, structs.DynamicHostVolumeStateDeleting)
}

// checkRemoval reads the stored volume once and makes every refusal that needs
// no write. It returns the volume the node must be called for, or nil when the
// record is removed without a call: a volume that never reached a node, or one
// whose node is gone and force was given.
func (v *HostVolume) checkRemoval(deregister bool, ns, id string, force bool) (*structs.DynamicHostVolume, error) {
	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return nil, err
	}
	vol, err := snap.DynamicHostVolumeByID(nil, ns, id)
	if err != nil {
		return nil, hostVolumeStateErr(err, "invalid volume ID")
	}
	if vol == nil {
		return nil, hostVolumeCoded(http.StatusNotFound, hostVolumeErrNotFound)
	}

	switch {
	case deregister && vol.Origin == structs.DynamicHostVolumeOriginRegistered,
		!deregister && vol.Origin == structs.DynamicHostVolumeOriginPluginCreated:
	case vol.Origin == structs.DynamicHostVolumeOriginRegistered:
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrUseDereg)
	case vol.Origin == structs.DynamicHostVolumeOriginPluginCreated:
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrUseDelete)
	default:
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrOriginKnown)
	}

	inUse, err := snap.DynamicHostVolumeInUse(vol)
	if err != nil {
		return nil, err
	}
	if inUse {
		return nil, hostVolumeCoded(http.StatusConflict, hostVolumeErrInUse)
	}

	// A volume with no node was never created or adopted on one, so nothing is
	// on disk and force is not needed.
	if vol.NodeID == "" {
		return nil, nil
	}
	node, err := snap.NodeByID(nil, vol.NodeID)
	if err != nil {
		return nil, fmt.Errorf("looking up node: %w", err)
	}
	status := hostVolumeNodeStatusName(node)
	gone := status == structs.NodeStatusDown || status == structs.NodeStatusDisconnected || status == "unknown"
	switch {
	case gone && force:
		return nil, nil
	case gone:
		return nil, hostVolumeCoded(http.StatusBadRequest, "node is "+status+", force is needed to remove the volume")
	case force:
		return nil, hostVolumeCoded(http.StatusBadRequest,
			fmt.Sprintf("force is only for a down, disconnected or unknown node, node status is %q", status))
	}
	return vol, nil
}

// hostVolumeNodeStatusName names the status of a node for an error message. A
// node that is not stored, or has a status the server does not know, is
// unknown, so a record value never reaches the message.
func hostVolumeNodeStatusName(node *structs.Node) string {
	if node == nil {
		return "unknown"
	}
	switch node.Status {
	case structs.NodeStatusInit, structs.NodeStatusReady, structs.NodeStatusDown, structs.NodeStatusDisconnected:
		return node.Status
	}
	return "unknown"
}

// callRemoval asks the node to delete or deregister the volume. The text of a
// transport error and the message of a rejection are never returned: they can
// carry addresses and host paths. The record stays deleting in both cases, and
// a repeat of the request runs the call again.
func (v *HostVolume) callRemoval(deregister bool, vol *structs.DynamicHostVolume) error {
	reason, err := v.removalCall(deregister, vol)
	deleting := structs.DynamicHostVolumeStateDeleting
	if err != nil {
		return hostVolumeStateError(http.StatusBadGateway, hostVolumeErrClientCall, deleting)
	}
	if reason == "" {
		return nil
	}
	return hostVolumeStateError(http.StatusBadGateway, hostVolumeErrRejected+": "+reason, deleting)
}

// removalCall makes the client call of a removal and tells the answers apart.
// It returns the RPC error as it is, so the leader's retry can tell an unknown
// node from a transport failure, and the client's rejection reason replaced by
// a fixed code when it is not one of the listed reasons. An empty reason and a
// nil error mean the client has removed the volume.
func (v *HostVolume) removalCall(deregister bool, vol *structs.DynamicHostVolume) (string, error) {
	var reason string
	var err error
	if deregister {
		var resp cstructs.HostVolumeDeregisterResponse
		err = v.srv.RPC("ClientHostVolume.Deregister", &cstructs.HostVolumeDeregisterRequest{
			VolumeID: vol.ID, Namespace: vol.Namespace, NodeID: vol.NodeID,
		}, &resp)
		reason = resp.Reason
	} else {
		var resp cstructs.HostVolumeDeleteResponse
		err = v.srv.RPC("ClientHostVolume.Delete", &cstructs.HostVolumeDeleteRequest{
			VolumeID: vol.ID, Namespace: vol.Namespace, NodeID: vol.NodeID,
		}, &resp)
		reason = resp.Reason
	}
	if err != nil {
		return "", err
	}
	if reason != "" && !slices.Contains(hostVolumeClientReasons, reason) {
		reason = hostVolumeReasonOther
	}
	return reason, nil
}

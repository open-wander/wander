// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"net/http"
	"slices"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-memdb"

	"github.com/open-wander/wander/acl"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
)

// Fixed texts of the coded errors of HostVolume.RemoveOrphan.
const (
	hostVolumeErrOrphanMode      = "mode must be delete or forget"
	hostVolumeErrOrphanMissingID = "missing node ID"
	hostVolumeErrOrphanNotFound  = "orphaned volume not found"
	hostVolumeErrOrphanHasRecord = "volume has a record, not an orphan"
	hostVolumeErrOrphanUseForget = "volume was registered, use forget"
	hostVolumeErrOrphanNamespace = "namespace is not the namespace of the volume on the node"
)

// An orphan is a dynamic volume that a node advertises and the servers hold no
// record of. It can hold real data, for example after a snapshot restore, so
// nothing here removes one on its own: an operator lists them and removes one
// by name. The definition and the rules below are Wander policy.

// ListOrphans lists the advertised dynamic volumes that have no record on
// their node. It is computed on request from the nodes and the volume records
// and stores nothing. Reading needs host-volume-read in the namespace the node
// advertises for the volume. A node that advertises no namespace, as one that
// predates the field does, is listed only for the wildcard namespace and only
// to a caller that may read nodes. The reply carries no host path.
//
// The advertised namespace is supplied by the client, so it only filters the
// listing: it never grants access to a volume.
func (v *HostVolume) ListOrphans(args *structs.HostVolumeOrphanListRequest, reply *structs.HostVolumeOrphanListResponse) error {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.ListOrphans", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricList, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRead)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "list_orphans"}, time.Now())

	opts := blockingOptions{
		queryOpts: &args.QueryOptions,
		queryMeta: &reply.QueryMeta,
		run: func(ws memdb.WatchSet, store *state.StateStore) error {
			snap, err := store.Snapshot()
			if err != nil {
				return err
			}

			var nodes []*structs.Node
			if args.NodeID != "" {
				node, err := orphanNode(ws, snap, args.NodeID)
				if err != nil {
					return err
				}
				if node != nil {
					nodes = append(nodes, node)
				}
			} else {
				iter, err := snap.Nodes(ws)
				if err != nil {
					return err
				}
				for raw := iter.Next(); raw != nil; raw = iter.Next() {
					nodes = append(nodes, raw.(*structs.Node))
				}
			}

			orphans := []*structs.HostVolumeOrphan{}
			for _, node := range nodes {
				found, err := orphansOfNode(ws, snap, aclObj, ns, node)
				if err != nil {
					return err
				}
				orphans = append(orphans, found...)
			}
			slices.SortFunc(orphans, func(a, b *structs.HostVolumeOrphan) int {
				if c := compareString(a.NodeID, b.NodeID); c != 0 {
					return c
				}
				return compareString(a.VolumeID, b.VolumeID)
			})
			reply.Orphans = orphans

			// Both the nodes and the volume records change the answer.
			nodesIndex, err := snap.Index("nodes")
			if err != nil {
				return err
			}
			volumesIndex, err := snap.Index(state.TableDynamicHostVolumes)
			if err != nil {
				return err
			}
			reply.Index = max(nodesIndex, volumesIndex, 1)
			v.srv.setQueryMeta(&reply.QueryMeta)
			return nil
		}}
	return v.srv.blockingRPC(&opts)
}

// orphanNode looks a node up by ID. An ID that is not shaped like a node ID
// names no node: the node table refuses it with an error, which would be a
// server error for what is only an unknown node.
func orphanNode(ws memdb.WatchSet, snap *state.StateSnapshot, nodeID string) (*structs.Node, error) {
	if len(nodeID) != len(uuid.Generate()) {
		return nil, nil
	}
	return snap.NodeByID(ws, nodeID)
}

func compareString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// orphansOfNode returns the advertised volumes of the node that have no record
// on it and that the caller may see in the request namespace.
func orphansOfNode(ws memdb.WatchSet, snap *state.StateSnapshot, aclObj *acl.ACL, reqNS string, node *structs.Node) ([]*structs.HostVolumeOrphan, error) {
	var advertised []*structs.ClientHostVolumeConfig
	for _, cfg := range node.HostVolumes {
		if cfg != nil && cfg.ID != "" {
			advertised = append(advertised, cfg)
		}
	}
	if len(advertised) == 0 {
		return nil, nil
	}

	held := map[string]struct{}{}
	iter, err := snap.DynamicHostVolumesByNode(ws, node.ID)
	if err != nil {
		return nil, err
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		held[raw.(*structs.DynamicHostVolume).ID] = struct{}{}
	}

	var out []*structs.HostVolumeOrphan
	for _, cfg := range advertised {
		if _, ok := held[cfg.ID]; ok {
			continue
		}
		if !orphanVisible(aclObj, reqNS, cfg.Namespace) {
			continue
		}
		out = append(out, &structs.HostVolumeOrphan{
			NodeID: node.ID, VolumeID: cfg.ID, Name: cfg.Name, Namespace: cfg.Namespace,
		})
	}
	return out, nil
}

// orphanVisible reports whether a caller may see an orphan that advertises
// the namespace advNS, in a listing for the request namespace reqNS. An empty
// advertised namespace belongs to no namespace the caller could be granted
// access to, so it needs the right to read nodes and the wildcard.
func orphanVisible(aclObj *acl.ACL, reqNS, advNS string) bool {
	wildcard := reqNS == structs.AllNamespacesSentinel
	if advNS == "" {
		return wildcard && aclObj.AllowNodeRead()
	}
	if !wildcard && advNS != reqNS {
		return false
	}
	return aclObj.AllowNsOp(advNS, acl.NamespaceCapabilityHostVolumeRead)
}

// RemoveOrphan removes an orphaned volume from its node. The mode is required
// and has no default:
//
//   - delete runs the plugin delete on the node and drops its record. A volume
//     that was registered has no plugin and is refused with 400.
//   - forget drops the record on the node and keeps the directory. No plugin
//     runs.
//
// Both write the client's tombstone, so a late create of the ID is refused.
// Nothing is written to Raft: the servers hold no record of an orphan. The
// request namespace is the namespace of the volume; the namespace the node
// advertises must match it when the node advertises one. A node that predates
// the field advertises none, and then the node's own record is the check: it
// refuses a namespace that is not its own. The server cannot tell an omitted
// namespace from "default".
//
// Removal needs host-volume-delete, which host-volume-write implies. A request
// without a valid mode removes nothing and cannot name its operation, so it is
// refused before the audit line is set up and writes none, but only for a caller
// that may remove the volume: any other call gets 403, and every call is
// metered.
func (v *HostVolume) RemoveOrphan(args *structs.HostVolumeOrphanRemoveRequest, reply *structs.HostVolumeOrphanRemoveResponse) (retErr error) {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.RemoveOrphan", args, args, reply); done {
		return err
	}
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	ns := args.RequestNamespace()
	authorize := func() error {
		if authErr != nil {
			return structs.ErrPermissionDenied
		}
		aclObj, err := v.srv.ResolveACL(args)
		if err != nil {
			return err
		}
		if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeDelete)(aclObj, ns) {
			return structs.ErrPermissionDenied
		}
		return nil
	}

	var op string
	switch args.Mode {
	case structs.HostVolumeOrphanModeDelete:
		op = hostVolumeAuditOrphanDelete
	case structs.HostVolumeOrphanModeForget:
		op = hostVolumeAuditOrphanForget
	default:
		// A caller that may not remove the volume learns nothing about the
		// request, not even that its mode is wrong.
		if err := authorize(); err != nil {
			return err
		}
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrOrphanMode)
	}
	audit := &hostVolumeAudit{op: op, ns: ns, volumeID: args.VolumeID}
	defer func() { v.srv.auditHostVolumeCall(args, authErr, audit, retErr, nil) }()
	if err := authorize(); err != nil {
		return err
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "remove_orphan"}, time.Now())

	return v.removeOrphan(ns, args.NodeID, args.VolumeID, args.Mode)
}

// removeOrphan makes the checks and the client call of RemoveOrphan. The
// volume stays marked in flight for the whole call, so a create or a second
// removal of the ID cannot race it. The client call runs with no other lock
// held. The error texts are fixed: a client message, a host path and the
// advertised name never reach them.
func (v *HostVolume) removeOrphan(ns, nodeID, id, mode string) error {
	if ns == structs.AllNamespacesSentinel {
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrSingleNamespace)
	}
	if nodeID == "" {
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrOrphanMissingID)
	}
	if id == "" {
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrMissingID)
	}
	coord := v.srv.dhvCoordinator
	key := ns + "/" + id
	if !coord.begin(key) {
		return hostVolumeCoded(http.StatusConflict, hostVolumeErrInProgress)
	}
	defer coord.end(key)

	if err := v.checkOrphanRemoval(ns, nodeID, id); err != nil {
		return err
	}
	coord.at("orphan-checked")

	// A record that appeared since the check is not an orphan.
	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return err
	}
	if err := orphanRecordRefusal(snap, id); err != nil {
		return err
	}

	reason, err := v.orphanClientCall(mode, ns, nodeID, id)
	if err != nil {
		return hostVolumeCoded(http.StatusBadGateway, hostVolumeErrClientCall)
	}
	return orphanClientReason(mode, reason)
}

// checkOrphanRemoval makes every refusal that can be made from the node and
// the stored records.
func (v *HostVolume) checkOrphanRemoval(ns, nodeID, id string) error {
	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return err
	}
	node, err := orphanNode(nil, snap, nodeID)
	if err != nil {
		return err
	}
	if node == nil {
		return hostVolumeCoded(http.StatusNotFound, hostVolumeErrOrphanNotFound)
	}
	var adv *structs.ClientHostVolumeConfig
	for _, cfg := range node.HostVolumes {
		if cfg != nil && cfg.ID == id {
			adv = cfg
			break
		}
	}
	if adv == nil {
		return hostVolumeCoded(http.StatusNotFound, hostVolumeErrOrphanNotFound)
	}
	// Another namespace's volume is reported as missing, so the answer never
	// reveals it.
	if adv.Namespace != "" && adv.Namespace != ns {
		return hostVolumeCoded(http.StatusNotFound, hostVolumeErrOrphanNotFound)
	}
	if err := orphanRecordRefusal(snap, id); err != nil {
		return err
	}

	// A client that predates the live-allocation check has none of its own, so
	// the allocations the servers know about are checked here. An allocation
	// that starts between this check and the removal is the window the
	// client's check under the volume lock closes for current clients.
	inUse, err := snap.DynamicHostVolumeInUse(&structs.DynamicHostVolume{Namespace: ns, Name: adv.Name, NodeID: nodeID})
	if err != nil {
		return err
	}
	if inUse {
		return hostVolumeCoded(http.StatusConflict, hostVolumeErrInUse)
	}
	return nil
}

// orphanRecordRefusal returns the 409 for a volume ID that has a record in any
// namespace on any node. There is no index on the ID alone, so it reads every
// record; removing an orphan is rare. A volume with a record is not an orphan
// and its removal is the delete, deregister or forget of that record.
func orphanRecordRefusal(snap *state.StateSnapshot, id string) error {
	iter, err := snap.DynamicHostVolumes(nil)
	if err != nil {
		return err
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		if raw.(*structs.DynamicHostVolume).ID == id {
			return hostVolumeCoded(http.StatusConflict, hostVolumeErrOrphanHasRecord)
		}
	}
	return nil
}

// orphanClientCall asks the node to remove the volume and returns the reason
// of its rejection, replaced by a fixed code when it is not a listed one.
func (v *HostVolume) orphanClientCall(mode, ns, nodeID, id string) (string, error) {
	var reason string
	var err error
	if mode == structs.HostVolumeOrphanModeForget {
		var resp cstructs.HostVolumeForgetResponse
		err = v.srv.RPC("ClientHostVolume.Forget", &cstructs.HostVolumeForgetRequest{
			VolumeID: id, Namespace: ns, NodeID: nodeID,
		}, &resp)
		reason = resp.Reason
	} else {
		var resp cstructs.HostVolumeDeleteResponse
		err = v.srv.RPC("ClientHostVolume.Delete", &cstructs.HostVolumeDeleteRequest{
			VolumeID: id, Namespace: ns, NodeID: nodeID,
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

// orphanClientReason turns the rejection of the client into the coded error of
// the call. Nothing is stored on the servers, so the errors carry no volume
// state.
func orphanClientReason(mode, reason string) error {
	switch {
	case reason == "":
		return nil
	case reason == "volume_kind_conflict" && mode == structs.HostVolumeOrphanModeDelete:
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrOrphanUseForget)
	case reason == "volume_in_use":
		return hostVolumeCoded(http.StatusConflict, hostVolumeErrInUse)
	case reason == "delete_mismatch":
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrOrphanNamespace)
	}
	return hostVolumeCoded(http.StatusBadGateway, hostVolumeErrRejected+": "+reason)
}

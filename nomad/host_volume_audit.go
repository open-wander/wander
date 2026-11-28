// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/open-wander/wander/nomad/structs"
)

// Operations of the audit trail of dynamic host volumes.
const (
	hostVolumeAuditCreate      = "create"
	hostVolumeAuditRegister    = "register"
	hostVolumeAuditUpdate      = "update"
	hostVolumeAuditDelete      = "delete"
	hostVolumeAuditDeregister  = "deregister"
	hostVolumeAuditDeleteClaim = "delete_claim"

	// The removal of an orphaned volume, by the mode of the request.
	hostVolumeAuditOrphanDelete = "orphan_delete"
	hostVolumeAuditOrphanForget = "orphan_forget"
)

// Outcome codes of the audit trail. The set is closed: an outcome is chosen
// from the kind of the result and never copied from an error text, which can
// carry a path, a parameter or client output.
const (
	hostVolumeOutcomeOK                = "ok"
	hostVolumeOutcomeReady             = "ready"
	hostVolumeOutcomeFailed            = "failed"
	hostVolumeOutcomeRemoved           = "removed"
	hostVolumeOutcomeDenied            = "denied"
	hostVolumeOutcomeInvalid           = "invalid"
	hostVolumeOutcomeNotFound          = "not_found"
	hostVolumeOutcomeInUse             = "in_use"
	hostVolumeOutcomeInProgress        = "in_progress"
	hostVolumeOutcomeConflict          = "conflict"
	hostVolumeOutcomeVersionGate       = "version_gate"
	hostVolumeOutcomeClientRejected    = "client_rejected"
	hostVolumeOutcomeClientUnreachable = "client_unreachable"
	hostVolumeOutcomeNotRecorded       = "outcome_not_recorded"
	hostVolumeOutcomeError             = "error"
)

// Actors that are not a token accessor ID.
const (
	hostVolumeActorManagement      = "management"
	hostVolumeActorUnauthenticated = "unauthenticated"
	// hostVolumeActorLeader is the actor of the leader's own settlements. It is
	// the accessor ID of the leader's internal token.
	hostVolumeActorLeader = "leader"
)

// hostVolumeAudit describes the operation an audit line reports. The volume
// ID is set once the record exists, so a line for a call that fails after the
// first write still names the volume.
type hostVolumeAudit struct {
	op       string
	ns       string
	volumeID string
	claimID  string
	force    bool
}

// hostVolumeAuditActor names the caller without ever touching a token secret.
// A request that failed authentication, or carries no identity, is
// unauthenticated; ACLs being disabled gives an identity with no token, which
// is management.
func hostVolumeAuditActor(identity *structs.AuthenticatedIdentity, authErr error) string {
	switch {
	case authErr != nil || identity == nil:
		return hostVolumeActorUnauthenticated
	case identity.ACLToken != nil:
		return identity.ACLToken.AccessorID
	case identity.Claims != nil:
		return "workload:" + identity.Claims.AllocationID
	case identity.ClientID != "":
		return "client:" + identity.ClientID
	}
	return hostVolumeActorManagement
}

// hostVolumeAuditOutcome maps the result of a call to an outcome code and,
// for a rejection by the client, a reason from the list of passed-on reasons.
// A volume in a reply decides between ready and failed for a create, register
// or update.
func hostVolumeAuditOutcome(err error, vol *structs.DynamicHostVolume) (outcome, reason string) {
	if err == nil {
		if vol != nil {
			switch vol.State {
			case structs.DynamicHostVolumeStateReady:
				return hostVolumeOutcomeReady, ""
			case structs.DynamicHostVolumeStateFailed:
				return hostVolumeOutcomeFailed, ""
			}
		}
		return hostVolumeOutcomeOK, ""
	}
	if errors.Is(err, structs.ErrPermissionDenied) {
		return hostVolumeOutcomeDenied, ""
	}
	code, msg, ok := structs.CodeFromRPCCodedErr(err)
	if !ok {
		return hostVolumeOutcomeError, ""
	}
	switch code {
	case http.StatusBadRequest:
		return hostVolumeOutcomeInvalid, ""
	case http.StatusNotFound:
		return hostVolumeOutcomeNotFound, ""
	case http.StatusServiceUnavailable:
		return hostVolumeOutcomeVersionGate, ""
	case http.StatusInternalServerError:
		return hostVolumeOutcomeNotRecorded, ""
	case http.StatusConflict:
		switch {
		case msg == hostVolumeErrInUse || msg == hostVolumeErrClaimInUse:
			return hostVolumeOutcomeInUse, ""
		case msg == hostVolumeErrInProgress:
			return hostVolumeOutcomeInProgress, ""
		}
		return hostVolumeOutcomeConflict, ""
	case http.StatusBadGateway:
		rest, found := strings.CutPrefix(msg, hostVolumeErrRejected+": ")
		if !found {
			return hostVolumeOutcomeClientUnreachable, ""
		}
		// The text after the reason is the volume state, which is fixed.
		reasonCode, _, _ := strings.Cut(rest, ",")
		if !slices.Contains(hostVolumeClientReasons, reasonCode) {
			reasonCode = hostVolumeReasonOther
		}
		return hostVolumeOutcomeClientRejected, reasonCode
	}
	return hostVolumeOutcomeError, ""
}

// auditHostVolume writes the one audit line of a volume-changing call or of a
// settlement by the leader. The keys are fixed, so no record value, token
// secret, host path, parameter, plugin output or client message can reach the
// line: only the actor, the namespace, an ID, an operation and a coded
// outcome do. The IDs come from the caller and are logged as values, which
// hclog quotes.
func (s *Server) auditHostVolume(actor string, a *hostVolumeAudit, outcome, reason string) {
	kv := []any{"actor", actor, "namespace", a.ns}
	if a.op == hostVolumeAuditDeleteClaim {
		kv = append(kv, "claim_id", a.claimID)
	} else {
		kv = append(kv, "volume_id", a.volumeID)
	}
	kv = append(kv, "operation", a.op, "outcome", outcome)
	if reason != "" {
		kv = append(kv, "reason", reason)
	}
	if a.force {
		kv = append(kv, "force", true)
	}
	s.logger.Named("dynamic_host_volume").Named("audit").Info("volume operation", kv...)
}

// auditHostVolumeCall writes the audit line of an RPC from its result.
func (s *Server) auditHostVolumeCall(args structs.RequestWithIdentity, authErr error, a *hostVolumeAudit,
	err error, vol *structs.DynamicHostVolume) {
	outcome, reason := hostVolumeAuditOutcome(err, vol)
	s.auditHostVolume(hostVolumeAuditActor(args.GetIdentity(), authErr), a, outcome, reason)
}

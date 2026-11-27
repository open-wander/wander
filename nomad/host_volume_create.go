// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-multierror"

	"github.com/open-wander/wander/acl"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
)

// Fixed texts of the coded errors of HostVolume.Create and HostVolume.Register.
// The texts never carry a record value, a path, a node ID or client output: an
// error that crosses the forwarding hop is only its code and this text.
const (
	hostVolumeErrSingleNamespace   = "a volume write needs a single namespace"
	hostVolumeErrNamespaceMismatch = "volume namespace does not match the request namespace"
	hostVolumeErrNamespaceNotFound = "namespace not found"
	hostVolumeErrMissingVolume     = "missing volume"
	hostVolumeErrNotFound          = "volume not found"
	hostVolumeErrOriginMismatch    = "volume origin mismatch"
	hostVolumeErrInProgress        = "operation in progress"
	hostVolumeErrConcurrent        = "volume changed concurrently, retry"
	hostVolumeErrNameConflict      = "name_conflict"
	hostVolumeErrNodeNotReady      = "node_not_ready"
	hostVolumeErrNodeIneligible    = "node_ineligible"
	hostVolumeErrNodeDraining      = "node_draining"
	hostVolumeErrVersionGate       = "dynamic host volumes need every server to run Wander 0.1.2 or later"
	hostVolumeErrClientCall        = "client call failed"
	hostVolumeErrOutcome           = "outcome not recorded"
)

// dynamicHostVolumeCoordinator holds the server-side state that keeps volume
// requests apart. placeMu is held over the node check and the pending upsert
// of a new volume, so two requests for one name on one node cannot both pass
// the check before either record exists; the state schema has no unique index
// on node and name. It is never held across a call to a client. inFlight holds
// the volumes (keyed namespace/ID) with an operation running, so that a second
// request for the same volume is refused instead of racing the first.
type dynamicHostVolumeCoordinator struct {
	placeMu sync.Mutex

	mu       sync.Mutex
	inFlight map[string]struct{}
	hook     func(point string)

	// The leader's retry loop keeps its settings and its run handle here, so
	// the server needs no field of its own for it.
	reconcileSettings dynamicHostVolumeReconcileSettings
	reconcileRun      dynamicHostVolumeReconcileRun
}

func newDynamicHostVolumeCoordinator() *dynamicHostVolumeCoordinator {
	return &dynamicHostVolumeCoordinator{
		inFlight:          make(map[string]struct{}),
		reconcileSettings: defaultDynamicHostVolumeReconcileSettings(),
	}
}

// begin marks the volume in flight. It reports false if it already is.
func (c *dynamicHostVolumeCoordinator) begin(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.inFlight[key]; ok {
		return false
	}
	c.inFlight[key] = struct{}{}
	return true
}

func (c *dynamicHostVolumeCoordinator) end(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inFlight, key)
}

func (c *dynamicHostVolumeCoordinator) isInFlight(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.inFlight[key]
	return ok
}

// setHook installs a function called at named points of a request. It is a
// seam for tests and is never set in production.
func (c *dynamicHostVolumeCoordinator) setHook(hook func(point string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hook = hook
}

func (c *dynamicHostVolumeCoordinator) at(point string) {
	c.mu.Lock()
	hook := c.hook
	c.mu.Unlock()
	if hook != nil {
		hook(point)
	}
}

// Create creates a dynamic host volume with a plugin, or, when the volume
// carries the ID of a stored volume, updates its mutable fields in Raft only.
// PolicyOverride is accepted and ignored: Wander has no policy engine.
func (v *HostVolume) Create(args *structs.HostVolumeCreateRequest, reply *structs.HostVolumeCreateResponse) (retErr error) {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Create", args, args, reply); done {
		return err
	}
	// The audit line is written here, after the forward, so that a forwarded
	// call is logged once, by the server that runs it, and on every return
	// path below.
	audit := newHostVolumeVolumeAudit(hostVolumeAuditCreate, args.RequestNamespace(), args.Volume)
	defer func() { v.srv.auditHostVolumeCall(args, authErr, audit, retErr, reply.Volume) }()
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeCreate)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "create"}, time.Now())

	vol, err := hostVolumeBindNamespace(args.Volume, ns)
	if err != nil {
		return err
	}
	out, index, err := v.write(false, ns, vol, audit)
	if err != nil {
		return err
	}
	reply.Volume, reply.Index = out, index
	return nil
}

// Register adopts an existing directory on a node as a dynamic host volume,
// or, when the volume carries the ID of a stored volume, updates its mutable
// fields in Raft only. The directory is never modified. PolicyOverride is
// accepted and ignored.
func (v *HostVolume) Register(args *structs.HostVolumeRegisterRequest, reply *structs.HostVolumeRegisterResponse) (retErr error) {
	authErr := v.srv.Authenticate(v.ctx, args)
	if done, err := v.srv.forward("HostVolume.Register", args, args, reply); done {
		return err
	}
	// The audit line is written here, after the forward, so that a forwarded
	// call is logged once, by the server that runs it, and on every return
	// path below.
	audit := newHostVolumeVolumeAudit(hostVolumeAuditRegister, args.RequestNamespace(), args.Volume)
	defer func() { v.srv.auditHostVolumeCall(args, authErr, audit, retErr, reply.Volume) }()
	v.srv.MeasureRPCRate("host_volume", structs.RateMetricWrite, args)
	if authErr != nil {
		return structs.ErrPermissionDenied
	}

	aclObj, err := v.srv.ResolveACL(args)
	if err != nil {
		return err
	}
	ns := args.RequestNamespace()
	if !acl.NamespaceValidator(acl.NamespaceCapabilityHostVolumeRegister)(aclObj, ns) {
		return structs.ErrPermissionDenied
	}

	defer metrics.MeasureSince([]string{"nomad", "host_volume", "register"}, time.Now())

	vol, err := hostVolumeBindNamespace(args.Volume, ns)
	if err != nil {
		return err
	}
	out, index, err := v.write(true, ns, vol, audit)
	if err != nil {
		return err
	}
	reply.Volume, reply.Index = out, index
	return nil
}

// newHostVolumeVolumeAudit starts the audit value of a create or a register. A
// volume that carries an ID is an update of that volume. The ID of a new volume
// is set once its record exists.
func newHostVolumeVolumeAudit(op, ns string, vol *structs.DynamicHostVolume) *hostVolumeAudit {
	a := &hostVolumeAudit{op: op, ns: ns}
	if vol != nil && vol.ID != "" {
		a.op, a.volumeID = hostVolumeAuditUpdate, vol.ID
	}
	return a
}

func hostVolumeCoded(code int, msg string) error {
	return structs.NewErrRPCCoded(code, msg)
}

// hostVolumeBindNamespace returns a copy of the request volume that belongs to
// the request namespace. The ACL check ran on the request namespace, so a body
// that names another one is refused rather than written under it.
func hostVolumeBindNamespace(vol *structs.DynamicHostVolume, ns string) (*structs.DynamicHostVolume, error) {
	if ns == structs.AllNamespacesSentinel {
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrSingleNamespace)
	}
	if vol == nil {
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrMissingVolume)
	}
	if vol.Namespace != "" && vol.Namespace != ns {
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNamespaceMismatch)
	}
	bound := vol.Copy()
	bound.Namespace = ns
	return bound, nil
}

// hostVolumeStateName names a state for an error message. Only a state the
// model knows is quoted, so a record value never reaches the message.
func hostVolumeStateName(st structs.DynamicHostVolumeState) string {
	switch st {
	case structs.DynamicHostVolumeStateUnknown:
		return "none"
	case structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending,
		structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeStateUnavailable,
		structs.DynamicHostVolumeStateDeleting, structs.DynamicHostVolumeStateFailed:
		return string(st)
	}
	return "unknown"
}

func hostVolumeStateError(code int, prefix string, st structs.DynamicHostVolumeState) error {
	msg := fmt.Sprintf("volume state is %q", hostVolumeStateName(st))
	if prefix != "" {
		msg = prefix + ", " + msg
	}
	return hostVolumeCoded(code, msg)
}

// hostVolumeValidationError reports the failing fields and their codes. The
// messages of the validators can quote request values, so only the field name
// and the code are kept.
func hostVolumeValidationError(err error) error {
	errs := []error{err}
	var merr *multierror.Error
	if errors.As(err, &merr) {
		errs = merr.Errors
	}
	codes := []struct {
		name string
		err  error
	}{
		{"required", structs.ErrDynamicHostVolumeRequired},
		{"unsupported", structs.ErrDynamicHostVolumeUnsupported},
		{"invalid", structs.ErrDynamicHostVolumeInvalid},
		{"immutable", structs.ErrDynamicHostVolumeImmutable},
	}
	var parts []string
	for _, e := range errs {
		field, _, _ := strings.Cut(e.Error(), ":")
		part := field
		for _, code := range codes {
			if errors.Is(e, code.err) {
				part = field + ": " + code.name
				break
			}
		}
		parts = append(parts, part)
	}
	slices.Sort(parts)
	parts = slices.Compact(parts)
	return hostVolumeCoded(http.StatusBadRequest, "invalid volume: "+strings.Join(parts, ", "))
}

// hostVolumeApplyError maps the refusal of a first write. Nothing has been
// written when it applies.
func hostVolumeApplyError(err error) error {
	switch {
	case errors.Is(err, ErrDynamicHostVolumeVersion):
		return hostVolumeCoded(http.StatusServiceUnavailable, hostVolumeErrVersionGate)
	case errors.Is(err, state.ErrDynamicHostVolumeStateNamespace):
		return hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNamespaceNotFound)
	case errors.Is(err, state.ErrDynamicHostVolumeStateInvalid):
		return hostVolumeCoded(http.StatusBadRequest, "invalid volume")
	}
	return err
}

func (v *HostVolume) apply(t structs.MessageType, msg any) (uint64, error) {
	_, index, err := v.srv.dynamicHostVolumeRaftApply(t, msg)
	return index, err
}

func (v *HostVolume) writeRequest() structs.WriteRequest {
	return structs.WriteRequest{Region: v.srv.Region()}
}

// write runs a create or a register. A volume without an ID is new; one with
// an ID updates the stored volume.
func (v *HostVolume) write(register bool, ns string, req *structs.DynamicHostVolume, audit *hostVolumeAudit) (*structs.DynamicHostVolume, uint64, error) {
	if req.ID != "" {
		return v.update(register, ns, req)
	}
	return v.add(register, ns, req, audit)
}

// add records a new volume pending on its node, asks the node to act, and
// records the answer. The client call runs with no lock held. A client that
// answers with a rejection gives a failed record in a normal reply; a call
// that fails in transit leaves the record pending, because the client may have
// finished the work, and a later retry decides.
func (v *HostVolume) add(register bool, ns string, req *structs.DynamicHostVolume, audit *hostVolumeAudit) (*structs.DynamicHostVolume, uint64, error) {
	vol := req.Copy()
	now := time.Now().UnixNano()
	vol.ID = uuid.Generate()
	if vol.Type == "" {
		vol.Type = structs.VolumeTypeHost
	}
	vol.State = structs.DynamicHostVolumeStateUnknown
	vol.StatusDescription = ""
	vol.CapacityBytes = 0
	vol.Attachments = nil
	vol.CreateIndex, vol.ModifyIndex = 0, 0
	vol.CreateTime, vol.ModifyTime = now, now
	if register {
		vol.Origin = structs.DynamicHostVolumeOriginRegistered
	} else {
		vol.Origin = structs.DynamicHostVolumeOriginPluginCreated
		vol.HostPath = ""
	}

	if err := validateHostVolume(register, vol); err != nil {
		return nil, 0, err
	}
	coord := v.srv.dhvCoordinator
	key := ns + "/" + vol.ID
	if !coord.begin(key) {
		return nil, 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrInProgress)
	}
	defer coord.end(key)

	failed, index, err := v.place(register, vol, audit)
	if err != nil {
		return nil, 0, err
	}
	if failed {
		out, err := v.readBack(ns, vol.ID, structs.DynamicHostVolumeStateFailed)
		return out, index, err
	}

	index, err = v.settle(register, vol)
	if err != nil {
		return nil, 0, err
	}
	out, err := v.readBack(ns, vol.ID, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeStateFailed,
		structs.DynamicHostVolumeStateUnavailable)
	return out, index, err
}

func validateHostVolume(register bool, vol *structs.DynamicHostVolume) error {
	var err error
	if register {
		err = vol.ValidateRegister()
	} else {
		err = vol.ValidateCreate()
	}
	if err != nil {
		return hostVolumeValidationError(err)
	}
	return nil
}

// place chooses the node and writes the first record under the placement
// lock. It returns true, with the index of the last write, when no node was
// eligible and the volume was recorded as failed; otherwise the record is
// pending on its node. The lock is released before it returns.
func (v *HostVolume) place(register bool, vol *structs.DynamicHostVolume, audit *hostVolumeAudit) (bool, uint64, error) {
	coord := v.srv.dhvCoordinator
	coord.placeMu.Lock()
	defer coord.placeMu.Unlock()

	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return false, 0, err
	}

	if register {
		node, err := registerHostVolumeNode(snap, vol)
		if err != nil {
			return false, 0, err
		}
		vol.NodePool = node.NodePool
	} else {
		node, diag, err := selectDynamicHostVolumeNode(snap, vol)
		switch {
		case errors.Is(err, ErrDynamicHostVolumeNodeNotFound):
			return false, 0, hostVolumeCoded(http.StatusBadRequest, ErrDynamicHostVolumeNodeNotFound.Error())
		case errors.Is(err, ErrDynamicHostVolumeNodePoolMismatch):
			return false, 0, hostVolumeCoded(http.StatusBadRequest, ErrDynamicHostVolumeNodePoolMismatch.Error())
		case err != nil:
			return false, 0, err
		case diag != nil:
			return v.recordNoNode(vol, diag, audit)
		}
		vol.NodeID, vol.NodePool = node.ID, node.NodePool
	}

	vol.State = structs.DynamicHostVolumeStatePending
	if _, err := v.apply(structs.DynamicHostVolumeUpsertRequestType, &structs.DynamicHostVolumeUpsertRequest{
		Volumes: []*structs.DynamicHostVolume{vol}, WriteRequest: v.writeRequest(),
	}); err != nil {
		return false, 0, hostVolumeApplyError(err)
	}
	audit.volumeID = vol.ID
	return false, 0, nil
}

// recordNoNode records a volume for which no node qualifies as requested and
// then failed, with the diagnostic codes as its description. A name that every
// candidate node already uses is a conflict of the request, not a placement
// failure, and writes nothing.
func (v *HostVolume) recordNoNode(vol *structs.DynamicHostVolume, diag *DynamicHostVolumeNodeDiagnostic, audit *hostVolumeAudit) (bool, uint64, error) {
	if len(diag.Filtered) == 1 && diag.Filtered[0].Reason == DynamicHostVolumeReasonNameConflict {
		return false, 0, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNameConflict)
	}

	vol.State, vol.NodeID = structs.DynamicHostVolumeStateRequested, ""
	if _, err := v.apply(structs.DynamicHostVolumeUpsertRequestType, &structs.DynamicHostVolumeUpsertRequest{
		Volumes: []*structs.DynamicHostVolume{vol}, WriteRequest: v.writeRequest(),
	}); err != nil {
		return false, 0, hostVolumeApplyError(err)
	}
	audit.volumeID = vol.ID

	text := dynamicHostVolumeDiagnosticText(diag)
	index, err := v.apply(structs.DynamicHostVolumeOutcomeRequestType, &structs.DynamicHostVolumeOutcomeRequest{
		Key:          structs.DynamicHostVolumeKey{Namespace: vol.Namespace, ID: vol.ID},
		State:        structs.DynamicHostVolumeStateFailed,
		Diagnostic:   text,
		ModifyTime:   time.Now().UnixNano(),
		WriteRequest: v.writeRequest(),
	})
	if err != nil {
		return false, 0, v.leftError(vol.Namespace, vol.ID, http.StatusInternalServerError, hostVolumeErrOutcome)
	}
	return true, index, nil
}

// dynamicHostVolumeDiagnosticText renders a diagnostic as its reason followed
// by the codes and counts of the filtered nodes, for example
// "no_eligible_node: node_pool=1".
func dynamicHostVolumeDiagnosticText(diag *DynamicHostVolumeNodeDiagnostic) string {
	text := diag.Reason
	for i, f := range diag.Filtered {
		sep := ","
		if i == 0 {
			sep = ": "
		}
		text += fmt.Sprintf("%s%s=%d", sep, f.Reason, f.Count)
	}
	return text
}

// registerHostVolumeNode checks the node a registration names. Register names
// its node, so nothing is selected: the node must exist, be in the requested
// pool, be ready, eligible and not draining, and must not already use the name
// in any namespace, either through a record or through a static host volume.
func registerHostVolumeNode(snap *state.StateSnapshot, vol *structs.DynamicHostVolume) (*structs.Node, error) {
	node, err := snap.NodeByID(nil, vol.NodeID)
	if err != nil {
		return nil, fmt.Errorf("looking up node: %w", err)
	}
	if node == nil {
		return nil, hostVolumeCoded(http.StatusBadRequest, ErrDynamicHostVolumeNodeNotFound.Error())
	}
	if vol.NodePool != "" && vol.NodePool != structs.NodePoolAll && vol.NodePool != node.NodePool {
		return nil, hostVolumeCoded(http.StatusBadRequest, ErrDynamicHostVolumeNodePoolMismatch.Error())
	}
	switch {
	case node.Status != structs.NodeStatusReady:
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNodeNotReady)
	case node.DrainStrategy != nil:
		// A drain also makes the node ineligible, so it is checked first to
		// report the cause.
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNodeDraining)
	case node.SchedulingEligibility != structs.NodeSchedulingEligible:
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNodeIneligible)
	}
	if node.HostVolumes[vol.Name] != nil {
		return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNameConflict)
	}
	iter, err := snap.DynamicHostVolumes(nil)
	if err != nil {
		return nil, fmt.Errorf("listing dynamic host volumes: %w", err)
	}
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		rec := raw.(*structs.DynamicHostVolume)
		if rec.NodeID == node.ID && rec.Name == vol.Name {
			return nil, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrNameConflict)
		}
	}
	return node, nil
}

// errHostVolumeClientReply marks a client reply that is neither a rejection nor
// a success, so it settles nothing.
var errHostVolumeClientReply = errors.New("client reply carries neither a reason nor a usable status")

// clientCall asks the node to act on the pending volume and turns the reply
// into the outcome to record. It runs with no lock held and writes nothing. It
// returns the raw error of the call, or errHostVolumeClientReply for a reply
// that settles nothing; neither is ever stored or returned to a caller.
func (v *HostVolume) clientCall(register bool, vol *structs.DynamicHostVolume) (*structs.DynamicHostVolumeOutcomeRequest, error) {
	var reason, message, status, hostPath string
	var capacity int64
	var err error
	if register {
		var resp cstructs.HostVolumeRegisterResponse
		err = v.srv.RPC("ClientHostVolume.Register", &cstructs.HostVolumeRegisterRequest{
			VolumeID: vol.ID, Namespace: vol.Namespace, Name: vol.Name, NodeID: vol.NodeID,
			NodePool: vol.NodePool, HostPath: vol.HostPath,
		}, &resp)
		reason, message, status, hostPath = resp.Reason, resp.Message, resp.Status, vol.HostPath
	} else {
		var resp cstructs.HostVolumeCreateResponse
		err = v.srv.RPC("ClientHostVolume.Create", &cstructs.HostVolumeCreateRequest{
			VolumeID: vol.ID, Namespace: vol.Namespace, Name: vol.Name, NodeID: vol.NodeID,
			NodePool: vol.NodePool, PluginID: vol.PluginID,
			CapacityMinBytes: vol.RequestedCapacityMinBytes, CapacityMaxBytes: vol.RequestedCapacityMaxBytes,
			Parameters: vol.Parameters,
		}, &resp)
		reason, message, status, hostPath, capacity = resp.Reason, resp.Message, resp.Status, resp.HostPath, resp.CapacityBytes
	}
	if err != nil {
		return nil, err
	}

	outcome := &structs.DynamicHostVolumeOutcomeRequest{
		Key:          structs.DynamicHostVolumeKey{Namespace: vol.Namespace, ID: vol.ID},
		ModifyTime:   time.Now().UnixNano(),
		WriteRequest: v.writeRequest(),
	}
	switch {
	case reason != "":
		outcome.State = structs.DynamicHostVolumeStateFailed
		outcome.Diagnostic = reason
		if message != "" {
			outcome.Diagnostic = reason + ": " + message
		}
	case status == "ready" || status == "unavailable":
		// The stored state of a volume the client reports as unavailable
		// follows from the node, not from this reply.
		outcome.State = structs.DynamicHostVolumeStateReady
		outcome.HostPath, outcome.CapacityBytes = hostPath, capacity
	default:
		return nil, errHostVolumeClientReply
	}
	return outcome, nil
}

// settle asks the node to act on the pending volume and records the answer. It
// runs with no lock held. It returns the index of the outcome write.
func (v *HostVolume) settle(register bool, vol *structs.DynamicHostVolume) (uint64, error) {
	outcome, err := v.clientCall(register, vol)
	// The text of a transport error is never stored or returned: it can
	// carry addresses and client output.
	if err != nil {
		return 0, v.leftError(vol.Namespace, vol.ID, http.StatusBadGateway, hostVolumeErrClientCall)
	}
	index, err := v.apply(structs.DynamicHostVolumeOutcomeRequestType, outcome)
	if err != nil {
		return 0, v.leftError(vol.Namespace, vol.ID, http.StatusInternalServerError, hostVolumeErrOutcome)
	}
	v.srv.dhvCoordinator.at("outcome-applied")
	return index, nil
}

// leftError builds the error for a request that stopped after the first write:
// it names the state the record is in now, which is pending unless something
// else has moved it.
func (v *HostVolume) leftError(ns, id string, code int, prefix string) error {
	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return hostVolumeCoded(code, prefix)
	}
	rec, err := snap.DynamicHostVolumeByID(nil, ns, id)
	if err != nil || rec == nil {
		return hostVolumeCoded(code, prefix)
	}
	if rec.State == structs.DynamicHostVolumeStatePending || rec.State == structs.DynamicHostVolumeStateRequested {
		return hostVolumeStateError(code, prefix, rec.State)
	}
	return hostVolumeStateError(http.StatusConflict, "", rec.State)
}

// readBack returns a copy of the stored volume. The write was only a request:
// the record is what the state now says, so a state other than the ones the
// caller expects, such as deleting after a concurrent delete, is an error.
func (v *HostVolume) readBack(ns, id string, expect ...structs.DynamicHostVolumeState) (*structs.DynamicHostVolume, error) {
	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return nil, err
	}
	rec, err := snap.DynamicHostVolumeByID(nil, ns, id)
	if err != nil {
		return nil, hostVolumeStateErr(err, "invalid volume ID")
	}
	if rec == nil {
		return nil, hostVolumeStateError(http.StatusConflict, "", structs.DynamicHostVolumeStateUnknown)
	}
	if !slices.Contains(expect, rec.State) {
		return nil, hostVolumeStateError(http.StatusConflict, "", rec.State)
	}
	return rec.Copy(), nil
}

// update changes the mutable fields of a stored ready volume. Everything the
// server owns comes from the stored record, and the update is recorded in Raft
// only: the plugin is not called, so anything it would have to act on is
// refused as immutable.
func (v *HostVolume) update(register bool, ns string, req *structs.DynamicHostVolume) (*structs.DynamicHostVolume, uint64, error) {
	coord := v.srv.dhvCoordinator
	key := ns + "/" + req.ID
	if !coord.begin(key) {
		return nil, 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrInProgress)
	}
	defer coord.end(key)

	snap, err := v.srv.State().Snapshot()
	if err != nil {
		return nil, 0, err
	}
	stored, err := snap.DynamicHostVolumeByID(nil, ns, req.ID)
	if err != nil {
		return nil, 0, hostVolumeStateErr(err, "invalid volume ID")
	}
	if stored == nil {
		return nil, 0, hostVolumeCoded(http.StatusNotFound, hostVolumeErrNotFound)
	}

	wantOrigin := structs.DynamicHostVolumeOriginPluginCreated
	if register {
		wantOrigin = structs.DynamicHostVolumeOriginRegistered
	}
	if stored.Origin != wantOrigin {
		return nil, 0, hostVolumeCoded(http.StatusBadRequest, hostVolumeErrOriginMismatch)
	}
	if stored.State != structs.DynamicHostVolumeStateReady {
		return nil, 0, hostVolumeStateError(http.StatusConflict, "", stored.State)
	}

	// An omitted node, host path or type means unchanged; a different one is
	// refused rather than silently ignored.
	for _, f := range []struct {
		name string
		same bool
	}{
		{"Name", req.Name == stored.Name},
		{"Type", req.Type == "" || req.Type == stored.Type},
		{"NodeID", req.NodeID == "" || req.NodeID == stored.NodeID},
		{"HostPath", !register || req.HostPath == "" || req.HostPath == stored.HostPath},
		{"PluginID", req.PluginID == stored.PluginID},
		{"NodePool", req.NodePool == "" || req.NodePool == structs.NodePoolAll || req.NodePool == stored.NodePool},
	} {
		if !f.same {
			return nil, 0, hostVolumeCoded(http.StatusBadRequest, "immutable field "+f.name)
		}
	}

	candidate := stored.Copy()
	candidate.RequestedCapacityMinBytes = req.RequestedCapacityMinBytes
	candidate.RequestedCapacityMaxBytes = req.RequestedCapacityMaxBytes
	candidate.RequestedCapabilities = req.Copy().RequestedCapabilities
	candidate.Constraints = req.Copy().Constraints
	candidate.Parameters = req.Copy().Parameters
	if err := validateHostVolume(register, candidate); err != nil {
		return nil, 0, err
	}

	if hostVolumeSameRecord(stored, candidate) {
		return stored.Copy(), stored.ModifyIndex, nil
	}

	// ModifyIndex is the compare-and-set value. A conflict is reported, not
	// retried: the caller decides against what it now sees.
	candidate.ModifyIndex = stored.ModifyIndex
	candidate.ModifyTime = time.Now().UnixNano()
	coord.at("update-apply")
	index, err := v.apply(structs.DynamicHostVolumeUpdateRequestType, &structs.DynamicHostVolumeUpdateRequest{
		Volume: candidate, WriteRequest: v.writeRequest(),
	})
	switch {
	case err == nil:
	case errors.Is(err, state.ErrDynamicHostVolumeStateCAS):
		return nil, 0, hostVolumeCoded(http.StatusConflict, hostVolumeErrConcurrent)
	case errors.Is(err, state.ErrDynamicHostVolumeStateTransition):
		return nil, 0, v.leftError(ns, req.ID, http.StatusConflict, "")
	case errors.Is(err, structs.ErrDynamicHostVolumeImmutable):
		return nil, 0, hostVolumeCoded(http.StatusBadRequest, "immutable field")
	default:
		return nil, 0, hostVolumeApplyError(err)
	}
	out, err := v.readBack(ns, req.ID, structs.DynamicHostVolumeStateReady, structs.DynamicHostVolumeStateUnavailable)
	return out, index, err
}

// hostVolumeSameRecord reports whether the candidate repeats the stored record
// apart from its modify time. An empty map or list counts as an absent one,
// because the encoding of a request does not tell them apart.
func hostVolumeSameRecord(stored, candidate *structs.DynamicHostVolume) bool {
	left, right := stored.Copy(), candidate.Copy()
	right.ModifyTime = left.ModifyTime
	for _, rec := range []*structs.DynamicHostVolume{left, right} {
		if len(rec.Parameters) == 0 {
			rec.Parameters = nil
		}
		if len(rec.Constraints) == 0 {
			rec.Constraints = nil
		}
		if len(rec.RequestedCapabilities) == 0 {
			rec.RequestedCapabilities = nil
		}
		if len(rec.Attachments) == 0 {
			rec.Attachments = nil
		}
	}
	return left.Equal(right)
}

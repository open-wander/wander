// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package nomad

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/hashicorp/go-hclog"

	"github.com/open-wander/wander/helper"
	"github.com/open-wander/wander/nomad/state"
	"github.com/open-wander/wander/nomad/structs"
)

// Fixed diagnostic codes the reconciler records. They never carry an error
// text, a path or a parameter.
const (
	hostVolumeCodeRetryLimit      = "retry_limit_reached"
	hostVolumeCodeNodeNotFound    = "node_not_found"
	hostVolumeCodeRequestedOrigin = "invalid_requested_origin"

	// Codes stored as the status description of a deleting volume that waits.
	// A removal is never recorded failed: it is retried until it succeeds.
	hostVolumeCodeClientUnreachable = "client_unreachable"
	hostVolumeCodeVolumeInUse       = "volume_in_use"
)

// dynamicHostVolumeReconcileSettings are the timings of the leader's retry of
// volumes left requested, pending or deleting. Production uses the defaults;
// tests set shorter values before the reconciler starts.
type dynamicHostVolumeReconcileSettings struct {
	// Interval is the time between two scans of the volume table.
	Interval time.Duration
	// BackoffBase and BackoffCap bound the delay before a volume is tried
	// again: the delay doubles with each attempt, with jitter.
	BackoffBase time.Duration
	BackoffCap  time.Duration
	// Bound is the age after which a requested or pending volume that keeps
	// failing transiently is recorded as failed. It is counted from the
	// record's modify time, or from the last refusal of the version gate when
	// that is later. It does not apply to deleting volumes: a removal is
	// retried until it succeeds and is never recorded failed.
	Bound time.Duration
	// CallTimeout is the deadline of one client call.
	CallTimeout time.Duration
}

func defaultDynamicHostVolumeReconcileSettings() dynamicHostVolumeReconcileSettings {
	return dynamicHostVolumeReconcileSettings{
		Interval:    10 * time.Second,
		BackoffBase: time.Second,
		BackoffCap:  time.Minute,
		Bound:       10 * time.Minute,
		CallTimeout: time.Minute,
	}
}

// delay is the wait after the n-th attempt, with jitter between half and all
// of the exponential value.
func (s dynamicHostVolumeReconcileSettings) delay(n int) time.Duration {
	d := s.BackoffBase
	for i := 1; i < n && d < s.BackoffCap; i++ {
		d *= 2
	}
	if d > s.BackoffCap {
		d = s.BackoffCap
	}
	half := d / 2
	return half + helper.RandomStagger(half+1)
}

// dynamicHostVolumeReconcileRun is the handle of the running reconciler. It
// lives on the coordinator, so the server needs no field of its own.
type dynamicHostVolumeReconcileRun struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}

	// running counts the loops that have started and not yet exited. It is
	// at most one; tests read it.
	running atomic.Int32
}

// setReconcileSettings replaces the settings the next start reads. It is a seam
// for tests and is never called in production.
func (c *dynamicHostVolumeCoordinator) setReconcileSettings(set dynamicHostVolumeReconcileSettings) {
	c.reconcileRun.mu.Lock()
	defer c.reconcileRun.mu.Unlock()
	c.reconcileSettings = set
}

// startDynamicHostVolumeReconciler starts the leader's retry loop. It stops a
// running instance first, because the reassert path of the leader loop calls
// revoke and then establish with the same stop channel: a loop that only
// watched that channel would run twice.
func (s *Server) startDynamicHostVolumeReconciler(stopCh <-chan struct{}) {
	coord := s.dhvCoordinator
	run := &coord.reconcileRun
	run.mu.Lock()
	defer run.mu.Unlock()
	run.stopLocked()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	run.cancel, run.done = cancel, done
	r := s.newDynamicHostVolumeReconciler(coord.reconcileSettings)
	run.running.Add(1)
	go func() {
		defer close(done)
		defer run.running.Add(-1)
		r.run(ctx, stopCh)
	}()
}

func (s *Server) newDynamicHostVolumeReconciler(set dynamicHostVolumeReconcileSettings) *dynamicHostVolumeReconciler {
	r := &dynamicHostVolumeReconciler{
		srv:      s,
		coord:    s.dhvCoordinator,
		set:      set,
		logger:   s.logger.Named("dynamic_host_volume_reconciler"),
		backoff:  make(map[string]*dynamicHostVolumeBackoff),
		overdue:  make(map[string]*dynamicHostVolumeCall),
		returned: make(chan struct{}, 1),
	}
	r.hv = &HostVolume{srv: s, logger: r.logger}
	return r
}

// stopDynamicHostVolumeReconciler stops the loop and waits for it to exit. A
// client call still outstanding, or one past its deadline that has returned
// but is not yet recorded, is abandoned: it writes nothing, and its volume
// stays in flight until the call returns.
func (s *Server) stopDynamicHostVolumeReconciler() {
	run := &s.dhvCoordinator.reconcileRun
	run.mu.Lock()
	defer run.mu.Unlock()
	run.stopLocked()
}

func (r *dynamicHostVolumeReconcileRun) stopLocked() {
	if r.cancel == nil {
		return
	}
	r.cancel()
	<-r.done
	r.cancel, r.done = nil, nil
}

type dynamicHostVolumeBackoff struct {
	tries int       // failed attempts and refusals of the version gate
	next  time.Time // no attempt before this time
	// gateAt is the last refusal of the version gate. The bound is counted
	// from it when it is later than the record's modify time, so time spent
	// waiting for the gate never counts toward the bound.
	gateAt time.Time
}

// dynamicHostVolumeCall is a client call that runs in its own goroutine. The
// volume stays in flight until the answer has been handled or, after a stop,
// until the call returns.
type dynamicHostVolumeCall struct {
	key     string
	cur     *structs.DynamicHostVolume
	results chan dynamicHostVolumeCallResult // buffered, receives one answer
}

type dynamicHostVolumeReconciler struct {
	srv    *Server
	coord  *dynamicHostVolumeCoordinator
	hv     *HostVolume
	set    dynamicHostVolumeReconcileSettings
	logger log.Logger

	// Only the loop goroutine touches these maps.
	backoff map[string]*dynamicHostVolumeBackoff
	overdue map[string]*dynamicHostVolumeCall // calls past their deadline
	// returned wakes the loop when a call returns, so the answer of a call
	// past its deadline is recorded without waiting for the next scan.
	returned chan struct{}
}

func (r *dynamicHostVolumeReconciler) run(ctx context.Context, stopCh <-chan struct{}) {
	defer r.abandonOverdue()
	r.pass(ctx)
	timer, cancel := helper.NewSafeTimer(r.set.Interval)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case <-r.returned:
			r.collect(ctx)
		case <-timer.C:
			r.pass(ctx)
			timer.Reset(r.set.Interval)
		}
	}
}

// pass scans the whole table: it has no index on the state, and a modest
// interval keeps the cost low.
func (r *dynamicHostVolumeReconciler) pass(ctx context.Context) {
	r.collect(ctx)
	snap, err := r.srv.State().Snapshot()
	if err != nil {
		return
	}
	iter, err := snap.DynamicHostVolumes(nil)
	if err != nil {
		return
	}
	var due []*structs.DynamicHostVolume
	seen := make(map[string]bool)
	for raw := iter.Next(); raw != nil; raw = iter.Next() {
		rec := raw.(*structs.DynamicHostVolume)
		if !dynamicHostVolumeReconciled(rec.State) {
			continue
		}
		key := rec.Namespace + "/" + rec.ID
		seen[key] = true
		if e := r.backoff[key]; e != nil && time.Now().Before(e.next) {
			continue
		}
		due = append(due, rec.Copy())
	}
	for key := range r.backoff {
		if !seen[key] {
			delete(r.backoff, key)
		}
	}
	for _, rec := range due {
		if ctx.Err() != nil {
			return
		}
		r.process(ctx, rec)
	}
	r.coord.at("reconcile-pass")
}

// process drives one volume. The volume is taken in flight first, so an RPC
// that holds it is skipped, and the record is read again afterwards: an RPC may
// have settled it between the scan and the mark, and a second client call
// would follow.
func (r *dynamicHostVolumeReconciler) process(ctx context.Context, scanned *structs.DynamicHostVolume) {
	key := scanned.Namespace + "/" + scanned.ID
	if !r.coord.begin(key) {
		return
	}
	handedOff := false
	defer func() {
		if !handedOff {
			r.coord.end(key)
		}
	}()

	cur := r.reload(scanned)
	if cur == nil {
		r.forget(key)
		return
	}
	if cur.State == structs.DynamicHostVolumeStateDeleting {
		if !r.prepareRemoval(ctx, key, cur) {
			return
		}
		handedOff = true
		r.call(ctx, key, cur)
		return
	}
	if cur.State == structs.DynamicHostVolumeStateRequested {
		var ok bool
		if cur, ok = r.assign(ctx, key, cur); !ok {
			return
		}
	}
	handedOff = true
	r.call(ctx, key, cur)
}

// reload returns the stored record when it is still the one that was scanned.
func (r *dynamicHostVolumeReconciler) reload(scanned *structs.DynamicHostVolume) *structs.DynamicHostVolume {
	snap, err := r.srv.State().Snapshot()
	if err != nil {
		return nil
	}
	cur, err := snap.DynamicHostVolumeByID(nil, scanned.Namespace, scanned.ID)
	if err != nil || cur == nil || cur.ModifyIndex != scanned.ModifyIndex {
		return nil
	}
	if !dynamicHostVolumeReconciled(cur.State) {
		return nil
	}
	return cur.Copy()
}

// dynamicHostVolumeReconciled reports whether the leader drives volumes in the
// state: requested and pending ones towards ready, deleting ones towards
// removal.
func dynamicHostVolumeReconciled(st structs.DynamicHostVolumeState) bool {
	switch st {
	case structs.DynamicHostVolumeStateRequested, structs.DynamicHostVolumeStatePending, structs.DynamicHostVolumeStateDeleting:
		return true
	}
	return false
}

// assign chooses a node for a requested volume and records it as pending, or
// records the volume as failed when no node qualifies. It reports whether the
// volume is pending and ready for the client call.
func (r *dynamicHostVolumeReconciler) assign(ctx context.Context, key string, cur *structs.DynamicHostVolume) (*structs.DynamicHostVolume, bool) {
	if cur.Origin != structs.DynamicHostVolumeOriginPluginCreated {
		// Register never writes a requested record, so this cannot arise.
		r.fail(ctx, key, cur, hostVolumeCodeRequestedOrigin)
		return nil, false
	}

	r.coord.placeMu.Lock()
	pending, code, ok := r.selectAndAssign(ctx, key, cur)
	r.coord.placeMu.Unlock()
	if code != "" {
		r.fail(ctx, key, cur, code)
		return nil, false
	}
	return pending, ok
}

// selectAndAssign runs with the placement lock held. It returns a failure code
// when the volume cannot be placed and will not be.
func (r *dynamicHostVolumeReconciler) selectAndAssign(ctx context.Context, key string, cur *structs.DynamicHostVolume) (*structs.DynamicHostVolume, string, bool) {
	snap, err := r.srv.State().Snapshot()
	if err != nil {
		r.transient(ctx, key, cur)
		return nil, "", false
	}
	node, diag, err := selectDynamicHostVolumeNode(snap, cur)
	switch {
	case errors.Is(err, ErrDynamicHostVolumeNodeNotFound), errors.Is(err, ErrDynamicHostVolumeNodePoolMismatch):
		return nil, err.Error(), false
	case err != nil:
		r.transient(ctx, key, cur)
		return nil, "", false
	case diag != nil:
		if len(diag.Filtered) == 1 && diag.Filtered[0].Reason == DynamicHostVolumeReasonNameConflict {
			return nil, hostVolumeErrNameConflict, false
		}
		return nil, dynamicHostVolumeDiagnosticText(diag), false
	}

	// The assign step is compare-and-set on the stored modify index.
	pending := cur.Copy()
	pending.State, pending.NodeID, pending.NodePool = structs.DynamicHostVolumeStatePending, node.ID, node.NodePool
	pending.ModifyTime = time.Now().UnixNano()
	if _, err := r.hv.apply(structs.DynamicHostVolumeUpsertRequestType, &structs.DynamicHostVolumeUpsertRequest{
		Volumes: []*structs.DynamicHostVolume{pending}, WriteRequest: r.hv.writeRequest(),
	}); err != nil {
		r.writeFailed(ctx, key, cur, err, false)
		return nil, "", false
	}
	delete(r.backoff, key)
	return pending, "", true
}

type dynamicHostVolumeCallResult struct {
	outcome *structs.DynamicHostVolumeOutcomeRequest
	// reason is the fixed code of a rejected removal.
	reason string
	err    error
}

// call asks the client again and records the answer. The client call has a
// deadline so that one node cannot hold up the loop. A call past its deadline
// may still act on the node, so its volume is neither retried nor failed while
// the call runs: it stays in flight, and the answer is recorded when the call
// returns.
func (r *dynamicHostVolumeReconciler) call(ctx context.Context, key string, cur *structs.DynamicHostVolume) {
	c := r.startCall(key, cur)
	timer := time.NewTimer(r.set.CallTimeout)
	defer timer.Stop()
	select {
	case res := <-c.results:
		r.finish(ctx, c, res)
	case <-timer.C:
		r.overdue[key] = c
		r.logger.Debug("client call passed its deadline", "namespace", cur.Namespace, "id", cur.ID)
	case <-ctx.Done():
		r.abandon(c)
	}
}

func (r *dynamicHostVolumeReconciler) startCall(key string, cur *structs.DynamicHostVolume) *dynamicHostVolumeCall {
	c := &dynamicHostVolumeCall{key: key, cur: cur, results: make(chan dynamicHostVolumeCallResult, 1)}
	register := cur.Origin == structs.DynamicHostVolumeOriginRegistered
	go func() {
		if cur.State == structs.DynamicHostVolumeStateDeleting {
			reason, err := r.hv.removalCall(register, cur)
			c.results <- dynamicHostVolumeCallResult{reason: reason, err: err}
		} else {
			outcome, err := r.hv.clientCall(register, cur)
			c.results <- dynamicHostVolumeCallResult{outcome: outcome, err: err}
		}
		select {
		case r.returned <- struct{}{}:
		default:
		}
	}()
	return c
}

// collect handles the calls past their deadline that have returned.
func (r *dynamicHostVolumeReconciler) collect(ctx context.Context) {
	for key, c := range r.overdue {
		select {
		case res := <-c.results:
			delete(r.overdue, key)
			r.finish(ctx, c, res)
		default:
		}
	}
}

// abandon releases the volume once the call returns, and writes nothing.
func (r *dynamicHostVolumeReconciler) abandon(c *dynamicHostVolumeCall) {
	go func() {
		<-c.results
		r.coord.end(c.key)
	}()
}

func (r *dynamicHostVolumeReconciler) abandonOverdue() {
	for key, c := range r.overdue {
		delete(r.overdue, key)
		r.abandon(c)
	}
}

// finish records the answer of a call and then releases the volume, so no RPC
// can take the volume between the answer and its record.
func (r *dynamicHostVolumeReconciler) finish(ctx context.Context, c *dynamicHostVolumeCall, res dynamicHostVolumeCallResult) {
	defer r.coord.end(c.key)
	if ctx.Err() != nil {
		return
	}
	if c.cur.State == structs.DynamicHostVolumeStateDeleting {
		r.finishRemoval(ctx, c, res)
		return
	}
	switch {
	case res.err != nil && structs.IsErrUnknownNode(res.err):
		// The node is gone from the state store, so no retry can reach it.
		r.fail(ctx, c.key, c.cur, hostVolumeCodeNodeNotFound)
	case res.err != nil:
		// No connection to the node, a transport error, or a reply that
		// settles nothing: the node may be down or the client may have
		// finished the work, so wait and ask again.
		r.transient(ctx, c.key, c.cur)
	default:
		r.record(ctx, c.key, c.cur, res.outcome, false)
	}
}

// fail records a fixed diagnostic code as the failure of the volume.
func (r *dynamicHostVolumeReconciler) fail(ctx context.Context, key string, cur *structs.DynamicHostVolume, code string) {
	r.record(ctx, key, cur, &structs.DynamicHostVolumeOutcomeRequest{
		Key:          structs.DynamicHostVolumeKey{Namespace: cur.Namespace, ID: cur.ID},
		State:        structs.DynamicHostVolumeStateFailed,
		Diagnostic:   code,
		ModifyTime:   time.Now().UnixNano(),
		WriteRequest: r.hv.writeRequest(),
	}, true)
}

// record writes the outcome. It never writes once the loop has been stopped.
func (r *dynamicHostVolumeReconciler) record(ctx context.Context, key string, cur *structs.DynamicHostVolume,
	outcome *structs.DynamicHostVolumeOutcomeRequest, failing bool) {
	if ctx.Err() != nil {
		return
	}
	if _, err := r.hv.apply(structs.DynamicHostVolumeOutcomeRequestType, outcome); err != nil {
		r.writeFailed(ctx, key, cur, err, failing)
		return
	}
	delete(r.backoff, key)
	r.logger.Debug("recorded outcome", "namespace", cur.Namespace, "id", cur.ID, "state", string(outcome.State))
	r.audit(cur, string(outcome.State))
}

// audit writes the audit line of a volume the leader has settled: a volume
// that became ready or failed, or whose record was removed. It is called only
// after the write succeeded, so a retry that settles nothing writes no line.
// The operation is the one that left the volume unsettled: a requested or
// pending volume is a create or a register, a deleting one a delete or a
// deregister, by origin.
func (r *dynamicHostVolumeReconciler) audit(cur *structs.DynamicHostVolume, outcome string) {
	registered := cur.Origin == structs.DynamicHostVolumeOriginRegistered
	op := hostVolumeAuditCreate
	switch {
	case cur.State == structs.DynamicHostVolumeStateDeleting && registered:
		op = hostVolumeAuditDeregister
	case cur.State == structs.DynamicHostVolumeStateDeleting:
		op = hostVolumeAuditDelete
	case registered:
		op = hostVolumeAuditRegister
	}
	r.srv.auditHostVolume(hostVolumeActorLeader, &hostVolumeAudit{op: op, ns: cur.Namespace, volumeID: cur.ID}, outcome, "")
}

// writeFailed handles a refused write. A refusal of the version gate waits and
// moves the start of the bound to the refusal, so time spent waiting for the
// gate never counts toward it. A refusal by the state means the record moved on (for
// example to deleting) or was changed by another writer: it is dropped, and the
// next scan reads the record again. Anything else, such as an outcome the state
// finds invalid, counts as a failed attempt. A write that records a failure
// only waits, so a failure cannot trigger another failure.
func (r *dynamicHostVolumeReconciler) writeFailed(ctx context.Context, key string, cur *structs.DynamicHostVolume, err error, failing bool) {
	switch {
	case errors.Is(err, ErrDynamicHostVolumeVersion):
		r.wait(key, true)
		r.logger.Debug("waiting for the version gate", "namespace", cur.Namespace, "id", cur.ID)
	case errors.Is(err, state.ErrDynamicHostVolumeStateTransition),
		errors.Is(err, state.ErrDynamicHostVolumeStateCAS),
		errors.Is(err, state.ErrDynamicHostVolumeStateConflict):
		r.forget(key)
		r.logger.Debug("dropped an outcome the state refused", "namespace", cur.Namespace, "id", cur.ID, "state", string(cur.State))
	case failing:
		r.wait(key, false)
	default:
		r.transient(ctx, key, cur)
	}
}

// transient counts a failed attempt and schedules the next one. Once the
// volume is older than the bound it is recorded as failed. The age is counted
// from the record's modify time, or from the last refusal of the version gate
// when that is later, so time spent waiting for the gate does not count. It is
// only checked here, after an attempt, so at least one attempt is made on this
// leader first and a cluster that was down does not fail volumes it never
// retried.
func (r *dynamicHostVolumeReconciler) transient(ctx context.Context, key string, cur *structs.DynamicHostVolume) {
	e := r.wait(key, false)
	since := time.Unix(0, cur.ModifyTime)
	if e.gateAt.After(since) {
		since = e.gateAt
	}
	if time.Since(since) <= r.set.Bound {
		return
	}
	r.logger.Warn("retry limit reached", "namespace", cur.Namespace, "id", cur.ID, "state", string(cur.State))
	r.fail(ctx, key, cur, hostVolumeCodeRetryLimit)
}

// wait schedules the next attempt. A refusal of the version gate also moves
// the start of the bound.
func (r *dynamicHostVolumeReconciler) wait(key string, gate bool) *dynamicHostVolumeBackoff {
	e := r.backoff[key]
	if e == nil {
		e = &dynamicHostVolumeBackoff{}
		r.backoff[key] = e
	}
	now := time.Now()
	e.tries++
	if gate {
		e.gateAt = now
	}
	e.next = now.Add(r.set.delay(e.tries))
	return e
}

// forget lets the next scan try the volume at once. It keeps the last refusal
// of the version gate, which still holds for the record.
func (r *dynamicHostVolumeReconciler) forget(key string) {
	e := r.backoff[key]
	if e == nil {
		return
	}
	if e.gateAt.IsZero() {
		delete(r.backoff, key)
		return
	}
	r.backoff[key] = &dynamicHostVolumeBackoff{gateAt: e.gateAt}
}

// prepareRemoval makes the checks that need no client call and reports whether
// the client must be called. A volume with no node never reached one, so its
// record is removed at once. A volume a live allocation uses is checked before
// the call, because the plugin delete wipes the directory and the removal of
// the record is refused only afterwards. The state of the node is not checked:
// the call fails and is retried, and force through the RPC is the operator's
// way out for a node that is gone.
func (r *dynamicHostVolumeReconciler) prepareRemoval(ctx context.Context, key string, cur *structs.DynamicHostVolume) bool {
	if cur.NodeID == "" {
		r.removeRecord(ctx, key, cur)
		return false
	}
	snap, err := r.srv.State().Snapshot()
	if err != nil {
		r.wait(key, false)
		return false
	}
	inUse, err := snap.DynamicHostVolumeInUse(cur)
	if err != nil {
		r.wait(key, false)
		return false
	}
	if inUse {
		r.deletingRetry(ctx, key, cur, hostVolumeCodeVolumeInUse)
		return false
	}
	return true
}

// finishRemoval records the answer of a removal call. A deleting volume is
// never recorded failed: whatever went wrong is stored as a fixed code and the
// removal is tried again, however long that takes.
func (r *dynamicHostVolumeReconciler) finishRemoval(ctx context.Context, c *dynamicHostVolumeCall, res dynamicHostVolumeCallResult) {
	switch {
	case res.err != nil && structs.IsErrUnknownNode(res.err):
		r.deletingRetry(ctx, c.key, c.cur, hostVolumeCodeNodeNotFound)
	case res.err != nil:
		// No connection, a transport error, or a client that could not read
		// its records: the node may be down, so wait and ask again.
		r.deletingRetry(ctx, c.key, c.cur, hostVolumeCodeClientUnreachable)
	case res.reason != "":
		r.deletingRetry(ctx, c.key, c.cur, res.reason)
	default:
		r.removeRecord(ctx, c.key, c.cur)
	}
}

// removeRecord removes the record of a volume the client no longer holds.
func (r *dynamicHostVolumeReconciler) removeRecord(ctx context.Context, key string, cur *structs.DynamicHostVolume) {
	if ctx.Err() != nil {
		return
	}
	_, err := r.hv.apply(structs.DynamicHostVolumeRemoveRequestType, &structs.DynamicHostVolumeRemoveRequest{
		Keys:         []structs.DynamicHostVolumeKey{{Namespace: cur.Namespace, ID: cur.ID}},
		WriteRequest: r.hv.writeRequest(),
	})
	switch {
	case err == nil:
		delete(r.backoff, key)
		r.logger.Debug("removed volume", "namespace", cur.Namespace, "id", cur.ID)
		r.audit(cur, hostVolumeOutcomeRemoved)
	case errors.Is(err, ErrDynamicHostVolumeVersion):
		r.wait(key, true)
		r.logger.Debug("waiting for the version gate", "namespace", cur.Namespace, "id", cur.ID)
	case errors.Is(err, state.ErrDynamicHostVolumeStateInUse):
		r.deletingRetry(ctx, key, cur, hostVolumeCodeVolumeInUse)
	case errors.Is(err, state.ErrDynamicHostVolumeStateTransition):
		r.forget(key)
	default:
		r.wait(key, false)
	}
}

// deletingRetry stores the reason a removal is waiting and schedules the next
// attempt. The code is written only when it differs from the stored one, so a
// volume that keeps failing the same way costs no Raft write. The write does not
// go through record: that resets the backoff, and the delay would never reach
// its cap. A refused write never fails the volume.
func (r *dynamicHostVolumeReconciler) deletingRetry(ctx context.Context, key string, cur *structs.DynamicHostVolume, code string) {
	if ctx.Err() != nil {
		return
	}
	if cur.StatusDescription != code {
		r.coord.at("reconcile-outcome-apply")
		_, err := r.hv.apply(structs.DynamicHostVolumeOutcomeRequestType, &structs.DynamicHostVolumeOutcomeRequest{
			Key:          structs.DynamicHostVolumeKey{Namespace: cur.Namespace, ID: cur.ID},
			State:        structs.DynamicHostVolumeStateDeleting,
			Diagnostic:   code,
			WriteRequest: r.hv.writeRequest(),
		})
		switch {
		case err == nil:
		case errors.Is(err, ErrDynamicHostVolumeVersion):
			r.wait(key, true)
			return
		case errors.Is(err, state.ErrDynamicHostVolumeStateTransition),
			errors.Is(err, state.ErrDynamicHostVolumeStateCAS),
			errors.Is(err, state.ErrDynamicHostVolumeStateConflict):
			// The record moved on, for example it was removed meanwhile.
			r.forget(key)
			return
		}
	}
	r.wait(key, false)
	r.logger.Debug("removal is waiting", "namespace", cur.Namespace, "id", cur.ID)
}

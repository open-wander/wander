// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package hostvolume

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/open-wander/wander/nomad/structs"
)

const (
	// DefaultReconcileInterval is how often the manager re-checks every
	// volume. It is fixed Wander policy, not a configuration setting.
	DefaultReconcileInterval = 30 * time.Second

	// DefaultRestoreTimeout bounds the whole initial restore. Each plugin
	// call is also bounded by the Ops timeouts. Wander policy.
	DefaultRestoreTimeout = 5 * time.Minute
)

// Errors a PluginLookup returns to say why a plugin cannot be used.
var (
	ErrPluginMissing   = errors.New("plugin is not installed")
	ErrPluginUnhealthy = errors.New("plugin fingerprint is unhealthy")
)

// ErrRegisteredVolume is returned when an operation only valid for a volume
// a plugin created is attempted on a registered one.
var ErrRegisteredVolume = errors.New("volume is registered and has no plugin to delete it")

// Store persists the volume records. The manager is the only writer.
type Store interface {
	PutHostVolume(*hvstate.Record) error
	GetHostVolumes() ([]*hvstate.Record, map[string]error, error)
	DeleteHostVolume(volumeID string) error
}

// PluginLookup finds a plugin by ID and checks that its fingerprint is
// healthy. It returns ErrPluginMissing or ErrPluginUnhealthy otherwise.
type PluginLookup func(ctx context.Context, pluginID string) (Candidate, error)

// ManagerConfig holds the dependencies of a Manager.
type ManagerConfig struct {
	Logger hclog.Logger
	Store  Store

	// Ops runs plugin and built-in operations. It must be the instance
	// shared by every caller in the process.
	Ops *Ops

	// NodeID is the ID of this node. Records for another node are never
	// reported.
	NodeID string

	// Lookup finds and fingerprints a plugin. The default discovers
	// Ops.PluginDir and runs the plugin's fingerprint.
	Lookup PluginLookup

	// StaticName reports whether a static host_volume has the name.
	StaticName func(name string) bool

	// Publish receives the complete set of ready dynamic volumes, keyed by
	// name, whenever it changes. It is never called with the manager mutex
	// held and the map is the receiver's to keep.
	Publish func(map[string]*structs.ClientHostVolumeConfig)

	Clock          func() time.Time
	Interval       time.Duration
	RestoreTimeout time.Duration
}

// VolumeStatus is a copy of the last known state of one volume: Wander
// policy diagnostics. Message never holds a parameter value.
type VolumeStatus struct {
	VolumeID  string
	Name      string
	Path      string
	Kind      hvstate.Kind
	Status    hvstate.Status
	Reason    string
	Message   string
	CheckedAt time.Time
}

// RegisterRequest describes an existing external path to register.
type RegisterRequest struct {
	VolumeID  string
	Namespace string
	Name      string
	NodeID    string
	NodePool  string
	Path      string
}

// entry is a record with its in-memory bookkeeping.
type entry struct {
	rec *hvstate.Record
	// rev changes whenever the record is replaced or removed, so a pass
	// that evaluated an older record does not overwrite a newer one.
	rev uint64
	// stored is the diagnostic last written to the store.
	stored diag
	// loaded marks a record read from the store at start, as opposed to one
	// created in this run. Only loaded records are restored.
	loaded bool
	// checked marks a record that this run created or evaluated. Only
	// checked records are published: a status loaded from the store is what
	// an earlier run saw.
	checked bool
}

type diag struct {
	status  hvstate.Status
	reason  string
	message string
}

// result is the outcome of evaluating one record in a pass.
type result struct {
	rev     uint64
	status  hvstate.Status
	reason  string
	message string
	at      time.Time
}

// Manager owns the dynamic host volume records of this node: it is the only
// code that writes or removes them. It restores them when the client starts,
// reconciles them periodically and publishes the ready ones. The behaviour
// is Wander policy.
type Manager struct {
	cfg    ManagerConfig
	logger hclog.Logger

	// locks serialises operations per volume ID. It is separate from the
	// lock table of Ops, which Ops takes itself inside each operation.
	locks lockTable

	// mu guards entries and every store write. It is never held while a
	// plugin runs or while Publish is called.
	mu      sync.Mutex
	entries map[string]*entry
	nextRev uint64

	// loadOnce reads the store before anything else touches entries, so a
	// record written through an entry point is never replaced by a later
	// read of the store.
	loadOnce sync.Once

	// pubMu serialises building and publishing the set so that an older
	// set is never delivered after a newer one.
	pubMu     sync.Mutex
	published map[string]*structs.ClientHostVolumeConfig

	// restored is closed when the first Restore ends, however it ends, so
	// that a caller that needs the restored set can wait for it. Restore
	// closes it after the first publish has returned.
	restored     chan struct{}
	restoredOnce sync.Once
}

// NewManager returns a manager. Store, Ops, NodeID and Publish are required.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	switch {
	case cfg.Store == nil:
		return nil, errors.New("host volume manager: store is required")
	case cfg.Ops == nil:
		return nil, errors.New("host volume manager: ops is required")
	case cfg.NodeID == "":
		return nil, errors.New("host volume manager: node id is required")
	case cfg.Publish == nil:
		return nil, errors.New("host volume manager: publish is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultReconcileInterval
	}
	if cfg.RestoreTimeout <= 0 {
		cfg.RestoreTimeout = DefaultRestoreTimeout
	}
	if cfg.StaticName == nil {
		cfg.StaticName = func(string) bool { return false }
	}
	m := &Manager{
		cfg:      cfg,
		logger:   cfg.Logger.Named("host_volumes"),
		entries:  make(map[string]*entry),
		restored: make(chan struct{}),
	}
	if cfg.Lookup == nil {
		m.cfg.Lookup = m.defaultLookup
	}
	return m, nil
}

// defaultLookup discovers the plugin directory and fingerprints the plugin.
func (m *Manager) defaultLookup(ctx context.Context, pluginID string) (Candidate, error) {
	cands, _ := Discover(m.cfg.Ops.PluginDir)
	for _, c := range cands {
		if c.ID != pluginID {
			continue
		}
		if r := (Runner{}).Fingerprint(ctx, c); !r.Healthy {
			return Candidate{}, ErrPluginUnhealthy
		}
		return c, nil
	}
	return Candidate{}, ErrPluginMissing
}

// Restore loads the persisted records and reconciles all of them once,
// running create once for every plugin-created volume. It is bounded by
// RestoreTimeout and never fails: problems are logged and recorded as
// per-volume diagnostics.
func (m *Manager) Restore(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.RestoreTimeout)
	defer cancel()
	defer m.restoredOnce.Do(func() { close(m.restored) })

	m.ensureLoaded()
	m.reconcile(ctx, true)
}

// Restored returns a channel that is closed when the first Restore has ended,
// after it published the volumes it verified. It is also closed when that
// Restore was cut short by its deadline or by shutdown, so a waiter is
// bounded by RestoreTimeout.
func (m *Manager) Restored() <-chan struct{} {
	return m.restored
}

// Namespace returns the namespace of the record of a volume. A volume the
// manager does not know, for example one that was deleted meanwhile, reports
// false. The namespace of a record never changes.
func (m *Manager) Namespace(volumeID string) (string, bool) {
	m.ensureLoaded()
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[volumeID]
	if e == nil {
		return "", false
	}
	return e.rec.Namespace, true
}

// ensureLoaded reads the store once. Restore and every entry point call it
// first, whichever runs first, because Restore runs in the background.
func (m *Manager) ensureLoaded() {
	m.loadOnce.Do(m.load)
}

// load reads the store. A record that cannot be decoded or fails validation
// is skipped and logged without its contents.
func (m *Manager) load() {
	recs, errs, err := m.cfg.Store.GetHostVolumes()
	if err != nil {
		m.logger.Error("failed to read host volume records", "error", errClass(err))
		return
	}
	for id := range errs {
		m.logger.Warn("skipping unreadable host volume record", "volume_id", id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rec := range recs {
		if err := validateRecord(rec); err != nil {
			m.logger.Warn("skipping invalid host volume record", "volume_id", rec.VolumeID, "error", err)
			continue
		}
		m.nextRev++
		m.entries[rec.VolumeID] = &entry{
			rec: rec, rev: m.nextRev,
			stored: diag{rec.Status, rec.Reason, rec.Message},
			loaded: true,
		}
	}
}

func errClass(err error) string {
	return fmt.Sprintf("%T", err)
}

// validateRecord checks a record loaded from the store, including the plugin
// ID syntax and the built-in rules for a volume that uses the built-in plugin.
func validateRecord(r *hvstate.Record) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := validatePath(r.Path); err != nil {
		return err
	}
	if r.Kind == hvstate.KindPlugin {
		if err := ValidateID(r.PluginID); err != nil {
			return err
		}
		if r.PluginID == BuiltinMkdirID && !validBuiltinID(r.VolumeID) {
			return errors.New("volume id is not valid for the built-in plugin")
		}
	}
	return nil
}

// Run reconciles every interval until ctx is done, then returns.
func (m *Manager) Run(ctx context.Context) {
	t := time.NewTicker(m.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.reconcile(ctx, false)
		}
	}
}

// reconcile evaluates every record, then applies the results, resolves name
// conflicts, persists changed diagnostics and publishes a changed set.
// restoring runs create for every plugin-created volume.
func (m *Manager) reconcile(ctx context.Context, restoring bool) {
	results := make(map[string]result)
	for _, id := range m.ids() {
		if ctx.Err() != nil {
			break
		}
		if res, ok := m.evaluateOne(ctx, id, restoring); ok {
			results[id] = res
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		// Shutdown: do not publish or store a half-finished pass.
		return
	}
	if restoring && ctx.Err() != nil {
		// The restore deadline passed. A volume that was not verified is not
		// reported; the next periodic pass retries it.
		for _, id := range m.ids() {
			if _, done := results[id]; done {
				continue
			}
			if _, rev, ok := m.snapshot(id); ok {
				res := unavailable(hvstate.ReasonRestoreFailed, "the restore did not finish in time")
				res.rev, res.at = rev, m.cfg.Clock()
				results[id] = res
			}
		}
	}
	m.finalize(results)
	m.publishIfChanged()
}

func (m *Manager) ids() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Sorted(maps.Keys(m.entries))
}

// snapshot returns a copy of the record and its revision.
func (m *Manager) snapshot(id string) (*hvstate.Record, uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[id]
	if e == nil {
		return nil, 0, false
	}
	return e.rec.Copy(), e.rev, true
}

// evaluateOne takes the volume lock, re-reads the record (a delete may have
// removed it while this waited) and evaluates it.
func (m *Manager) evaluateOne(ctx context.Context, id string, restoring bool) (result, bool) {
	if err := m.locks.acquire(ctx, id); err != nil {
		return result{}, false
	}
	defer m.locks.release(id)

	rec, rev, ok := m.snapshot(id)
	if !ok {
		return result{}, false
	}
	// A volume created in this run was made by its plugin moments ago, so
	// only one loaded from the store is restored.
	res := m.evaluate(ctx, rec, restoring && m.loadedFromStore(id))
	res.rev = rev
	res.at = m.cfg.Clock()
	return res, true
}

func (m *Manager) loadedFromStore(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[id]
	return e != nil && e.loaded
}

func ready() result { return result{status: hvstate.StatusReady} }

func unavailable(reason, msg string) result {
	return result{status: hvstate.StatusUnavailable, reason: reason, message: msg}
}

// evaluate decides the status of one record. The volume lock is held.
func (m *Manager) evaluate(ctx context.Context, rec *hvstate.Record, restoring bool) result {
	if rec.NodeID != m.cfg.NodeID {
		return unavailable(hvstate.ReasonNodeMismatch, "the record belongs to another node")
	}
	if rec.Kind == hvstate.KindRegistered {
		return statPath(rec.Path)
	}

	// A volume whose plugin returned a different path is retried only at the
	// next client start. Every retry is another create, and a plugin that
	// keeps returning new paths would keep making storage that nothing
	// removes.
	if !restoring && rec.Status == hvstate.StatusUnavailable && rec.Reason == hvstate.ReasonPathChanged {
		return unavailable(rec.Reason, rec.Message)
	}

	// A ready volume, and one only held back by a name conflict, is just
	// checked. Create runs on restore and for any other unavailable volume.
	runCreate := restoring ||
		(rec.Status == hvstate.StatusUnavailable && rec.Reason != hvstate.ReasonNameConflict)
	if !runCreate {
		return statPath(rec.Path)
	}

	res, oerr := m.runCreate(ctx, rec)
	if oerr != nil {
		return oerr.result(rec.Parameters)
	}
	if res.Path != rec.Path {
		return unavailable(hvstate.ReasonPathChanged, "the plugin returned a different path")
	}
	return statPath(rec.Path)
}

// statPath checks that the path still exists. It never writes.
func statPath(p string) result {
	if err := validatePath(p); err != nil {
		return unavailable(hvstate.ReasonMissingPath, "the path is invalid")
	}
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return unavailable(hvstate.ReasonMissingPath, "the path does not exist")
		}
		return unavailable(hvstate.ReasonMissingPath, "the path cannot be inspected")
	}
	return ready()
}

// createFailure carries what a failed create tells the diagnostic.
type createFailure struct {
	reason  string
	message string
}

func (f *createFailure) result(params map[string]string) result {
	return unavailable(f.reason, boundMessage(f.message, params))
}

// boundMessage replaces parameter values and then bounds the text. The order
// matters: a cut value must not leave a prefix behind.
func boundMessage(msg string, params map[string]string) string {
	return sanitize([]byte(redact(msg, params)))
}

func requestOf(rec *hvstate.Record) OpRequest {
	return OpRequest{
		VolumeID: rec.VolumeID, Namespace: rec.Namespace, Name: rec.Name,
		NodeID: rec.NodeID, NodePool: rec.NodePool,
		CapacityMinBytes: rec.CapacityMinBytes, CapacityMaxBytes: rec.CapacityMaxBytes,
		Parameters: rec.Parameters,
	}
}

// runCreate runs the create operation of the record's plugin once.
func (m *Manager) runCreate(ctx context.Context, rec *hvstate.Record) (CreateResult, *createFailure) {
	req := requestOf(rec)
	var (
		res CreateResult
		err error
	)
	if rec.PluginID == BuiltinMkdirID {
		if !BuiltinSupported() {
			return CreateResult{}, &createFailure{hvstate.ReasonPluginUnavailable, "the built-in plugin is not supported on this platform"}
		}
		// A changed volumes directory would make the built-in plugin create a
		// second directory, so report the change instead.
		if filepath.Join(m.cfg.Ops.VolumesDir, rec.VolumeID) != rec.Path {
			return CreateResult{}, &createFailure{hvstate.ReasonPathChanged, "the volumes directory changed"}
		}
		res, err = m.cfg.Ops.CreateBuiltin(ctx, req)
	} else {
		cand, lerr := m.cfg.Lookup(ctx, rec.PluginID)
		if lerr != nil {
			return CreateResult{}, &createFailure{hvstate.ReasonPluginUnavailable, pluginMessage(lerr)}
		}
		res, err = m.cfg.Ops.Create(ctx, cand, req)
	}
	if err != nil {
		return CreateResult{}, &createFailure{hvstate.ReasonRestoreFailed, opMessage(err)}
	}
	return res, nil
}

func pluginMessage(err error) string {
	switch {
	case errors.Is(err, ErrPluginMissing):
		return ErrPluginMissing.Error()
	case errors.Is(err, ErrPluginUnhealthy):
		return ErrPluginUnhealthy.Error()
	}
	return "plugin cannot be used"
}

// opMessage describes a failed operation from its reason code and its
// already redacted message. Other errors are reduced to a fixed text.
func opMessage(err error) string {
	var oe *OpError
	if errors.As(err, &oe) {
		if oe.Message != "" {
			return oe.Reason + ": " + oe.Message
		}
		return oe.Reason
	}
	return "operation failed"
}

// finalize applies the evaluation results, resolves names, persists the
// diagnostics that changed and logs transitions.
func (m *Manager) finalize(results map[string]result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	before := make(map[string]diag, len(m.entries))
	for id, e := range m.entries {
		before[id] = diag{e.rec.Status, e.rec.Reason, e.rec.Message}
	}
	for id, res := range results {
		e := m.entries[id]
		if e == nil || e.rev != res.rev {
			continue
		}
		e.rec.Status, e.rec.Reason, e.rec.Message, e.rec.CheckedAt = res.status, res.reason, res.message, res.at
		e.checked = true
	}
	m.resolveNamesLocked()

	for _, id := range slices.Sorted(maps.Keys(m.entries)) {
		e := m.entries[id]
		now := diag{e.rec.Status, e.rec.Reason, e.rec.Message}
		if now != before[id] {
			m.logger.Info("host volume status changed", "volume_id", id, "name", e.rec.Name,
				"status", now.status, "reason", now.reason)
		}
		m.persistDiagLocked(e)
	}
}

// resolveNamesLocked gives each name to one volume. Among the volumes of
// this node that would be ready, a name used by a static volume goes to the
// static volume, and between dynamic volumes the lowest volume ID wins.
func (m *Manager) resolveNamesLocked() {
	owner := make(map[string]string)
	for _, id := range slices.Sorted(maps.Keys(m.entries)) {
		e := m.entries[id]
		if e.rec.NodeID != m.cfg.NodeID || e.rec.Status != hvstate.StatusReady {
			continue
		}
		switch {
		case m.cfg.StaticName(e.rec.Name):
			e.setUnavailable(hvstate.ReasonNameConflict, "the name is used by a static host volume")
		case owner[e.rec.Name] != "":
			e.setUnavailable(hvstate.ReasonNameConflict, "the name is used by another dynamic host volume")
		default:
			owner[e.rec.Name] = id
		}
	}
}

func (e *entry) setUnavailable(reason, msg string) {
	e.rec.Status, e.rec.Reason, e.rec.Message = hvstate.StatusUnavailable, reason, msg
}

// persistDiagLocked writes the record when its diagnostic differs from the
// stored one. CheckedAt is kept in memory on every pass and stored with the
// next change, which avoids a write every interval.
func (m *Manager) persistDiagLocked(e *entry) {
	now := diag{e.rec.Status, e.rec.Reason, e.rec.Message}
	if now == e.stored {
		return
	}
	if err := m.cfg.Store.PutHostVolume(e.rec); err != nil {
		m.logger.Error("failed to store host volume status", "volume_id", e.rec.VolumeID, "error", errClass(err))
		return
	}
	e.stored = now
}

// buildSetLocked returns the entries to publish: ready volumes of this node
// that this run has created or evaluated.
func (m *Manager) buildSetLocked() map[string]*structs.ClientHostVolumeConfig {
	set := make(map[string]*structs.ClientHostVolumeConfig)
	for _, id := range slices.Sorted(maps.Keys(m.entries)) {
		e := m.entries[id]
		r := e.rec
		if !e.checked || r.NodeID != m.cfg.NodeID || r.Status != hvstate.StatusReady || m.cfg.StaticName(r.Name) {
			continue
		}
		if _, taken := set[r.Name]; taken {
			continue
		}
		set[r.Name] = &structs.ClientHostVolumeConfig{Name: r.Name, Path: r.Path, ReadOnly: false, ID: r.VolumeID}
	}
	return set
}

// publishIfChanged calls Publish when the ready set differs from the last
// one published.
func (m *Manager) publishIfChanged() {
	m.pubMu.Lock()
	defer m.pubMu.Unlock()

	m.mu.Lock()
	next := m.buildSetLocked()
	m.mu.Unlock()

	if m.published == nil && len(next) == 0 {
		return
	}
	if sameSet(m.published, next) {
		return
	}
	m.published = next
	m.cfg.Publish(structs.CopyMapStringClientHostVolumeConfig(next))
}

func sameSet(a, b map[string]*structs.ClientHostVolumeConfig) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || *v != *w {
			return false
		}
	}
	return true
}

// nameTakenLocked reports whether another volume of this node, or a static
// volume, already has the name.
func (m *Manager) nameTakenLocked(name, exceptID string) bool {
	if m.cfg.StaticName(name) {
		return true
	}
	for id, e := range m.entries {
		if id != exceptID && e.rec.NodeID == m.cfg.NodeID && e.rec.Name == name {
			return true
		}
	}
	return false
}

// Create runs the plugin's create operation and, only when it succeeds,
// persists the record and publishes the volume. A failed create stores
// nothing. If the record cannot be stored after a successful create, the
// matching delete runs as a best-effort rollback and the store error is
// returned. The volume name must be unused on this node.
func (m *Manager) Create(ctx context.Context, pluginID string, req OpRequest) (VolumeStatus, error) {
	const op = OperationCreate
	m.ensureLoaded()
	if req.NodeID == "" {
		req.NodeID = m.cfg.NodeID
	}
	if req.NodeID != m.cfg.NodeID {
		return VolumeStatus{}, opErr(op, hvstate.ReasonNodeMismatch, "")
	}
	if err := ValidateID(pluginID); err != nil {
		return VolumeStatus{}, opErr(op, ReasonInvalidInput, "invalid plugin id")
	}
	if _, oerr := validateFields(op, req); oerr != nil {
		return VolumeStatus{}, oerr
	}

	if err := m.locks.acquire(ctx, req.VolumeID); err != nil {
		return VolumeStatus{}, opErr(op, ReasonCanceled, "")
	}
	defer m.locks.release(req.VolumeID)

	existed, oerr := m.precheck(op, req.VolumeID, req.Name, hvstate.KindPlugin)
	if oerr != nil {
		return VolumeStatus{}, oerr
	}

	rec := &hvstate.Record{
		VolumeID: req.VolumeID, Namespace: req.Namespace, Name: req.Name,
		NodeID: req.NodeID, NodePool: req.NodePool,
		Kind: hvstate.KindPlugin, PluginID: pluginID,
		CapacityMinBytes: req.CapacityMinBytes, CapacityMaxBytes: req.CapacityMaxBytes,
		Parameters: maps.Clone(req.Parameters),
		Status:     hvstate.StatusReady,
	}
	var (
		res CreateResult
		err error
	)
	if pluginID == BuiltinMkdirID {
		if !BuiltinSupported() {
			return VolumeStatus{}, opErr(op, hvstate.ReasonPluginUnavailable, "the built-in plugin is not supported on this platform")
		}
		res, err = m.cfg.Ops.CreateBuiltin(ctx, req)
	} else {
		cand, lerr := m.cfg.Lookup(ctx, pluginID)
		if lerr != nil {
			return VolumeStatus{}, opErr(op, hvstate.ReasonPluginUnavailable, pluginMessage(lerr))
		}
		res, err = m.cfg.Ops.Create(ctx, cand, req)
	}
	if err != nil {
		return VolumeStatus{}, err
	}
	rec.Path = res.Path
	rec.CheckedAt = m.cfg.Clock()

	if err := m.commit(op, rec); err != nil {
		// A volume that already had a record is not removed: the create
		// reused it.
		if !existed {
			m.rollbackCreate(rec)
		}
		return VolumeStatus{}, err
	}
	m.finalize(nil)
	m.publishIfChanged()
	st, _ := m.Status(rec.VolumeID)
	return st, nil
}

// precheck rejects a request that would clash with an existing record. The
// volume lock is held. It reports whether a record for the ID exists.
func (m *Manager) precheck(op, id, name string, kind hvstate.Kind) (bool, *OpError) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[id]
	if e != nil && e.rec.Kind != kind {
		return true, opErr(op, ReasonInvalidInput, "volume id is used by a volume of another kind")
	}
	if m.nameTakenLocked(name, id) {
		return e != nil, opErr(op, hvstate.ReasonNameConflict, "the name is already used on this node")
	}
	return e != nil, nil
}

// commit stores the record and makes it visible. The name is checked again
// because a volume with another ID may have taken it since precheck.
func (m *Manager) commit(op string, rec *hvstate.Record) error {
	if err := validateRecord(rec); err != nil {
		return opErr(op, ReasonInvalidPath, err.Error())
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nameTakenLocked(rec.Name, rec.VolumeID) {
		return opErr(op, hvstate.ReasonNameConflict, "the name is already used on this node")
	}
	if err := m.cfg.Store.PutHostVolume(rec); err != nil {
		return fmt.Errorf("host volume %s: store record: %s", rec.VolumeID, errClass(err))
	}
	m.nextRev++
	m.entries[rec.VolumeID] = &entry{rec: rec, rev: m.nextRev, stored: diag{rec.Status, rec.Reason, rec.Message}, checked: true}
	return nil
}

// rollbackCreate removes what a create made when the record was not stored.
// It is best effort: a failure is logged.
func (m *Manager) rollbackCreate(rec *hvstate.Record) {
	// A background context is deliberate: the caller's may be done, and the
	// delete is bounded by the Ops timeout.
	ctx := context.Background()
	if err := m.runDelete(ctx, rec); err != nil {
		m.logger.Warn("failed to roll back host volume create", "volume_id", rec.VolumeID, "error", opMessage(err))
	}
}

// runDelete runs the delete operation of the record's plugin.
func (m *Manager) runDelete(ctx context.Context, rec *hvstate.Record) error {
	req := requestOf(rec)
	if rec.PluginID == BuiltinMkdirID {
		if !BuiltinSupported() {
			return opErr(OperationDelete, hvstate.ReasonPluginUnavailable, "the built-in plugin is not supported on this platform")
		}
		return m.cfg.Ops.DeleteBuiltin(ctx, req)
	}
	cand, err := m.cfg.Lookup(ctx, rec.PluginID)
	if err != nil {
		return opErr(OperationDelete, hvstate.ReasonPluginUnavailable, pluginMessage(err))
	}
	return m.cfg.Ops.Delete(ctx, cand, req, rec.Path)
}

// Delete runs the delete operation of a plugin-created volume, then removes
// the record and unpublishes the volume. A failed delete keeps the record
// and its status. Removing a registered record is not supported here and
// changes nothing.
func (m *Manager) Delete(ctx context.Context, volumeID string) error {
	const op = OperationDelete
	m.ensureLoaded()
	if err := m.locks.acquire(ctx, volumeID); err != nil {
		return opErr(op, ReasonCanceled, "")
	}
	defer m.locks.release(volumeID)

	rec, _, ok := m.snapshot(volumeID)
	if !ok {
		return opErr(op, ReasonInvalidInput, "unknown volume")
	}
	if rec.Kind == hvstate.KindRegistered {
		return ErrRegisteredVolume
	}
	if rec.NodeID != m.cfg.NodeID {
		return opErr(op, hvstate.ReasonNodeMismatch, "")
	}
	if err := m.runDelete(ctx, rec); err != nil {
		return err
	}

	m.mu.Lock()
	if err := m.cfg.Store.DeleteHostVolume(volumeID); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("host volume %s: remove record: %s", volumeID, errClass(err))
	}
	delete(m.entries, volumeID)
	m.nextRev++
	m.mu.Unlock()

	m.publishIfChanged()
	return nil
}

// AddRegistered records an existing external path as a volume. It is Wander
// policy and never writes to the path: it only inspects it with os.Stat.
//
// Checks run in this order and the first failure decides the reason code:
// field validation (invalid_input, invalid_path), the node (node_mismatch),
// the rules for an existing record with the same ID, the name
// (name_conflict) and last that the path exists (missing_path). Repeating a
// registration with the same ID and the same namespace, name and path
// succeeds at once, returns the current status and writes and publishes
// nothing, even if the path has since disappeared. A repeat that differs
// in namespace, name or path is rejected with ReasonRegistrationMismatch. An
// ID that belongs to a plugin-created volume is rejected with
// ReasonVolumeKindConflict. NodePool is not compared on a repeat. The
// request's NodeID must be this node; it is never defaulted.
func (m *Manager) AddRegistered(ctx context.Context, req RegisterRequest) (VolumeStatus, error) {
	const op = "register"
	m.ensureLoaded()

	// Validate before the lock so that untrusted input is bounded before it
	// becomes a lock table key.
	rec := &hvstate.Record{
		VolumeID: req.VolumeID, Namespace: req.Namespace, Name: req.Name,
		NodeID: req.NodeID, NodePool: req.NodePool,
		Kind: hvstate.KindRegistered, Path: req.Path, Status: hvstate.StatusReady,
	}
	if req.Namespace == "" {
		return VolumeStatus{}, opErr(op, ReasonInvalidInput, "namespace is empty")
	}
	if err := validatePath(req.Path); err != nil {
		return VolumeStatus{}, opErr(op, ReasonInvalidPath, err.Error())
	}
	if err := rec.Validate(); err != nil {
		return VolumeStatus{}, opErr(op, ReasonInvalidInput, err.Error())
	}
	if req.NodeID != m.cfg.NodeID {
		return VolumeStatus{}, opErr(op, hvstate.ReasonNodeMismatch, "")
	}

	if err := m.locks.acquire(ctx, req.VolumeID); err != nil {
		return VolumeStatus{}, opErr(op, ReasonCanceled, "")
	}
	defer m.locks.release(req.VolumeID)

	if old, _, ok := m.snapshot(req.VolumeID); ok {
		switch {
		case old.Kind != hvstate.KindRegistered:
			return VolumeStatus{}, opErr(op, ReasonVolumeKindConflict, "volume id is used by a volume of another kind")
		case old.NodeID == m.cfg.NodeID && old.Namespace == req.Namespace && old.Name == req.Name && old.Path == req.Path:
			return statusOf(old), nil
		default:
			return VolumeStatus{}, opErr(op, ReasonRegistrationMismatch, "volume id is registered with other values")
		}
	}

	m.mu.Lock()
	taken := m.nameTakenLocked(rec.Name, rec.VolumeID)
	m.mu.Unlock()
	if taken {
		return VolumeStatus{}, opErr(op, hvstate.ReasonNameConflict, "the name is already used on this node")
	}
	if res := statPath(rec.Path); res.status != hvstate.StatusReady {
		return VolumeStatus{}, opErr(op, res.reason, res.message)
	}

	rec.CheckedAt = m.cfg.Clock()
	if err := m.commit(op, rec); err != nil {
		return VolumeStatus{}, err
	}
	m.finalize(nil)
	m.publishIfChanged()
	st, _ := m.Status(rec.VolumeID)
	return st, nil
}

// Status returns a copy of the last known state of a volume.
func (m *Manager) Status(volumeID string) (VolumeStatus, bool) {
	m.ensureLoaded()
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[volumeID]
	if e == nil {
		return VolumeStatus{}, false
	}
	return statusOf(e.rec), true
}

// Statuses returns copies of the state of every volume, sorted by volume ID.
func (m *Manager) Statuses() []VolumeStatus {
	m.ensureLoaded()
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]VolumeStatus, 0, len(m.entries))
	for _, id := range slices.Sorted(maps.Keys(m.entries)) {
		out = append(out, statusOf(m.entries[id].rec))
	}
	return out
}

func statusOf(r *hvstate.Record) VolumeStatus {
	return VolumeStatus{
		VolumeID: r.VolumeID, Name: r.Name, Path: r.Path, Kind: r.Kind,
		Status: r.Status, Reason: r.Reason, Message: r.Message, CheckedAt: r.CheckedAt,
	}
}

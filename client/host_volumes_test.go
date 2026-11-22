// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/allocrunner"
	arinterfaces "github.com/open-wander/wander/client/allocrunner/interfaces"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/hostvolume"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	regMock "github.com/open-wander/wander/client/serviceregistration/mock"
	"github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/command/agent/consul"
	"github.com/open-wander/wander/helper/pluginutils/catalog"
	"github.com/open-wander/wander/helper/pluginutils/singleton"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
	"github.com/shoenig/test/wait"
)

func hostVolumeTestClient(static map[string]*structs.ClientHostVolumeConfig) *Client {
	node := &structs.Node{HostVolumes: static}
	return &Client{
		config:            &config.Config{Node: node, HostVolumes: static},
		logger:            hclog.NewInterceptLogger(nil),
		triggerNodeUpdate: make(chan struct{}, 16),
	}
}

func TestClient_updateNodeFromHostVolumes(t *testing.T) {
	ci.Parallel(t)

	static := map[string]*structs.ClientHostVolumeConfig{
		"static": {Name: "static", Path: "/synthetic/static", ReadOnly: true},
	}
	c := hostVolumeTestClient(static)
	before := c.config.Node.HostVolumes

	dyn := map[string]*structs.ClientHostVolumeConfig{
		"dyn":    {Name: "dyn", Path: "/synthetic/dyn", ID: "vol-a"},
		"static": {Name: "static", Path: "/synthetic/hijack", ID: "vol-b"},
		"noid":   {Name: "noid", Path: "/synthetic/noid"},
	}
	c.updateNodeFromHostVolumes(dyn)

	got := c.config.Node.HostVolumes
	must.MapLen(t, 2, got)
	must.Eq(t, structs.ClientHostVolumeConfig{Name: "dyn", Path: "/synthetic/dyn", ID: "vol-a"}, *got["dyn"])
	must.Eq(t, structs.ClientHostVolumeConfig{Name: "static", Path: "/synthetic/static", ReadOnly: true}, *got["static"])
	must.Eq(t, 1, len(c.triggerNodeUpdate))

	// The map that was shared before is not changed in place.
	must.MapLen(t, 1, before)

	// The same set again is not a change.
	c.updateNodeFromHostVolumes(map[string]*structs.ClientHostVolumeConfig{
		"dyn": {Name: "dyn", Path: "/synthetic/dyn", ID: "vol-a"},
	})
	must.Eq(t, 1, len(c.triggerNodeUpdate))

	// A changed path is a change.
	c.updateNodeFromHostVolumes(map[string]*structs.ClientHostVolumeConfig{
		"dyn": {Name: "dyn", Path: "/synthetic/dyn2", ID: "vol-a"},
	})
	must.Eq(t, 2, len(c.triggerNodeUpdate))
	must.Eq(t, "/synthetic/dyn2", c.config.Node.HostVolumes["dyn"].Path)

	// Removing the dynamic volume leaves the static one.
	c.updateNodeFromHostVolumes(nil)
	must.Eq(t, 3, len(c.triggerNodeUpdate))
	must.MapLen(t, 1, c.config.Node.HostVolumes)
	must.Eq(t, "/synthetic/static", c.config.Node.HostVolumes["static"].Path)
	c.updateNodeFromHostVolumes(nil)
	must.Eq(t, 3, len(c.triggerNodeUpdate))
}

func TestClient_updateNodeFromHostVolumes_NoStatic(t *testing.T) {
	ci.Parallel(t)

	c := hostVolumeTestClient(nil)
	c.updateNodeFromHostVolumes(nil)
	must.Eq(t, 0, len(c.triggerNodeUpdate))
	c.updateNodeFromHostVolumes(map[string]*structs.ClientHostVolumeConfig{"d": {Name: "d", Path: "/synthetic/d", ID: "v"}})
	must.Eq(t, 1, len(c.triggerNodeUpdate))
	c.updateNodeFromHostVolumes(nil)
	must.Eq(t, 2, len(c.triggerNodeUpdate))
	must.Nil(t, c.config.Node.HostVolumes)
}

// The client restores the stored records at start, and the volumes the
// manager makes ready appear in the node next to the static ones.
func TestClient_DynamicHostVolumesWiring(t *testing.T) {
	ci.Parallel(t)

	staticDir, dynDir := t.TempDir(), t.TempDir()
	db := state.NewMemDB(hclog.NewNullLogger())
	// The node ID is generated at start, so this record is for another node.
	must.NoError(t, db.PutHostVolume(&hvstate.Record{
		VolumeID: "vol-x", Namespace: "default", Name: "foreign", NodeID: uuid.Generate(),
		Kind: hvstate.KindRegistered, Path: t.TempDir(), Status: hvstate.StatusReady,
	}))

	c, cleanup := TestClient(t, func(conf *config.Config) {
		conf.HostVolumes = map[string]*structs.ClientHostVolumeConfig{
			"static": {Name: "static", Path: staticDir},
		}
		conf.StateDBFactory = func(hclog.Logger, string) (state.StateDB, error) { return db, nil }
	})
	t.Cleanup(func() { _ = cleanup() })

	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	waitFor("restore", func() bool {
		st, ok := c.hostVolumeManager.Status("vol-x")
		return ok && st.Reason == hvstate.ReasonNodeMismatch
	})
	must.MapLen(t, 1, c.GetConfig().Node.HostVolumes)

	ctx := context.Background()
	_, err := c.hostVolumeManager.AddRegistered(ctx, hostvolume.RegisterRequest{NodeID: c.NodeID(), Namespace: "default", VolumeID: "vol-a", Name: "static", Path: dynDir})
	must.Error(t, err)

	_, err = c.hostVolumeManager.AddRegistered(ctx, hostvolume.RegisterRequest{NodeID: c.NodeID(), Namespace: "default", VolumeID: "vol-a", Name: "dyn", Path: dynDir})
	must.NoError(t, err)
	hv := c.GetConfig().Node.HostVolumes
	must.Eq(t, structs.ClientHostVolumeConfig{Name: "dyn", Path: dynDir, ID: "vol-a"}, *hv["dyn"])
	must.Eq(t, structs.ClientHostVolumeConfig{Name: "static", Path: staticDir}, *hv["static"])
	must.Nil(t, hv["foreign"])
}

// The task runners see the volumes the node offers now, and a dynamic volume
// only in the namespace of its record.
func TestClient_hostVolumesFor(t *testing.T) {
	ci.Parallel(t)

	static := map[string]*structs.ClientHostVolumeConfig{
		"static": {Name: "static", Path: "/synthetic/static"},
	}
	c := hostVolumeTestClient(static)
	m, err := hostvolume.NewManager(hostvolume.ManagerConfig{
		Store: state.NewMemDB(hclog.NewNullLogger()), Ops: &hostvolume.Ops{}, NodeID: "node-synthetic",
		Publish: c.updateNodeFromHostVolumes,
	})
	must.NoError(t, err)
	c.hostVolumeManager = m
	// Only volumes the manager has checked are published.
	m.Restore(context.Background())

	for name, ns := range map[string]string{"data-a": "ns-a", "data-b": "ns-b"} {
		_, err := m.AddRegistered(context.Background(), hostvolume.RegisterRequest{
			NodeID: "node-synthetic", Namespace: ns, VolumeID: "vol-" + name, Name: name, Path: t.TempDir(),
		})
		must.NoError(t, err)
	}
	// An entry whose record is gone, as after a delete between two reads.
	// The node's map is never changed in place, so a new one is set.
	c.configLock.Lock()
	withGone := c.config.Copy()
	withGone.Node.HostVolumes = maps.Clone(c.config.Node.HostVolumes)
	withGone.Node.HostVolumes["data-gone"] = &structs.ClientHostVolumeConfig{Name: "data-gone", Path: "/synthetic/gone", ID: "vol-gone"}
	c.config = withGone
	c.configLock.Unlock()

	got := c.hostVolumesFor("ns-a")
	must.MapContainsKeys(t, got, []string{"static", "data-a"})
	must.MapLen(t, 2, got)
	must.Eq(t, "/synthetic/static", got["static"].Path)

	got = c.hostVolumesFor("ns-b")
	must.MapContainsKeys(t, got, []string{"static", "data-b"})
	must.MapLen(t, 2, got)

	got = c.hostVolumesFor("ns-other")
	must.MapLen(t, 1, got)
	must.MapContainsKey(t, got, "static")

	// The set of the node is not changed by the lookups.
	must.MapLen(t, 4, c.GetConfig().Node.HostVolumes)
}

func TestClient_hostVolumesFor_NoManager(t *testing.T) {
	ci.Parallel(t)

	static := map[string]*structs.ClientHostVolumeConfig{
		"static": {Name: "static", Path: "/synthetic/static"},
	}
	c := hostVolumeTestClient(static)
	must.Eq(t, static, c.hostVolumesFor("ns-a"))

	// Without a manager nothing is ever restored, so nothing may wait.
	select {
	case <-c.hostVolumesRestored():
	default:
		t.Fatal("expected a closed channel without a manager")
	}
}

func TestClient_hostVolumesRestored(t *testing.T) {
	ci.Parallel(t)

	c := hostVolumeTestClient(nil)
	m, err := hostvolume.NewManager(hostvolume.ManagerConfig{
		Store: state.NewMemDB(hclog.NewNullLogger()), Ops: &hostvolume.Ops{}, NodeID: "node-synthetic",
		Publish: c.updateNodeFromHostVolumes,
	})
	must.NoError(t, err)
	c.hostVolumeManager = m

	select {
	case <-c.hostVolumesRestored():
		t.Fatal("closed before the restore")
	default:
	}
	m.Restore(context.Background())
	select {
	case <-c.hostVolumesRestored():
	default:
		t.Fatal("not closed after the restore")
	}
}

// hostVolumeClientAlloc returns an allocation of the namespace whose group
// requests the host volume data-a without mounting it: the mock driver cannot
// mount, so the request alone tells a present volume from a missing one, and
// a missing volume fails the task for good.
func hostVolumeClientAlloc(ns string) *structs.Allocation {
	alloc := mock.BatchAlloc()
	alloc.Namespace = ns
	alloc.Job.Namespace = ns
	tg := alloc.Job.TaskGroups[0]
	tg.Count = 1
	tg.RestartPolicy = &structs.RestartPolicy{Attempts: 0, Mode: structs.RestartPolicyModeFail}
	tg.Volumes = map[string]*structs.VolumeRequest{
		"data": {Name: "data", Type: structs.VolumeTypeHost, Source: "data-a"},
	}
	task := tg.Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{"run_for": "1m"}
	task.RestartPolicy = &structs.RestartPolicy{Attempts: 0, Mode: structs.RestartPolicyModeFail}
	task.VolumeMounts = nil
	// A restored logmon waits for the output pipes of a task that the mock
	// driver never reopens, which would hold the volume hook back for its
	// timeout.
	task.LogConfig = &structs.LogConfig{Disabled: true}
	return alloc
}

// hostVolumeTaskState reads the persisted state of the only task of the
// allocation. The alloc runner is not asked: its AllocState writes without
// the lock before the first task update.
func hostVolumeTaskState(c *Client, alloc *structs.Allocation) *structs.TaskState {
	_, ts, err := c.stateDB.GetTaskRunnerState(alloc.ID, alloc.Job.TaskGroups[0].Tasks[0].Name)
	if err != nil {
		return nil
	}
	return ts
}

func hostVolumeFailureEvents(ts *structs.TaskState) []string {
	var msgs []string
	for _, ev := range ts.Events {
		if ev.Type == structs.TaskHookFailed {
			msgs = append(msgs, ev.DisplayMessage)
		}
	}
	return msgs
}

func destroyClientAllocs(t *testing.T, c *Client) {
	t.Helper()
	for _, ar := range c.getAllocRunners() {
		ar.Destroy()
	}
	for _, ar := range c.getAllocRunners() {
		<-ar.DestroyCh()
	}
}

func waitForHostVolumeRestore(t *testing.T, c *Client) {
	t.Helper()
	select {
	case <-c.hostVolumesRestored():
	case <-time.After(15 * time.Second):
		t.Fatal("the host volume restore did not end")
	}
}

// A new allocation goes through the namespace filter of the client: the
// volume of a namespace is mounted there and is missing in another one.
func TestClient_HostVolumes_NewAllocationUsesNamespaceFilter(t *testing.T) {
	ci.Parallel(t)

	// The task states are read from the state database of the client, which
	// development mode replaces with a no-op one.
	c, cleanup := TestClient(t, func(conf *config.Config) { conf.DevMode = false })
	t.Cleanup(func() {
		destroyClientAllocs(t, c)
		_ = cleanup()
	})
	waitForHostVolumeRestore(t, c)

	_, err := c.hostVolumeManager.AddRegistered(context.Background(), hostvolume.RegisterRequest{
		NodeID: c.NodeID(), Namespace: "ns-a", VolumeID: "vol-a", Name: "data-a", Path: t.TempDir(),
	})
	must.NoError(t, err)

	own, other := hostVolumeClientAlloc("ns-a"), hostVolumeClientAlloc("ns-b")
	must.NoError(t, c.addAlloc(own, ""))
	must.NoError(t, c.addAlloc(other, ""))

	deadline := time.Now().Add(15 * time.Second)
	for {
		ownState, otherState := hostVolumeTaskState(c, own), hostVolumeTaskState(c, other)
		if ownState != nil && ownState.State == structs.TaskStateDead {
			t.Fatalf("the allocation of the namespace of the volume died: %v", hostVolumeFailureEvents(ownState))
		}
		if ownState != nil && otherState != nil &&
			ownState.State == structs.TaskStateRunning && otherState.State == structs.TaskStateDead {
			must.True(t, otherState.Failed)
			failures := hostVolumeFailureEvents(otherState)
			must.Len(t, 1, failures)
			must.True(t, strings.Contains(failures[0], "missing data-a"), must.Sprintf("got %q", failures[0]))
			must.True(t, strings.Contains(failures[0], "host volume validation error"))
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out: own=%v other=%v", ownState, otherState)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// hostVolumeGate holds back the read of the host volume records, and with it
// the first restore of the volume manager, while it is held.
type hostVolumeGate struct {
	mu   sync.Mutex
	gate chan struct{}
}

func (g *hostVolumeGate) hold() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.gate = make(chan struct{})
}

func (g *hostVolumeGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gate != nil {
		close(g.gate)
		g.gate = nil
	}
}

func (g *hostVolumeGate) wait() {
	g.mu.Lock()
	gate := g.gate
	g.mu.Unlock()
	if gate != nil {
		<-gate
	}
}

// gatedHostVolumeStore is a client state database whose read of the host
// volume records waits for the gate.
type gatedHostVolumeStore struct {
	state.StateDB
	gate *hostVolumeGate
}

func (s *gatedHostVolumeStore) GetHostVolumes() ([]*hvstate.Record, map[string]error, error) {
	s.gate.wait()
	return s.StateDB.GetHostVolumes()
}

// An allocation restored from the client state database gets the filtered
// getter and the restore channel of the client: the client restores its
// allocations before the volume manager has published the dynamic volume, so
// only the wait for the restore, and then the filtered set, lets the task
// pass the volume hook.
func TestClient_HostVolumes_RestoredAllocationUsesFilterAndWaits(t *testing.T) {
	ci.Parallel(t)

	gate := &hostVolumeGate{}
	t.Cleanup(gate.release)

	var getterCalls, restoredCalls atomic.Int32
	factory := func(arc *config.AllocRunnerConfig) (arinterfaces.AllocRunner, error) {
		// Count the calls of the functions the client injects; a missing
		// function stays missing so that the break is not hidden.
		if inner := arc.HostVolumes; inner != nil {
			arc.HostVolumes = func(ns string) map[string]*structs.ClientHostVolumeConfig {
				getterCalls.Add(1)
				return inner(ns)
			}
		}
		if inner := arc.HostVolumesRestored; inner != nil {
			arc.HostVolumesRestored = func() <-chan struct{} {
				restoredCalls.Add(1)
				return inner()
			}
		}
		return allocrunner.NewAllocRunner(arc)
	}

	c1, cleanup1 := TestClient(t, func(conf *config.Config) {
		conf.DevMode = false
		// The state is kept in the state directory, which the second client
		// shares, and not in memory: closing a memory database drops it.
		conf.StateDBFactory = func(l hclog.Logger, dir string) (state.StateDB, error) {
			db, err := state.NewBoltStateDB(l, dir)
			if err != nil {
				return nil, err
			}
			return &gatedHostVolumeStore{StateDB: db, gate: gate}, nil
		}
		conf.AllocRunnerFactory = factory
	})
	t.Cleanup(func() { _ = cleanup1() })
	waitForHostVolumeRestore(t, c1)

	_, err := c1.hostVolumeManager.AddRegistered(context.Background(), hostvolume.RegisterRequest{
		NodeID: c1.NodeID(), Namespace: "ns-a", VolumeID: "vol-a", Name: "data-a", Path: t.TempDir(),
	})
	must.NoError(t, err)

	alloc := hostVolumeClientAlloc("ns-a")
	must.NoError(t, c1.addAlloc(alloc, ""))
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		ts := hostVolumeTaskState(c1, alloc)
		if ts == nil || ts.State != structs.TaskStateRunning {
			return fmt.Errorf("task is not running yet: %v", ts)
		}
		return nil
	}), wait.Timeout(15*time.Second), wait.Gap(10*time.Millisecond)))
	must.Zero(t, restoredCalls.Load())
	must.NoError(t, c1.Shutdown())

	// The second client finds the allocation in the state database, and its
	// volume manager cannot restore until the store is released.
	gate.hold()
	getterCalls.Store(0)
	// A restarted agent starts from its configuration, not from the node of
	// the previous run, so the dynamic volume is not in the node yet.
	cfg2 := c1.GetConfig().Copy()
	cfg2.Node.HostVolumes = nil
	cfg2.Logger = testlog.HCLogger(t)
	cfg2.PluginLoader = catalog.TestPluginLoaderWithOptions(t, "", cfg2.Options, nil)
	cfg2.PluginSingletonLoader = singleton.NewSingletonLoader(cfg2.Logger, cfg2.PluginLoader)
	c2, err := NewClient(cfg2, consul.NewMockCatalog(cfg2.Logger), nil, regMock.NewServiceRegistrationHandler(cfg2.Logger), nil)
	must.NoError(t, err)
	t.Cleanup(func() {
		// Shutdown, not destroy: it waits for the task runners to leave
		// their run loops, which a kill that races with the last state
		// update of a restored task does not.
		gate.release()
		_ = c2.Shutdown()
	})
	must.Eq(t, c1.NodeID(), c2.NodeID())

	// The hook reaches the wait on the channel of the client, and the task
	// has not been failed while the volume is missing.
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		if restoredCalls.Load() == 0 {
			ts := hostVolumeTaskState(c2, alloc)
			if ts != nil && ts.State == structs.TaskStateDead {
				t.Fatalf("the restored task died instead of waiting for the restore: %v", hostVolumeFailureEvents(ts))
			}
			return fmt.Errorf("the hook has not reached the wait")
		}
		return nil
	}), wait.Timeout(15*time.Second), wait.Gap(10*time.Millisecond)))
	select {
	case <-c2.hostVolumesRestored():
		t.Fatal("the restore ended while the store is held")
	default:
	}
	waitingCalls := getterCalls.Load()
	ts := hostVolumeTaskState(c2, alloc)
	must.NotNil(t, ts)
	must.NotEq(t, structs.TaskStateDead, ts.State)
	must.Len(t, 0, hostVolumeFailureEvents(ts))

	// Releasing the store ends the restore; the hook looks again, with the
	// namespace filter, and finds the volume.
	gate.release()
	waitForHostVolumeRestore(t, c2)
	must.Wait(t, wait.InitialSuccess(wait.ErrorFunc(func() error {
		if getterCalls.Load() <= waitingCalls {
			return fmt.Errorf("the hook has not looked again")
		}
		return nil
	}), wait.Timeout(15*time.Second), wait.Gap(10*time.Millisecond)))
	must.MapContainsKey(t, c2.hostVolumesFor("ns-a"), "data-a")
	must.MapNotContainsKey(t, c2.hostVolumesFor("ns-b"), "data-a")

	ts = hostVolumeTaskState(c2, alloc)
	must.NotNil(t, ts)
	must.Eq(t, structs.TaskStateRunning, ts.State)
	must.False(t, ts.Failed)
	must.Len(t, 0, hostVolumeFailureEvents(ts))
}

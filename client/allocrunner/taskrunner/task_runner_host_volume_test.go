// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/allocrunner/interfaces"
	cstate "github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/stretchr/testify/require"
)

// hostVolumeAlloc returns a batch allocation whose group requests the host
// volume data-a without mounting it into the task: the mock driver cannot
// mount, so the request alone is what tells a present volume from a missing
// one.
func hostVolumeAlloc(restart *structs.RestartPolicy) *structs.Allocation {
	alloc := mock.BatchAlloc()
	alloc.Job.TaskGroups[0].Count = 1
	if restart != nil {
		alloc.Job.TaskGroups[0].RestartPolicy = restart
	}
	alloc.Job.TaskGroups[0].Volumes = map[string]*structs.VolumeRequest{
		"data": {Name: "data", Type: structs.VolumeTypeHost, Source: "data-a"},
	}
	task := alloc.Job.TaskGroups[0].Tasks[0]
	task.Driver = "mock_driver"
	task.Config = map[string]interface{}{"run_for": "1m"}
	task.VolumeMounts = nil
	return alloc
}

func presentHostVolumes() map[string]*structs.ClientHostVolumeConfig {
	return map[string]*structs.ClientHostVolumeConfig{
		"data-a": {Name: "data-a", Path: "/synthetic/data-a"},
	}
}

// signalledRestore returns a restore getter whose channel never closes and a
// channel that is closed once the getter has been called, which is the
// moment before the hook blocks on the restore.
func signalledRestore() (func() <-chan struct{}, <-chan struct{}) {
	var once sync.Once
	waiting := make(chan struct{})
	never := make(chan struct{})
	return func() <-chan struct{} {
		once.Do(func() { close(waiting) })
		return never
	}, waiting
}

// waitForHookToWait waits until the volume hook has reached the wait for the
// restore. It fails instead of hanging when the hook never gets there.
func waitForHookToWait(t *testing.T, waiting <-chan struct{}) {
	t.Helper()
	select {
	case <-waiting:
	case <-time.After(10 * time.Second):
		t.Fatal("the volume hook did not reach the wait for the restore")
	}
}

var hostVolumeRestartPolicies = map[string]*structs.RestartPolicy{
	"no restarts": {Attempts: 0, Mode: structs.RestartPolicyModeFail},
	"default":     nil,
}

func requireNoVolumeFailure(t *testing.T, state *structs.TaskState) {
	t.Helper()
	require.False(t, state.Failed)
	require.Zero(t, state.Restarts)
	for _, ev := range state.Events {
		require.NotEqual(t, structs.TaskHookFailed, ev.Type, ev.DisplayMessage)
		require.NotEqual(t, structs.TaskNotRestarting, ev.Type, ev.DisplayMessage)
		require.NotEqual(t, structs.TaskRestarting, ev.Type, ev.DisplayMessage)
	}
}

// TestTaskRunner_HostVolume_KillDuringWait asserts that a job stop that
// arrives while the volume hook waits for the restore ends the task as
// killed, not as failed with a hook error.
func TestTaskRunner_HostVolume_KillDuringWait(t *testing.T) {
	ci.Parallel(t)

	for name, policy := range hostVolumeRestartPolicies {
		t.Run(name, func(t *testing.T) {
			alloc := hostVolumeAlloc(policy)
			task := alloc.Job.TaskGroups[0].Tasks[0]
			conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
			defer cleanup()
			conf.StateDB = cstate.NewMemDB(conf.Logger)
			conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return nil }
			restored, waiting := signalledRestore()
			conf.HostVolumesRestored = restored

			tr, err := NewTaskRunner(conf)
			require.NoError(t, err)
			go tr.Run()

			waitForHookToWait(t, waiting)
			require.NoError(t, tr.Kill(context.Background(), structs.NewTaskEvent(structs.TaskKilling)))

			state := tr.TaskState()
			require.Equal(t, structs.TaskStateDead, state.State)
			requireNoVolumeFailure(t, state)
			var killing bool
			for _, ev := range state.Events {
				killing = killing || ev.Type == structs.TaskKilling
			}
			require.True(t, killing, "the kill event of the caller is recorded")
		})
	}
}

// TestTaskRunner_HostVolume_ShutdownDuringWait asserts that stopping the
// agent while the volume hook waits for the restore leaves the task for the
// next agent start instead of failing it.
func TestTaskRunner_HostVolume_ShutdownDuringWait(t *testing.T) {
	ci.Parallel(t)

	for name, policy := range hostVolumeRestartPolicies {
		t.Run(name, func(t *testing.T) {
			alloc := hostVolumeAlloc(policy)
			task := alloc.Job.TaskGroups[0].Tasks[0]
			conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
			defer cleanup()
			conf.StateDB = cstate.NewMemDB(conf.Logger)
			conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return nil }
			restored, waiting := signalledRestore()
			conf.HostVolumesRestored = restored

			tr, err := NewTaskRunner(conf)
			require.NoError(t, err)
			go tr.Run()

			waitForHookToWait(t, waiting)
			tr.Shutdown()

			state := tr.TaskState()
			require.NotEqual(t, structs.TaskStateDead, state.State)
			requireNoVolumeFailure(t, state)
			for _, ev := range state.Events {
				require.NotEqual(t, structs.TaskKilling, ev.Type)
				require.NotEqual(t, structs.TaskKilled, ev.Type)
			}

			local, persisted, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
			require.NoError(t, err)
			require.False(t, local.RunComplete)
			require.NotNil(t, persisted)
			require.NotEqual(t, structs.TaskStateDead, persisted.State)
			require.False(t, persisted.Failed)
		})
	}
}

// TestTaskRunner_HostVolume_ShutdownDuringWaitKeepsRestoredTask asserts that
// a task restored with a running driver handle is left running when the
// agent stops again while the volume hook waits, and that the next start
// runs the hook again.
func TestTaskRunner_HostVolume_ShutdownDuringWaitKeepsRestoredTask(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeAlloc(hostVolumeRestartPolicies["no restarts"])
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	conf.StateDB = cstate.NewMemDB(conf.Logger)

	// First run: the volume is present, the task starts.
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return presentHostVolumes() }
	origTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go origTR.Run()
	defer origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))
	testWaitForTaskToStart(t, origTR)
	origTR.Shutdown()

	// Second run: restored before the volume is back, stopped while waiting.
	restored, waiting := signalledRestore()
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return nil }
	conf.HostVolumesRestored = restored
	waitTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	require.NoError(t, waitTR.Restore())
	go waitTR.Run()
	waitForHookToWait(t, waiting)
	waitTR.Shutdown()

	handle := waitTR.getDriverHandle()
	require.NotNil(t, handle, "the handle of the restored task is kept")
	status, err := waitTR.driver.InspectTask(handle.ID())
	require.NoError(t, err)
	require.Equal(t, "running", string(status.State))
	state := waitTR.TaskState()
	require.NotEqual(t, structs.TaskStateDead, state.State)
	requireNoVolumeFailure(t, state)

	// Third run: the volume is back, the hook runs again and the task is
	// still the one that was started once.
	var lookups atomic.Int32
	conf.HostVolumesRestored = nil
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig {
		lookups.Add(1)
		return presentHostVolumes()
	}
	newTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	require.NoError(t, newTR.Restore())
	// The poststart hooks run once the task runner has handled the driver, so
	// the state is not written any more while the test reads it.
	signal := &poststartSignalHook{done: make(chan struct{})}
	newTR.runnerHooks = append(newTR.runnerHooks, signal)
	go newTR.Run()
	defer newTR.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	select {
	case <-signal.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the restored task did not get past its prestart hooks")
	}
	require.Positive(t, lookups.Load(), "the volume hook ran again")
	require.Equal(t, structs.TaskStateRunning, newTR.TaskState().State)

	started := 0
	for _, ev := range newTR.TaskState().Events {
		if ev.Type == structs.TaskStarted {
			started++
		}
	}
	require.Equal(t, 1, started)
}

// startAndShutdownWithVolume runs a task with the volume present, waits for
// it to start and stops the task runner, leaving the driver task running.
func startAndShutdownWithVolume(t *testing.T, conf *Config, alloc *structs.Allocation) {
	t.Helper()
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return presentHostVolumes() }
	origTR, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go origTR.Run()
	t.Cleanup(func() { origTR.Kill(context.Background(), structs.NewTaskEvent("cleanup")) })
	testWaitForTaskToStart(t, origTR)
	origTR.Shutdown()
}

// TestTaskRunner_HostVolume_RestoredRunningTaskKeepsRunning asserts that a
// task restored with a running driver handle is not failed or restarted when
// its volume is still missing after the first restore has ended.
func TestTaskRunner_HostVolume_RestoredRunningTaskKeepsRunning(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeAlloc(hostVolumeRestartPolicies["no restarts"])
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	conf.StateDB = cstate.NewMemDB(conf.Logger)
	startAndShutdownWithVolume(t, conf, alloc)

	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return nil }
	conf.HostVolumesRestored = func() <-chan struct{} { return closedChan() }
	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	require.NoError(t, tr.Restore())
	signal := &poststartSignalHook{done: make(chan struct{})}
	tr.runnerHooks = append(tr.runnerHooks, signal)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	select {
	case <-signal.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the restored task did not get past its prestart hooks")
	}

	handle := tr.getDriverHandle()
	require.NotNil(t, handle)
	status, err := tr.driver.InspectTask(handle.ID())
	require.NoError(t, err)
	require.Equal(t, "running", string(status.State))

	state := tr.TaskState()
	require.Equal(t, structs.TaskStateRunning, state.State)
	requireNoVolumeFailure(t, state)
	// The event of the first run is restored with the state; a second one
	// would mean the task was started again.
	started := 0
	for _, ev := range state.Events {
		if ev.Type == structs.TaskStarted {
			started++
		}
	}
	require.Equal(t, 1, started)
}

// TestTaskRunner_HostVolume_RestoredTaskRestartFailsOnMissingVolume asserts
// that a restored running task that has to start again while its volume is
// still missing fails as a fresh start does.
func TestTaskRunner_HostVolume_RestoredTaskRestartFailsOnMissingVolume(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeAlloc(hostVolumeRestartPolicies["no restarts"])
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	conf.StateDB = cstate.NewMemDB(conf.Logger)
	startAndShutdownWithVolume(t, conf, alloc)

	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return nil }
	conf.HostVolumesRestored = func() <-chan struct{} { return closedChan() }
	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	require.NoError(t, tr.Restore())
	signal := &poststartSignalHook{done: make(chan struct{})}
	tr.runnerHooks = append(tr.runnerHooks, signal)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	select {
	case <-signal.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the restored task did not get past its prestart hooks")
	}

	require.NoError(t, tr.Restart(context.Background(), structs.NewTaskEvent(structs.TaskRestartSignal), false))

	// The task state is read from the state database, which the task runner
	// writes after each update.
	var state *structs.TaskState
	require.Eventually(t, func() bool {
		_, persisted, err := conf.StateDB.GetTaskRunnerState(alloc.ID, task.Name)
		if err != nil || persisted == nil {
			return false
		}
		state = persisted
		return state.State == structs.TaskStateDead
	}, 15*time.Second, 50*time.Millisecond)

	require.True(t, state.Failed)
	var failed bool
	for _, ev := range state.Events {
		if ev.Type == structs.TaskHookFailed {
			failed = failed || strings.Contains(ev.DisplayMessage, "host volume validation error")
		}
	}
	require.True(t, failed, "the restart fails with a volume validation error")
}

// poststartSignalHook closes done when the poststart hooks run.
type poststartSignalHook struct {
	once sync.Once
	done chan struct{}
}

func (*poststartSignalHook) Name() string { return "poststart_signal" }

func (h *poststartSignalHook) Poststart(context.Context, *interfaces.TaskPoststartRequest, *interfaces.TaskPoststartResponse) error {
	h.once.Do(func() { close(h.done) })
	return nil
}

// recordingPrestartHook counts its calls.
type recordingPrestartHook struct {
	calls atomic.Int32
}

func (*recordingPrestartHook) Name() string { return "recording" }

func (h *recordingPrestartHook) Prestart(context.Context, *interfaces.TaskPrestartRequest, *interfaces.TaskPrestartResponse) error {
	h.calls.Add(1)
	return nil
}

// TestTaskRunner_HostVolume_CancelledWaitSkipsLaterHooks asserts that the
// hooks after the volume hook do not run once the wait was cancelled: they
// would download artifacts and reserve devices for a task that is stopping.
func TestTaskRunner_HostVolume_CancelledWaitSkipsLaterHooks(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeAlloc(hostVolumeRestartPolicies["no restarts"])
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	conf.StateDB = cstate.NewMemDB(conf.Logger)
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return nil }
	restored, waiting := signalledRestore()
	conf.HostVolumesRestored = restored

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	rec := &recordingPrestartHook{}
	volumeIdx := -1
	for i, h := range tr.runnerHooks {
		if h.Name() == "volumes" {
			volumeIdx = i
		}
	}
	require.GreaterOrEqual(t, volumeIdx, 0)
	// Directly after the volume hook, ahead of every other later hook.
	tr.runnerHooks = slices.Insert(tr.runnerHooks, volumeIdx+1, interfaces.TaskHook(rec))
	go tr.Run()

	waitForHookToWait(t, waiting)
	tr.Shutdown()
	require.Zero(t, rec.calls.Load())
}

// waitForRunning waits for the task to run and fails at once, with the last
// event, when it dies first: a dead task cannot start later.
func waitForRunning(t *testing.T, tr *TaskRunner) {
	t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		state := tr.TaskState()
		if state.State == structs.TaskStateDead {
			msg := ""
			if n := len(state.Events); n > 0 {
				msg = state.Events[n-1].DisplayMessage
			}
			t.Fatalf("task died before it ran: %s", msg)
		}
		return state.State == structs.TaskStateRunning, fmt.Errorf("task is %s", state.State)
	}, func(err error) { require.NoError(t, err) })
}

// TestTaskRunner_HostVolume_GetterIsUsed asserts that the getter of the
// config reaches the volume hook: the volume is not in the config of the
// client, only the getter knows it.
func TestTaskRunner_HostVolume_GetterIsUsed(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeAlloc(hostVolumeRestartPolicies["no restarts"])
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	require.NotContains(t, conf.ClientConfig.Node.HostVolumes, "data-a")
	conf.StateDB = cstate.NewMemDB(conf.Logger)
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig { return presentHostVolumes() }

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	waitForRunning(t, tr)
}

// TestTaskRunner_HostVolume_RestoreGateIsUsed asserts that the restore
// channel of the config reaches the volume hook: the task does not run
// before the channel closes and does after.
func TestTaskRunner_HostVolume_RestoreGateIsUsed(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeAlloc(hostVolumeRestartPolicies["no restarts"])
	task := alloc.Job.TaskGroups[0].Tasks[0]
	conf, cleanup := testTaskRunnerConfig(t, alloc, task.Name)
	defer cleanup()
	conf.StateDB = cstate.NewMemDB(conf.Logger)

	gate := make(chan struct{})
	var once sync.Once
	waiting := make(chan struct{})
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig {
		select {
		case <-gate:
			return presentHostVolumes()
		default:
			return nil
		}
	}
	conf.HostVolumesRestored = func() <-chan struct{} {
		once.Do(func() { close(waiting) })
		return gate
	}

	tr, err := NewTaskRunner(conf)
	require.NoError(t, err)
	go tr.Run()
	defer tr.Kill(context.Background(), structs.NewTaskEvent("cleanup"))

	// Without the channel the hook fails the task instead of waiting.
	testutil.WaitForResult(func() (bool, error) {
		select {
		case <-waiting:
			return true, nil
		default:
		}
		if tr.TaskState().State == structs.TaskStateDead {
			t.Fatal("the task died instead of waiting for the restore")
		}
		return false, errors.New("the hook has not reached the wait")
	}, func(err error) { require.NoError(t, err) })
	require.Equal(t, structs.TaskStatePending, tr.TaskState().State)

	close(gate)
	waitForRunning(t, tr)
}

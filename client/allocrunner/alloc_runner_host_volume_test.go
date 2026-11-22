// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package allocrunner

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/stretchr/testify/require"
)

// hostVolumeRunAlloc returns a batch allocation whose group requests the host
// volume data-a without mounting it: the mock driver cannot mount, so the
// request alone tells a present volume from a missing one. A missing volume
// fails the task for good.
func hostVolumeRunAlloc() *structs.Allocation {
	alloc := mock.BatchAlloc()
	tg := alloc.Job.TaskGroups[0]
	tg.Count = 1
	tg.RestartPolicy = &structs.RestartPolicy{Attempts: 0, Mode: structs.RestartPolicyModeFail}
	tg.Tasks[0].RestartPolicy = &structs.RestartPolicy{Attempts: 0, Mode: structs.RestartPolicyModeFail}
	tg.Volumes = map[string]*structs.VolumeRequest{
		"data": {Name: "data", Type: structs.VolumeTypeHost, Source: "data-a"},
	}
	tg.Tasks[0].Driver = "mock_driver"
	tg.Tasks[0].Config = map[string]interface{}{"run_for": "1m"}
	tg.Tasks[0].VolumeMounts = nil
	return alloc
}

// hostVolumeTaskState returns the last state of the task that the alloc
// runner reported to its state updater. The alloc runner is not asked: its
// AllocState writes without the lock before the first task update.
func hostVolumeTaskState(conf *config.AllocRunnerConfig, taskName string) *structs.TaskState {
	last := conf.StateUpdater.(*MockStateUpdater).Last()
	if last == nil {
		return nil
	}
	return last.TaskStates[taskName]
}

// waitForHostVolumeTask waits for the task to run and fails at once when it
// dies first.
func waitForHostVolumeTask(t *testing.T, conf *config.AllocRunnerConfig, taskName string) {
	t.Helper()
	testutil.WaitForResult(func() (bool, error) {
		ts := hostVolumeTaskState(conf, taskName)
		if ts == nil {
			return false, errors.New("no task state yet")
		}
		if ts.State == structs.TaskStateDead {
			t.Fatalf("task died before it ran: failed=%v", ts.Failed)
		}
		return ts.State == structs.TaskStateRunning, fmt.Errorf("task is %s", ts.State)
	}, func(err error) { require.NoError(t, err) })
}

// TestAllocRunner_HostVolumes_GetterReachesTasks asserts that the getter of
// the alloc runner config is kept and handed to the task runner: the volume
// is not in the client config, only the getter knows it.
func TestAllocRunner_HostVolumes_GetterReachesTasks(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeRunAlloc()
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()
	require.NotContains(t, conf.ClientConfig.Node.HostVolumes, "data-a")
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig {
		return map[string]*structs.ClientHostVolumeConfig{
			"data-a": {Name: "data-a", Path: "/synthetic/data-a"},
		}
	}

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	waitForHostVolumeTask(t, conf, alloc.Job.TaskGroups[0].Tasks[0].Name)
}

// TestAllocRunner_HostVolumes_RestoreGateReachesTasks asserts that the
// restore channel of the alloc runner config is kept and handed to the task
// runner: the task does not run before the channel closes and does after.
func TestAllocRunner_HostVolumes_RestoreGateReachesTasks(t *testing.T) {
	ci.Parallel(t)

	alloc := hostVolumeRunAlloc()
	taskName := alloc.Job.TaskGroups[0].Tasks[0].Name
	conf, cleanup := testAllocRunnerConfig(t, alloc)
	defer cleanup()

	gate := make(chan struct{})
	var once sync.Once
	waiting := make(chan struct{})
	conf.HostVolumes = func(string) map[string]*structs.ClientHostVolumeConfig {
		select {
		case <-gate:
			return map[string]*structs.ClientHostVolumeConfig{
				"data-a": {Name: "data-a", Path: "/synthetic/data-a"},
			}
		default:
			return nil
		}
	}
	conf.HostVolumesRestored = func() <-chan struct{} {
		once.Do(func() { close(waiting) })
		return gate
	}

	ar, err := NewAllocRunner(conf)
	require.NoError(t, err)
	go ar.Run()
	defer destroy(ar)

	// Without the channel the hook fails the task instead of waiting.
	testutil.WaitForResult(func() (bool, error) {
		select {
		case <-waiting:
			return true, nil
		default:
		}
		if ts := hostVolumeTaskState(conf, taskName); ts != nil && ts.State == structs.TaskStateDead {
			t.Fatal("the task died instead of waiting for the restore")
		}
		return false, errors.New("the hook has not reached the wait")
	}, func(err error) { require.NoError(t, err) })
	if ts := hostVolumeTaskState(conf, taskName); ts != nil {
		require.NotEqual(t, structs.TaskStateRunning, ts.State)
	}

	close(gate)
	waitForHostVolumeTask(t, conf, taskName)
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package rawexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	clienttestutil "github.com/open-wander/wander/client/testutil"
	"github.com/open-wander/wander/helper/testtask"
	"github.com/open-wander/wander/helper/uuid"
	basePlug "github.com/open-wander/wander/plugins/base"
	"github.com/open-wander/wander/plugins/drivers"
	dtestutil "github.com/open-wander/wander/plugins/drivers/testutils"
	"github.com/open-wander/wander/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestRawExecDriver_User(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireLinux(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "sleep",
		User: "alice",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "45s"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.Error(err)
	msg := "unknown user alice"
	require.Contains(err.Error(), msg)
}

func TestRawExecDriver_Signal(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireLinux(t)

	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "signal",
		Env:     defaultEnv(),
	}

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/bash",
		Args:    []string{"test.sh"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	testFile := filepath.Join(task.TaskDir().Dir, "test.sh")
	testData := []byte(`
at_term() {
    echo 'Terminated.'
    exit 3
}
trap at_term USR1
while true; do
    sleep 1
done
	`)
	require.NoError(os.WriteFile(testFile, testData, 0777))

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	go func() {
		time.Sleep(100 * time.Millisecond)
		require.NoError(harness.SignalTask(task.ID, "SIGUSR1"))
	}()

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)
	select {
	case res := <-waitCh:
		require.False(res.Successful())
		require.Equal(3, res.ExitCode)
	case <-time.After(time.Duration(testutil.TestMultiplier()*6) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check the log file to see it exited because of the signal
	outputFile := filepath.Join(task.TaskDir().LogDir, "signal.stdout.0")
	exp := "Terminated."
	testutil.WaitForResult(func() (bool, error) {
		act, err := os.ReadFile(outputFile)
		if err != nil {
			return false, fmt.Errorf("Couldn't read expected output: %v", err)
		}

		if strings.TrimSpace(string(act)) != exp {
			t.Logf("Read from %v", outputFile)
			return false, fmt.Errorf("Command outputted %v; want %v", act, exp)
		}
		return true, nil
	}, func(err error) { require.NoError(err) })
}

func TestRawExecDriver_StartWaitStop(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	// Disable cgroups so test works without root
	config := &Config{NoCgroups: true, Enabled: true}
	var data []byte
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig := &basePlug.Config{PluginConfig: data}
	require.NoError(harness.SetConfig(bconfig))

	task := &drivers.TaskConfig{
		ID:   uuid.Generate(),
		Name: "test",
	}

	taskConfig := map[string]interface{}{}
	taskConfig["command"] = testtask.Path()
	taskConfig["args"] = []string{"sleep", "100s"}

	require.NoError(task.EncodeConcreteDriverConfig(&taskConfig))

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	require.NoError(harness.WaitUntilStarted(task.ID, 1*time.Second))

	go func() {
		harness.StopTask(task.ID, 2*time.Second, "SIGINT")
	}()

	select {
	case result := <-ch:
		require.Equal(int(unix.SIGINT), result.Signal)
	case <-time.After(10 * time.Second):
		require.Fail("timeout waiting for task to shutdown")
	}

	// Ensure that the task is marked as dead, but account
	// for WaitTask() closing channel before internal state is updated
	testutil.WaitForResult(func() (bool, error) {
		status, err := harness.InspectTask(task.ID)
		if err != nil {
			return false, fmt.Errorf("inspecting task failed: %v", err)
		}
		if status.State != drivers.TaskStateExited {
			return false, fmt.Errorf("task hasn't exited yet; status: %v", status.State)
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	require.NoError(harness.DestroyTask(task.ID, true))
}

// TestRawExecDriver_DestroyKillsAll asserts that when TaskDestroy is called all
// task processes are cleaned up.
func TestRawExecDriver_DestroyKillsAll(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireLinux(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "test",
		Env:     defaultEnv(),
	}

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	taskConfig := map[string]interface{}{}
	taskConfig["command"] = "/bin/sh"
	taskConfig["args"] = []string{"-c", fmt.Sprintf(`sleep 3600 & echo "SLEEP_PID=$!"`)}

	require.NoError(t, task.EncodeConcreteDriverConfig(&taskConfig))

	handle, _, err := harness.StartTask(task)
	require.NoError(t, err)
	defer harness.DestroyTask(task.ID, true)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(t, err)

	select {
	case result := <-ch:
		require.True(t, result.Successful(), "command failed: %#v", result)
	case <-time.After(10 * time.Second):
		require.Fail(t, "timeout waiting for task to shutdown")
	}

	sleepPid := 0

	// Ensure that the task is marked as dead, but account
	// for WaitTask() closing channel before internal state is updated
	testutil.WaitForResult(func() (bool, error) {
		stdout, err := os.ReadFile(filepath.Join(task.TaskDir().LogDir, "test.stdout.0"))
		if err != nil {
			return false, fmt.Errorf("failed to output pid file: %v", err)
		}

		pidMatch := regexp.MustCompile(`SLEEP_PID=(\d+)`).FindStringSubmatch(string(stdout))
		if len(pidMatch) != 2 {
			return false, fmt.Errorf("failed to find pid in %s", string(stdout))
		}

		pid, err := strconv.Atoi(pidMatch[1])
		if err != nil {
			return false, fmt.Errorf("pid parts aren't int: %s", pidMatch[1])
		}

		sleepPid = pid
		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})

	// isProcessRunning returns an error if process is not running
	isProcessRunning := func(pid int) error {
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("failed to find process: %s", err)
		}

		err = process.Signal(syscall.Signal(0))
		if err != nil {
			return fmt.Errorf("failed to signal process: %s", err)
		}

		return nil
	}

	require.NoError(t, isProcessRunning(sleepPid))

	require.NoError(t, harness.DestroyTask(task.ID, true))

	testutil.WaitForResult(func() (bool, error) {
		err := isProcessRunning(sleepPid)
		if err == nil {
			return false, fmt.Errorf("child process is still running")
		}

		if !strings.Contains(err.Error(), "failed to signal process") {
			return false, fmt.Errorf("unexpected error: %v", err)
		}

		return true, nil
	}, func(err error) {
		require.NoError(t, err)
	})
}

func TestRawExec_ExecTaskStreaming(t *testing.T) {
	ci.Parallel(t)
	if runtime.GOOS == "darwin" {
		t.Skip("skip running exec tasks on darwin as darwin has restrictions on starting tty shells")
	}
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "sleep",
		Env:     defaultEnv(),
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "9000s"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.NoError(err)
	defer d.DestroyTask(task.ID, true)

	dtestutil.ExecTaskStreamingConformanceTests(t, harness, task.ID)

}

func TestRawExec_ExecTaskStreaming_User(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireLinux(t)

	d := newEnabledRawExecDriver(t)

	// because we cannot set AllocID, see below
	d.config.NoCgroups = true

	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	task := &drivers.TaskConfig{
		// todo(shoenig) - Setting AllocID causes test to fail - with or without
		//  cgroups, and with or without chroot. It has to do with MkAllocDir
		//  creating the directory structure, but the actual root cause is still
		//  TBD. The symptom is that any command you try to execute will result
		//  in "permission denied" coming from os/exec. This test is imperfect,
		//  the actual feature of running commands as another user works fine.
		// AllocID: uuid.Generate()
		ID:   uuid.Generate(),
		Name: "sleep",
		User: "nobody",
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	err := os.Chmod(task.AllocDir, 0777)
	require.NoError(t, err)

	tc := &TaskConfig{
		Command: "/bin/sleep",
		Args:    []string{"9000"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err = harness.StartTask(task)
	require.NoError(t, err)
	defer d.DestroyTask(task.ID, true)

	code, stdout, stderr := dtestutil.ExecTask(t, harness, task.ID, "whoami", false, "")
	require.Zero(t, code)
	require.Empty(t, stderr)
	require.Contains(t, stdout, "nobody")
}

func TestRawExecDriver_NoCgroup(t *testing.T) {
	ci.Parallel(t)
	clienttestutil.RequireLinux(t)

	expectedBytes, err := os.ReadFile("/proc/self/cgroup")
	require.NoError(t, err)
	expected := strings.TrimSpace(string(expectedBytes))

	d := newEnabledRawExecDriver(t)
	d.config.NoCgroups = true
	harness := dtestutil.NewDriverHarness(t, d)

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "nocgroup",
	}

	cleanup := harness.MkAllocDir(task, true)
	defer cleanup()

	tc := &TaskConfig{
		Command: "/bin/cat",
		Args:    []string{"/proc/self/cgroup"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err = harness.StartTask(task)
	require.NoError(t, err)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(t, err)
	select {
	case res := <-waitCh:
		require.True(t, res.Successful())
		require.Zero(t, res.ExitCode)
	case <-time.After(time.Duration(testutil.TestMultiplier()*6) * time.Second):
		require.Fail(t, "WaitTask timeout")
	}

	// Check the log file to see it exited because of the signal
	outputFile := filepath.Join(task.TaskDir().LogDir, "nocgroup.stdout.0")
	testutil.WaitForResult(func() (bool, error) {
		act, err := os.ReadFile(outputFile)
		if err != nil {
			return false, fmt.Errorf("Couldn't read expected output: %v", err)
		}

		if strings.TrimSpace(string(act)) != expected {
			t.Logf("Read from %v", outputFile)
			return false, fmt.Errorf("Command outputted\n%v; want\n%v", string(act), expected)
		}
		return true, nil
	}, func(err error) { require.NoError(t, err) })
}

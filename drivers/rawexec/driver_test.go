// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rawexec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/lib/cgutil"
	ctestutil "github.com/open-wander/wander/client/testutil"
	"github.com/open-wander/wander/helper/pluginutils/hclutils"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/helper/testtask"
	"github.com/open-wander/wander/helper/uuid"
	basePlug "github.com/open-wander/wander/plugins/base"
	"github.com/open-wander/wander/plugins/drivers"
	dtestutil "github.com/open-wander/wander/plugins/drivers/testutils"
	pstructs "github.com/open-wander/wander/plugins/shared/structs"
	"github.com/open-wander/wander/testutil"
	"github.com/stretchr/testify/require"
)

// defaultEnv creates the default environment for raw exec tasks
func defaultEnv() map[string]string {
	m := make(map[string]string)
	if cgutil.UseV2 {
		// normally the taskenv.Builder will set this automatically from the
		// Node object which gets created via Client configuration, but none of
		// that exists in the test harness so just set it here.
		m["NOMAD_PARENT_CGROUP"] = "nomad.slice"
	}
	return m
}

func TestMain(m *testing.M) {
	if !testtask.Run() {
		os.Exit(m.Run())
	}
}

func newEnabledRawExecDriver(t *testing.T) *Driver {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	logger := testlog.HCLogger(t)
	d := NewRawExecDriver(ctx, logger).(*Driver)
	d.config.Enabled = true

	return d
}

func TestRawExecDriver_SetConfig(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := testlog.HCLogger(t)

	d := NewRawExecDriver(ctx, logger)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	var (
		bconfig = new(basePlug.Config)
		config  = new(Config)
		data    = make([]byte, 0)
	)

	// Default is raw_exec is disabled.
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig.PluginConfig = data
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)

	// Enable raw_exec, but disable cgroups.
	config.Enabled = true
	config.NoCgroups = true
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig.PluginConfig = data
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)

	// Enable raw_exec, enable cgroups.
	config.NoCgroups = false
	data = []byte{}
	require.NoError(basePlug.MsgPackEncode(&data, config))
	bconfig.PluginConfig = data
	require.NoError(harness.SetConfig(bconfig))
	require.Exactly(config, d.(*Driver).config)
}

func TestRawExecDriver_Fingerprint(t *testing.T) {
	ci.Parallel(t)

	fingerprintTest := func(config *Config, expected *drivers.Fingerprint) func(t *testing.T) {
		return func(t *testing.T) {
			require := require.New(t)
			d := newEnabledRawExecDriver(t)
			harness := dtestutil.NewDriverHarness(t, d)
			defer harness.Kill()

			var data []byte
			require.NoError(basePlug.MsgPackEncode(&data, config))
			bconfig := &basePlug.Config{
				PluginConfig: data,
			}
			require.NoError(harness.SetConfig(bconfig))

			fingerCh, err := harness.Fingerprint(context.Background())
			require.NoError(err)
			select {
			case result := <-fingerCh:
				require.Equal(expected, result)
			case <-time.After(time.Duration(testutil.TestMultiplier()) * time.Second):
				require.Fail("timeout receiving fingerprint")
			}
		}
	}

	cases := []struct {
		Name     string
		Conf     Config
		Expected drivers.Fingerprint
	}{
		{
			Name: "Disabled",
			Conf: Config{
				Enabled: false,
			},
			Expected: drivers.Fingerprint{
				Attributes:        nil,
				Health:            drivers.HealthStateUndetected,
				HealthDescription: "disabled",
			},
		},
		{
			Name: "Enabled",
			Conf: Config{
				Enabled: true,
			},
			Expected: drivers.Fingerprint{
				Attributes:        map[string]*pstructs.Attribute{"driver.raw_exec": pstructs.NewBoolAttribute(true)},
				Health:            drivers.HealthStateHealthy,
				HealthDescription: drivers.DriverHealthy,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, fingerprintTest(&tc.Conf, &tc.Expected))
	}
}

func TestRawExecDriver_StartWait(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()
	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "test",
		Env:     defaultEnv(),
	}

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "10ms"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), handle.Config.ID)
	require.NoError(err)

	var result *drivers.ExitResult
	select {
	case result = <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	require.Zero(result.ExitCode)
	require.Zero(result.Signal)
	require.False(result.OOMKilled)
	require.NoError(result.Err)
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_StartWaitRecoverWaitStop(t *testing.T) {
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
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "sleep",
		Env:     defaultEnv(),
	}
	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "100s"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))

	testtask.SetTaskConfigEnv(task)
	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	handle, _, err := harness.StartTask(task)
	require.NoError(err)

	ch, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	var waitDone bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result := <-ch
		require.Error(result.Err)
		waitDone = true
	}()

	originalStatus, err := d.InspectTask(task.ID)
	require.NoError(err)

	d.tasks.Delete(task.ID)

	wg.Wait()
	require.True(waitDone)
	_, err = d.InspectTask(task.ID)
	require.Equal(drivers.ErrTaskNotFound, err)

	err = d.RecoverTask(handle)
	require.NoError(err)

	status, err := d.InspectTask(task.ID)
	require.NoError(err)
	require.Exactly(originalStatus, status)

	ch, err = harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	wg.Add(1)
	waitDone = false
	go func() {
		defer wg.Done()
		result := <-ch
		require.NoError(result.Err)
		require.NotZero(result.ExitCode)
		require.Equal(9, result.Signal)
		waitDone = true
	}()

	time.Sleep(300 * time.Millisecond)
	require.NoError(d.StopTask(task.ID, 0, "SIGKILL"))
	wg.Wait()
	require.NoError(d.DestroyTask(task.ID, false))
	require.True(waitDone)
}

func TestRawExecDriver_Start_Wait_AllocDir(t *testing.T) {
	ci.Parallel(t)
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

	exp := []byte("win")
	file := "output.txt"
	outPath := fmt.Sprintf(`%s/%s`, task.TaskDir().SharedAllocDir, file)

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "1s", "write", string(exp), outPath},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)

	select {
	case res := <-waitCh:
		require.NoError(res.Err)
		require.True(res.Successful())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	// Check that data was written to the shared alloc directory.
	outputFile := filepath.Join(task.TaskDir().SharedAllocDir, file)
	act, err := os.ReadFile(outputFile)
	require.NoError(err)
	require.Exactly(exp, act)
	require.NoError(harness.DestroyTask(task.ID, true))
}

// This test creates a process tree such that without cgroups tracking the
// processes cleanup of the children would not be possible. Thus the test
// asserts that the processes get killed properly when using cgroups.
func TestRawExecDriver_Start_Kill_Wait_Cgroup(t *testing.T) {
	ci.Parallel(t)
	ctestutil.ExecCompatible(t)

	require := require.New(t)
	pidFile := "pid"

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "sleep",
		User:    "root",
		Env:     defaultEnv(),
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"fork/exec", pidFile, "pgrp", "0", "sleep", "20s"},
	}
	require.NoError(task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)

	_, _, err := harness.StartTask(task)
	require.NoError(err)

	// Find the process
	var pidData []byte
	testutil.WaitForResult(func() (bool, error) {
		var err error
		pidData, err = os.ReadFile(filepath.Join(task.TaskDir().Dir, pidFile))
		if err != nil {
			return false, err
		}

		if len(pidData) == 0 {
			return false, fmt.Errorf("pidFile empty")
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	pid, err := strconv.Atoi(string(pidData))
	require.NoError(err, "failed to read pidData: %s", string(pidData))

	// Check the pid is up
	process, err := os.FindProcess(pid)
	require.NoError(err)
	require.NoError(process.Signal(syscall.Signal(0)))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		err := harness.StopTask(task.ID, 0, "")

		// Can't rely on the ordering between wait and kill on CI/travis...
		if !testutil.IsCI() {
			require.NoError(err)
		}
	}()

	// Task should terminate quickly
	waitCh, err := harness.WaitTask(context.Background(), task.ID)
	require.NoError(err)
	select {
	case res := <-waitCh:
		require.False(res.Successful())
	case <-time.After(time.Duration(testutil.TestMultiplier()*5) * time.Second):
		require.Fail("WaitTask timeout")
	}

	testutil.WaitForResult(func() (bool, error) {
		if err := process.Signal(syscall.Signal(0)); err == nil {
			return false, fmt.Errorf("process should not exist: %v", pid)
		}

		return true, nil
	}, func(err error) {
		require.NoError(err)
	})

	wg.Wait()
	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_ParentCgroup(t *testing.T) {
	ci.Parallel(t)
	ctestutil.ExecCompatible(t)
	ctestutil.CgroupsCompatibleV2(t)

	d := newEnabledRawExecDriver(t)
	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()

	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "sleep",
		Env: map[string]string{
			"NOMAD_PARENT_CGROUP": "custom.slice",
		},
	}

	cleanup := harness.MkAllocDir(task, false)
	defer cleanup()

	// run sleep task
	tc := &TaskConfig{
		Command: testtask.Path(),
		Args:    []string{"sleep", "9000s"},
	}
	require.NoError(t, task.EncodeConcreteDriverConfig(&tc))
	testtask.SetTaskConfigEnv(task)
	_, _, err := harness.StartTask(task)
	require.NoError(t, err)

	// inspect environment variable
	res, execErr := harness.ExecTask(task.ID, []string{"/usr/bin/env"}, 1*time.Second)
	require.NoError(t, execErr)
	require.True(t, res.ExitResult.Successful())
	require.Contains(t, string(res.Stdout), "custom.slice")

	// inspect /proc/self/cgroup
	res2, execErr2 := harness.ExecTask(task.ID, []string{"cat", "/proc/self/cgroup"}, 1*time.Second)
	require.NoError(t, execErr2)
	require.True(t, res2.ExitResult.Successful())
	require.Contains(t, string(res2.Stdout), "custom.slice")

	// kill the sleep task
	require.NoError(t, harness.DestroyTask(task.ID, true))
}

func TestRawExecDriver_Exec(t *testing.T) {
	ci.Parallel(t)
	ctestutil.ExecCompatible(t)

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

	if runtime.GOOS == "windows" {
		// Exec a command that should work
		res, err := harness.ExecTask(task.ID, []string{"cmd.exe", "/c", "echo", "hello"}, 1*time.Second)
		require.NoError(err)
		require.True(res.ExitResult.Successful())
		require.Equal(string(res.Stdout), "hello\r\n")

		// Exec a command that should fail
		res, err = harness.ExecTask(task.ID, []string{"cmd.exe", "/c", "stat", "notarealfile123abc"}, 1*time.Second)
		require.NoError(err)
		require.False(res.ExitResult.Successful())
		require.Contains(string(res.Stdout), "not recognized")
	} else {
		// Exec a command that should work
		res, err := harness.ExecTask(task.ID, []string{"/usr/bin/stat", "/tmp"}, 1*time.Second)
		require.NoError(err)
		require.True(res.ExitResult.Successful())
		require.True(len(res.Stdout) > 100)

		// Exec a command that should fail
		res, err = harness.ExecTask(task.ID, []string{"/usr/bin/stat", "notarealfile123abc"}, 1*time.Second)
		require.NoError(err)
		require.False(res.ExitResult.Successful())
		require.Contains(string(res.Stdout), "No such file or directory")
	}

	require.NoError(harness.DestroyTask(task.ID, true))
}

func TestConfig_ParseAllHCL(t *testing.T) {
	ci.Parallel(t)

	cfgStr := `
config {
  command = "/bin/bash"
  args = ["-c", "echo hello"]
}`

	expected := &TaskConfig{
		Command: "/bin/bash",
		Args:    []string{"-c", "echo hello"},
	}

	var tc *TaskConfig
	hclutils.NewConfigParser(taskConfigSpec).ParseHCL(t, cfgStr, &tc)

	require.EqualValues(t, expected, tc)
}

func TestRawExecDriver_Disabled(t *testing.T) {
	ci.Parallel(t)
	require := require.New(t)

	d := newEnabledRawExecDriver(t)
	d.config.Enabled = false

	harness := dtestutil.NewDriverHarness(t, d)
	defer harness.Kill()
	task := &drivers.TaskConfig{
		AllocID: uuid.Generate(),
		ID:      uuid.Generate(),
		Name:    "test",
		Env:     defaultEnv(),
	}

	handle, _, err := harness.StartTask(task)
	require.Error(err)
	require.Contains(err.Error(), errDisabledDriver.Error())
	require.Nil(handle)
}

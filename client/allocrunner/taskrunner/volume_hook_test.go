// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package taskrunner

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/allocrunner/interfaces"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/pluginmanager/csimanager"
	cstructs "github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/client/taskenv"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/plugins/drivers"
	dtu "github.com/open-wander/wander/plugins/drivers/testutils"
	"github.com/stretchr/testify/require"
)

func TestVolumeHook_PartitionMountsByVolume_Works(t *testing.T) {
	ci.Parallel(t)

	mounts := []*structs.VolumeMount{
		{
			Volume:      "foo",
			Destination: "/tmp",
			ReadOnly:    false,
		},
		{
			Volume:      "foo",
			Destination: "/bar",
			ReadOnly:    false,
		},
		{
			Volume:      "baz",
			Destination: "/baz",
			ReadOnly:    false,
		},
	}

	expected := map[string][]*structs.VolumeMount{
		"foo": {
			{
				Volume:      "foo",
				Destination: "/tmp",
				ReadOnly:    false,
			},
			{
				Volume:      "foo",
				Destination: "/bar",
				ReadOnly:    false,
			},
		},
		"baz": {
			{
				Volume:      "baz",
				Destination: "/baz",
				ReadOnly:    false,
			},
		},
	}

	// Test with a real collection

	partitioned := partitionMountsByVolume(mounts)
	require.Equal(t, expected, partitioned)

	// Test with nil/emptylist

	partitioned = partitionMountsByVolume(nil)
	require.Equal(t, map[string][]*structs.VolumeMount{}, partitioned)
}

func TestVolumeHook_prepareCSIVolumes(t *testing.T) {
	ci.Parallel(t)

	req := &interfaces.TaskPrestartRequest{
		Task: &structs.Task{
			Name:   "test",
			Driver: "mock",
			VolumeMounts: []*structs.VolumeMount{
				{
					Volume:      "foo",
					Destination: "/bar",
				},
			},
		},
	}

	volumes := map[string]*structs.VolumeRequest{
		"foo": {
			Type:   "csi",
			Source: "my-test-volume",
		},
	}

	cases := []struct {
		Name          string
		Driver        drivers.DriverPlugin
		Expected      []*drivers.MountConfig
		ExpectedError string
	}{
		{
			Name: "supported driver",
			Driver: &dtu.MockDriver{
				CapabilitiesF: func() (*drivers.Capabilities, error) {
					return &drivers.Capabilities{
						MountConfigs: drivers.MountConfigSupportAll,
					}, nil
				},
			},
			Expected: []*drivers.MountConfig{
				{
					HostPath: "/mnt/my-test-volume",
					TaskPath: "/bar",
				},
			},
		},
		{
			Name: "unsupported driver",
			Driver: &dtu.MockDriver{
				CapabilitiesF: func() (*drivers.Capabilities, error) {
					return &drivers.Capabilities{
						MountConfigs: drivers.MountConfigSupportNone,
					}, nil
				},
			},
			ExpectedError: "task driver \"mock\" for \"test\" does not support CSI",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {

			tr := &TaskRunner{
				task:               req.Task,
				driver:             tc.Driver,
				allocHookResources: cstructs.NewAllocHookResources(),
			}
			tr.allocHookResources.SetCSIMounts(map[string]*csimanager.MountInfo{
				"foo": {
					Source: "/mnt/my-test-volume",
				},
			})

			hook := &volumeHook{
				logger: testlog.HCLogger(t),
				alloc:  structs.MockAlloc(),
				runner: tr,
			}
			mounts, err := hook.prepareCSIVolumes(req, volumes)

			if tc.ExpectedError != "" {
				require.EqualError(t, err, tc.ExpectedError)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.Expected, mounts)
		})
	}
}

func TestVolumeHook_Interpolation(t *testing.T) {
	ci.Parallel(t)

	alloc := mock.Alloc()
	task := alloc.Job.TaskGroups[0].Tasks[0]
	taskEnv := taskenv.NewBuilder(mock.Node(), alloc, task, "global").SetHookEnv("volume",
		map[string]string{
			"PROPAGATION_MODE": "private",
			"VOLUME_ID":        "my-other-volume",
		},
	).Build()

	mounts := []*structs.VolumeMount{
		{
			Volume:          "foo",
			Destination:     "/tmp",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/bar-${NOMAD_JOB_NAME}",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "${VOLUME_ID}",
			Destination:     "/baz",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/quux",
			ReadOnly:        false,
			PropagationMode: "${PROPAGATION_MODE}",
		},
	}

	expected := []*structs.VolumeMount{
		{
			Volume:          "foo",
			Destination:     "/tmp",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/bar-my-job",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "my-other-volume",
			Destination:     "/baz",
			ReadOnly:        false,
			PropagationMode: "bidirectional",
		},
		{
			Volume:          "foo",
			Destination:     "/quux",
			ReadOnly:        false,
			PropagationMode: "private",
		},
	}

	interpolateVolumeMounts(mounts, taskEnv)
	require.Equal(t, expected, mounts)
}

// hostVolumeHookEnv builds a hook over a task runner whose static host
// volumes come from the captured client config and whose live set comes
// from the injected getters.
func hostVolumeHookEnv(t *testing.T, captured map[string]*structs.ClientHostVolumeConfig, get func(string) map[string]*structs.ClientHostVolumeConfig, restored func() <-chan struct{}) (*volumeHook, *interfaces.TaskPrestartRequest) {
	t.Helper()
	task := &structs.Task{
		Name:   "test",
		Driver: "mock",
		VolumeMounts: []*structs.VolumeMount{
			{Volume: "data", Destination: "/data", PropagationMode: "private"},
		},
	}
	tr := &TaskRunner{
		task: task,
		driver: &dtu.MockDriver{
			CapabilitiesF: func() (*drivers.Capabilities, error) {
				return &drivers.Capabilities{MountConfigs: drivers.MountConfigSupportAll}, nil
			},
		},
		clientConfig:        &config.Config{Node: &structs.Node{HostVolumes: captured}},
		hostVolumes:         get,
		hostVolumesRestored: restored,
	}
	alloc := mock.Alloc()
	alloc.Namespace = "ns-a"
	alloc.Name = "job-a.group-a[0]"
	hook := &volumeHook{logger: testlog.HCLogger(t), alloc: alloc, runner: tr}
	req := &interfaces.TaskPrestartRequest{Alloc: alloc, Task: task}
	return hook, req
}

func hostVolumeRequests(readOnly bool) map[string]*structs.VolumeRequest {
	return map[string]*structs.VolumeRequest{
		"data": {Type: structs.VolumeTypeHost, Source: "data-a", ReadOnly: readOnly},
	}
}

func closedChan() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func TestVolumeHook_prepareHostVolumes_Live(t *testing.T) {
	ci.Parallel(t)

	dyn := func(path string) map[string]*structs.ClientHostVolumeConfig {
		return map[string]*structs.ClientHostVolumeConfig{
			"data-a": {Name: "data-a", Path: path, ID: "11111111-2222-3333-4444-555555555555"},
		}
	}
	static := map[string]*structs.ClientHostVolumeConfig{
		"data-a": {Name: "data-a", Path: "/synthetic/static"},
	}

	t.Run("a nil getter keeps the captured set", func(t *testing.T) {
		hook, req := hostVolumeHookEnv(t, static, nil, nil)
		mounts, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
		require.NoError(t, err)
		require.Equal(t, []*drivers.MountConfig{{HostPath: "/synthetic/static", TaskPath: "/data", PropagationMode: "private"}}, mounts)
	})

	t.Run("a volume published after construction is mounted", func(t *testing.T) {
		hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig { return dyn("/synthetic/dyn") }, nil)
		mounts, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
		require.NoError(t, err)
		require.Equal(t, []*drivers.MountConfig{{HostPath: "/synthetic/dyn", TaskPath: "/data", PropagationMode: "private"}}, mounts)
	})

	t.Run("the getter receives the namespace of the allocation", func(t *testing.T) {
		var got string
		hook, req := hostVolumeHookEnv(t, nil, func(ns string) map[string]*structs.ClientHostVolumeConfig {
			got = ns
			return dyn("/synthetic/dyn")
		}, nil)
		_, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
		require.NoError(t, err)
		require.Equal(t, "ns-a", got)
	})

	t.Run("a complete set does not wait", func(t *testing.T) {
		// A restore that never ends must not delay a task whose volumes exist.
		never := func() <-chan struct{} { return make(chan struct{}) }
		hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig { return dyn("/synthetic/dyn") }, never)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := hook.prepareHostVolumes(ctx, req, hostVolumeRequests(false))
		require.NoError(t, err)
	})

	t.Run("a missing volume waits for the first restore", func(t *testing.T) {
		restored := make(chan struct{})
		published := make(chan struct{})
		get := func(string) map[string]*structs.ClientHostVolumeConfig {
			select {
			case <-published:
				return dyn("/synthetic/dyn")
			default:
				return nil
			}
		}
		hook, req := hostVolumeHookEnv(t, nil, get, func() <-chan struct{} { return restored })

		type res struct {
			mounts []*drivers.MountConfig
			err    error
		}
		done := make(chan res, 1)
		go func() {
			m, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
			done <- res{m, err}
		}()
		select {
		case <-done:
			t.Fatal("returned before the restore ended")
		case <-time.After(100 * time.Millisecond):
		}
		close(published)
		close(restored)
		r := <-done
		require.NoError(t, r.err)
		require.Len(t, r.mounts, 1)
		require.Equal(t, "/synthetic/dyn", r.mounts[0].HostPath)
	})

	t.Run("cancelling the context aborts the wait", func(t *testing.T) {
		hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig { return nil },
			func() <-chan struct{} { return make(chan struct{}) })
		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			_, err := hook.prepareHostVolumes(ctx, req, hostVolumeRequests(false))
			errCh <- err
		}()
		cancel()
		select {
		case err := <-errCh:
			require.Error(t, err)
			require.True(t, errors.Is(err, context.Canceled))
		case <-time.After(5 * time.Second):
			t.Fatal("the wait was not aborted")
		}
	})

	t.Run("still missing after the restore fails naming only the source", func(t *testing.T) {
		hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig {
			return map[string]*structs.ClientHostVolumeConfig{"other": {Name: "other", Path: "/synthetic/other"}}
		}, closedChan)
		_, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing data-a")
		require.NotContains(t, err.Error(), "/synthetic")
	})

	t.Run("a missing static volume still fails the same way", func(t *testing.T) {
		hook, req := hostVolumeHookEnv(t, nil, nil, nil)
		_, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
		require.EqualError(t, err, "host volume validation error: 1 error occurred:\n\t* missing data-a\n\n")
	})
}

func TestVolumeHook_prepareHostVolumes_ReadOnly(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name      string
		reqRO     bool
		mountRO   bool
		volumeRO  bool
		expectRO  bool
		propagate string
	}{
		{name: "read-write", propagate: "private"},
		{name: "request read-only", reqRO: true, expectRO: true, propagate: "private"},
		{name: "mount read-only", mountRO: true, expectRO: true, propagate: "rshared"},
		{name: "volume read-only", volumeRO: true, expectRO: true, propagate: "private"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			live := map[string]*structs.ClientHostVolumeConfig{
				"data-a": {Name: "data-a", Path: "/synthetic/dyn", ID: "11111111-2222-3333-4444-555555555555", ReadOnly: tc.volumeRO},
			}
			hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig { return live }, nil)
			req.Task.VolumeMounts[0].ReadOnly = tc.mountRO
			req.Task.VolumeMounts[0].PropagationMode = tc.propagate
			mounts, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(tc.reqRO))
			require.NoError(t, err)
			require.Equal(t, []*drivers.MountConfig{{HostPath: "/synthetic/dyn", TaskPath: "/data", Readonly: tc.expectRO, PropagationMode: tc.propagate}}, mounts)
		})
	}
}

func TestVolumeHook_prepareHostVolumes_UnsupportedDriver(t *testing.T) {
	ci.Parallel(t)

	hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig {
		return map[string]*structs.ClientHostVolumeConfig{"data-a": {Name: "data-a", Path: "/synthetic/dyn", ID: "11111111-2222-3333-4444-555555555555"}}
	}, nil)
	hook.runner.driver = &dtu.MockDriver{
		CapabilitiesF: func() (*drivers.Capabilities, error) {
			return &drivers.Capabilities{MountConfigs: drivers.MountConfigSupportNone}, nil
		},
	}
	_, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
	require.EqualError(t, err, "task driver \"mock\" for \"test\" does not support host volumes")
}

func TestVolumeHook_missingHostVolumes(t *testing.T) {
	ci.Parallel(t)

	requested := map[string]*structs.VolumeRequest{
		"b":   {Type: structs.VolumeTypeHost, Source: "src-b"},
		"a":   {Type: structs.VolumeTypeHost, Source: "src-a"},
		"pa":  {Type: structs.VolumeTypeHost, Source: "src-pa", PerAlloc: true},
		"csi": {Type: structs.VolumeTypeCSI, Source: "src-csi"},
		"ok":  {Type: structs.VolumeTypeHost, Source: "src-ok"},
	}
	have := map[string]*structs.ClientHostVolumeConfig{"src-ok": {Name: "src-ok"}}
	require.Equal(t, []string{"src-a", "src-b", "src-pa" + structs.AllocSuffix("job-a.group-a[0]")}, missingHostVolumes(requested, have, "job-a.group-a[0]"))
}

// The error log names the missing sources and no path of the client, whether
// the entries are static or belong to other namespaces.
func TestVolumeHook_prepareHostVolumes_LogsOnlySources(t *testing.T) {
	ci.Parallel(t)

	live := map[string]*structs.ClientHostVolumeConfig{
		"static-a": {Name: "static-a", Path: "/synthetic/static-path"},
		"other":    {Name: "other", Path: "/synthetic/other-path", ID: "99999999-2222-3333-4444-555555555555"},
	}
	hook, req := hostVolumeHookEnv(t, nil, func(string) map[string]*structs.ClientHostVolumeConfig { return live }, closedChan)
	var buf bytes.Buffer
	hook.logger = hclog.New(&hclog.LoggerOptions{Output: &buf, Level: hclog.Trace})

	_, err := hook.prepareHostVolumes(context.Background(), req, hostVolumeRequests(false))
	require.Error(t, err)
	out := buf.String()
	require.Contains(t, out, "data-a")
	require.NotContains(t, out, "/synthetic")
	require.NotContains(t, out, "99999999")
}

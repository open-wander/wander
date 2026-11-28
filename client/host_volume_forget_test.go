// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-wander/wander/ci"
	arinterfaces "github.com/open-wander/wander/client/allocrunner/interfaces"
	"github.com/open-wander/wander/client/hostvolume"
	"github.com/open-wander/wander/client/structs"
	nstructs "github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

// fakeVolumeAllocRunner answers only what the in-use check reads. The embedded
// interface is nil, so any other call fails the test loudly.
type fakeVolumeAllocRunner struct {
	arinterfaces.AllocRunner
	alloc     *nstructs.Allocation
	done      chan struct{}
	destroyed bool
}

func newFakeVolumeAllocRunner(alloc *nstructs.Allocation) *fakeVolumeAllocRunner {
	return &fakeVolumeAllocRunner{alloc: alloc, done: make(chan struct{})}
}

func (f *fakeVolumeAllocRunner) Alloc() *nstructs.Allocation { return f.alloc }
func (f *fakeVolumeAllocRunner) WaitCh() <-chan struct{}     { return f.done }
func (f *fakeVolumeAllocRunner) IsDestroyed() bool           { return f.destroyed }

// The client calls these on shutdown.
func (f *fakeVolumeAllocRunner) Shutdown()                   {}
func (f *fakeVolumeAllocRunner) PersistState() error         { return nil }
func (f *fakeVolumeAllocRunner) Destroy()                    {}
func (f *fakeVolumeAllocRunner) ShutdownCh() <-chan struct{} { return f.done }
func (f *fakeVolumeAllocRunner) DestroyCh() <-chan struct{}  { return f.done }

func (c *Client) addFakeAllocRunner(f *fakeVolumeAllocRunner) {
	c.allocLock.Lock()
	defer c.allocLock.Unlock()
	if c.allocs == nil {
		c.allocs = map[string]arinterfaces.AllocRunner{}
	}
	c.allocs[f.alloc.ID] = f
}

func TestClient_hostVolumeInUse(t *testing.T) {
	ci.Parallel(t)

	perAlloc := func(a *nstructs.Allocation) {
		req := a.Job.TaskGroups[0].Volumes["data"]
		req.PerAlloc = true
		a.Name = a.Job.Name + "." + a.TaskGroup + "[3]"
	}
	cases := []struct {
		name      string
		namespace string
		volName   string
		mutate    func(*nstructs.Allocation)
		finish    func(*fakeVolumeAllocRunner)
		want      bool
	}{
		{"live allocation mounts it", "ns-a", "data-a", nil, nil, true},
		{"other namespace", "ns-b", "data-a", nil, nil, false},
		{"other volume name", "ns-a", "data-b", nil, nil, false},
		{"per_alloc source resolves", "ns-a", "data-a[3]", perAlloc, nil, true},
		{"per_alloc source of another alloc", "ns-a", "data-a[4]", perAlloc, nil, false},
		{"finished allocation", "ns-a", "data-a", nil, func(f *fakeVolumeAllocRunner) { close(f.done) }, false},
		{"destroyed allocation", "ns-a", "data-a", nil, func(f *fakeVolumeAllocRunner) { f.destroyed = true }, false},
		{"no job", "ns-a", "data-a", func(a *nstructs.Allocation) { a.Job = nil }, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := hostVolumeTestClient(nil)
			alloc := hostVolumeClientAlloc("ns-a")
			if tc.mutate != nil {
				tc.mutate(alloc)
			}
			f := newFakeVolumeAllocRunner(alloc)
			if tc.finish != nil {
				tc.finish(f)
			}
			c.addFakeAllocRunner(f)
			must.Eq(t, tc.want, c.hostVolumeInUse(tc.namespace, tc.volName))
		})
	}

	t.Run("no allocations", func(t *testing.T) {
		must.False(t, hostVolumeTestClient(nil).hostVolumeInUse("ns-a", "data-a"))
	})
}

// The client wires the in-use check into its manager: a live allocation stops
// Delete, Deregister and Forget with volume_in_use and changes nothing.
func TestHostVolume_InUseRefusesRemoval(t *testing.T) {
	ci.Parallel(t)

	c, _, scratch := newCreateClient(t)
	dir := t.TempDir()
	must.NoError(t, os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0o600))
	var reg structs.HostVolumeRegisterResponse
	must.NoError(t, c.ClientRPC("HostVolume.Register", &structs.HostVolumeRegisterRequest{
		VolumeID: "vol-r", Namespace: "ns-a", Name: "data-r", NodeID: c.NodeID(), HostPath: dir,
	}, &reg))
	var created structs.HostVolumeCreateResponse
	must.NoError(t, c.ClientRPC("HostVolume.Create", &structs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "ns-a", Name: "data-a", NodeID: c.NodeID(), PluginID: "plug",
	}, &created))

	alloc := hostVolumeClientAlloc("ns-a")
	alloc.Job.TaskGroups[0].Volumes["reg"] = &nstructs.VolumeRequest{Name: "reg", Type: nstructs.VolumeTypeHost, Source: "data-r"}
	f := newFakeVolumeAllocRunner(alloc)
	c.addFakeAllocRunner(f)

	call := func(method, id string) string {
		t.Helper()
		switch method {
		case "HostVolume.Delete":
			var resp structs.HostVolumeDeleteResponse
			must.NoError(t, c.ClientRPC(method, &structs.HostVolumeDeleteRequest{VolumeID: id, Namespace: "ns-a", NodeID: c.NodeID()}, &resp))
			return resp.Reason
		case "HostVolume.Deregister":
			var resp structs.HostVolumeDeregisterResponse
			must.NoError(t, c.ClientRPC(method, &structs.HostVolumeDeregisterRequest{VolumeID: id, Namespace: "ns-a", NodeID: c.NodeID()}, &resp))
			return resp.Reason
		}
		var resp structs.HostVolumeForgetResponse
		must.NoError(t, c.ClientRPC(method, &structs.HostVolumeForgetRequest{VolumeID: id, Namespace: "ns-a", NodeID: c.NodeID()}, &resp))
		return resp.Reason
	}

	must.Eq(t, hostvolume.ReasonVolumeInUse, call("HostVolume.Delete", "vol-a"))
	must.Eq(t, hostvolume.ReasonVolumeInUse, call("HostVolume.Deregister", "vol-r"))
	must.Eq(t, hostvolume.ReasonVolumeInUse, call("HostVolume.Forget", "vol-a"))
	must.Eq(t, hostvolume.ReasonVolumeInUse, call("HostVolume.Forget", "vol-r"))
	must.Eq(t, 0, deleteCount(t, scratch))
	must.Eq(t, "vol-a", c.GetConfig().Node.HostVolumes["data-a"].ID)
	must.Eq(t, "vol-r", c.GetConfig().Node.HostVolumes["data-r"].ID)

	// With the allocation gone, forget drops both records, keeps both
	// directories and runs no plugin.
	close(f.done)
	must.Eq(t, "", call("HostVolume.Forget", "vol-a"))
	must.Eq(t, "", call("HostVolume.Forget", "vol-r"))
	must.MapLen(t, 0, c.GetConfig().Node.HostVolumes)
	must.Eq(t, 0, deleteCount(t, scratch))
	_, err := os.Stat(filepath.Join(dir, "marker"))
	must.NoError(t, err)
}

func TestHostVolume_Forget(t *testing.T) {
	ci.Parallel(t)

	c, volumes, scratch := newCreateClient(t)
	var created structs.HostVolumeCreateResponse
	must.NoError(t, c.ClientRPC("HostVolume.Create", &structs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: c.NodeID(), PluginID: "plug",
	}, &created))
	call := func(id, ns string) structs.HostVolumeForgetResponse {
		t.Helper()
		var resp structs.HostVolumeForgetResponse
		must.NoError(t, c.ClientRPC("HostVolume.Forget", &structs.HostVolumeForgetRequest{
			VolumeID: id, Namespace: ns, NodeID: c.NodeID(),
		}, &resp))
		return resp
	}

	must.Eq(t, hostvolume.ReasonDeleteMismatch, call("vol-a", "other").Reason)
	must.Eq(t, "vol-a", c.GetConfig().Node.HostVolumes["data"].ID)

	must.Eq(t, structs.HostVolumeForgetResponse{VolumeID: "vol-a"}, call("vol-a", "default"))
	must.MapLen(t, 0, c.GetConfig().Node.HostVolumes)
	must.Eq(t, 0, deleteCount(t, scratch))
	_, err := os.Stat(filepath.Join(volumes, "vol-a"))
	must.NoError(t, err)

	// A repeat answers success and a late create of the ID is refused.
	must.Eq(t, structs.HostVolumeForgetResponse{VolumeID: "vol-a"}, call("vol-a", "default"))
	var again structs.HostVolumeCreateResponse
	must.NoError(t, c.ClientRPC("HostVolume.Create", &structs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: c.NodeID(), PluginID: "plug",
	}, &again))
	must.NotEq(t, "", again.Reason)
	must.Eq(t, 1, createCount(t, scratch))
}

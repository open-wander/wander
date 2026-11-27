// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/hostvolume"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/client/structs"
	"github.com/open-wander/wander/helper/testlog"
	nstructs "github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

func TestHostVolume_Register(t *testing.T) {
	ci.Parallel(t)

	staticDir, dir := t.TempDir(), t.TempDir()
	c, cleanup := TestClient(t, func(conf *config.Config) {
		conf.HostVolumes = map[string]*nstructs.ClientHostVolumeConfig{
			"static": {Name: "static", Path: staticDir},
		}
	})
	t.Cleanup(func() { _ = cleanup() })

	valid := func() *structs.HostVolumeRegisterRequest {
		return &structs.HostVolumeRegisterRequest{
			VolumeID: "vol-a", Namespace: "default", Name: "data",
			NodeID: c.NodeID(), NodePool: "default", HostPath: dir,
		}
	}

	// Rejections leave nothing behind.
	rejections := []struct {
		name   string
		mutate func(*structs.HostVolumeRegisterRequest)
		reason string
	}{
		{"relative path", func(r *structs.HostVolumeRegisterRequest) { r.HostPath = "rel" }, hostvolume.ReasonInvalidPath},
		{"unclean path", func(r *structs.HostVolumeRegisterRequest) { r.HostPath = dir + "/." }, hostvolume.ReasonInvalidPath},
		{"nul path", func(r *structs.HostVolumeRegisterRequest) { r.HostPath = dir + "\x00" }, hostvolume.ReasonInvalidPath},
		{"missing path", func(r *structs.HostVolumeRegisterRequest) { r.HostPath = filepath.Join(dir, "gone") }, hvstate.ReasonMissingPath},
		{"empty id", func(r *structs.HostVolumeRegisterRequest) { r.VolumeID = "" }, hostvolume.ReasonInvalidInput},
		{"empty name", func(r *structs.HostVolumeRegisterRequest) { r.Name = "" }, hostvolume.ReasonInvalidInput},
		{"empty namespace", func(r *structs.HostVolumeRegisterRequest) { r.Namespace = "" }, hostvolume.ReasonInvalidInput},
		{"other node", func(r *structs.HostVolumeRegisterRequest) { r.NodeID = "node-synthetic-other" }, hvstate.ReasonNodeMismatch},
		{"static name", func(r *structs.HostVolumeRegisterRequest) { r.Name = "static" }, hvstate.ReasonNameConflict},
	}
	for _, tc := range rejections {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(req)
			var resp structs.HostVolumeRegisterResponse
			must.NoError(t, c.ClientRPC("HostVolume.Register", req, &resp))
			must.Eq(t, tc.reason, resp.Reason)
			must.Eq(t, "", resp.Status)
			must.Eq(t, 0, len(c.hostVolumeManager.Statuses()))
			must.MapLen(t, 1, c.GetConfig().Node.HostVolumes)
		})
	}

	var resp structs.HostVolumeRegisterResponse
	must.NoError(t, c.ClientRPC("HostVolume.Register", valid(), &resp))
	must.Eq(t, structs.HostVolumeRegisterResponse{VolumeID: "vol-a", HostPath: dir, Status: "ready"}, resp)
	hv := c.GetConfig().Node.HostVolumes
	must.Eq(t, nstructs.ClientHostVolumeConfig{Name: "data", Path: dir, ID: "vol-a"}, *hv["data"])

	t.Run("name used by another dynamic volume", func(t *testing.T) {
		req := valid()
		req.VolumeID, req.Namespace = "vol-b", "other"
		var r structs.HostVolumeRegisterResponse
		must.NoError(t, c.ClientRPC("HostVolume.Register", req, &r))
		must.Eq(t, hvstate.ReasonNameConflict, r.Reason)
		must.Eq(t, 1, len(c.hostVolumeManager.Statuses()))
	})

	t.Run("repeat", func(t *testing.T) {
		var r structs.HostVolumeRegisterResponse
		must.NoError(t, c.ClientRPC("HostVolume.Register", valid(), &r))
		must.Eq(t, "ready", r.Status)

		req := valid()
		req.Namespace = "other"
		r = structs.HostVolumeRegisterResponse{}
		must.NoError(t, c.ClientRPC("HostVolume.Register", req, &r))
		must.Eq(t, hostvolume.ReasonRegistrationMismatch, r.Reason)
		must.Eq(t, 1, len(c.hostVolumeManager.Statuses()))
	})
}

// blockingPutStore holds the first record write until released, which keeps
// the volume lock with the registration that is writing.
type blockingPutStore struct {
	hostvolume.Store
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (s *blockingPutStore) PutHostVolume(r *hvstate.Record) error {
	s.once.Do(func() { close(s.entered) })
	<-s.release
	return s.Store.PutHostVolume(r)
}

// A registration that is waiting for the volume lock when the client shuts
// down was not decided. It must return an error, not a classified rejection.
func TestHostVolume_Register_CanceledWaitIsAnError(t *testing.T) {
	ci.Parallel(t)

	const node = "node-synthetic-1"
	dir := t.TempDir()
	store := &blockingPutStore{
		Store:   state.NewMemDB(testlog.HCLogger(t)),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	m, err := hostvolume.NewManager(hostvolume.ManagerConfig{
		Store: store, Ops: &hostvolume.Ops{}, NodeID: node,
		Publish: func(map[string]*nstructs.ClientHostVolumeConfig) {},
	})
	must.NoError(t, err)
	c := &Client{shutdownCh: make(chan struct{}), hostVolumeManager: m}
	req := &structs.HostVolumeRegisterRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data",
		NodeID: node, NodePool: "default", HostPath: dir,
	}

	// The first registration takes the volume lock and stops in the store.
	firstErr := make(chan error, 1)
	go func() {
		var resp structs.HostVolumeRegisterResponse
		firstErr <- (&HostVolume{c}).Register(req, &resp)
	}()
	select {
	case <-store.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the first registration never reached the store")
	}

	// The second waits for the lock until the client shuts down.
	var resp structs.HostVolumeRegisterResponse
	secondErr := make(chan error, 1)
	go func() { secondErr <- (&HostVolume{c}).Register(req, &resp) }()
	select {
	case err := <-secondErr:
		t.Fatalf("the second registration did not wait for the lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(c.shutdownCh)
	select {
	case err = <-secondErr:
	case <-time.After(10 * time.Second):
		t.Fatal("the second registration did not return after shutdown")
	}
	close(store.release)
	<-firstErr

	must.ErrorContains(t, err, "HostVolume.Register")
	must.Eq(t, structs.HostVolumeRegisterResponse{}, resp)
}

// newCreateClient starts a client whose volumes directory and plugin
// directory are temporary. The synthetic plugin "plug" appends a line to
// scratch/creates on each create and answers with a directory under the
// volumes directory.
func newCreateClient(t *testing.T) (c *Client, volumes, scratch string) {
	t.Helper()
	plugins := t.TempDir()
	volumes, scratch = t.TempDir(), t.TempDir()
	body := "#!/bin/sh\n" + `case "$1" in
fingerprint) echo '{"version":"1.0.0"}' ;;
create)
  echo "$DHV_VOLUME_ID" >> '` + scratch + `/creates'
  d="$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
  mkdir -p "$d"
  printf '{"path":"%s","bytes":4096}\n' "$d" ;;
delete)
  echo "$DHV_VOLUME_ID" >> '` + scratch + `/deletes'
  [ -f '` + scratch + `/faildelete' ] && exit 1
  rm -rf "$DHV_CREATED_PATH" ;;
esac
`
	must.NoError(t, os.WriteFile(filepath.Join(plugins, "plug"), []byte(body), 0o755))
	must.NoError(t, os.Chmod(filepath.Join(plugins, "plug"), 0o755))
	c, cleanup := TestClient(t, func(conf *config.Config) {
		conf.HostVolumePluginDir = plugins
		conf.HostVolumesDir = volumes
	})
	t.Cleanup(func() { _ = cleanup() })
	return c, volumes, scratch
}

func createCount(t *testing.T, scratch string) int {
	t.Helper()
	return lineCount(t, filepath.Join(scratch, "creates"))
}

func deleteCount(t *testing.T, scratch string) int {
	t.Helper()
	return lineCount(t, filepath.Join(scratch, "deletes"))
}

func lineCount(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	must.NoError(t, err)
	return strings.Count(string(b), "\n")
}

func TestHostVolume_Create(t *testing.T) {
	ci.Parallel(t)

	c, volumes, scratch := newCreateClient(t)
	valid := func() *structs.HostVolumeCreateRequest {
		return &structs.HostVolumeCreateRequest{
			VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: c.NodeID(),
			NodePool: "default", PluginID: "plug", CapacityMinBytes: 1, CapacityMaxBytes: 8192,
			Parameters: map[string]string{"k": "v"},
		}
	}

	// Rejections come back as a reason with no RPC error and run nothing.
	rejections := []struct {
		name   string
		mutate func(*structs.HostVolumeCreateRequest)
		reason string
	}{
		{"other node", func(r *structs.HostVolumeCreateRequest) { r.NodeID = "node-synthetic-other" }, hvstate.ReasonNodeMismatch},
		{"empty name", func(r *structs.HostVolumeCreateRequest) { r.Name = "" }, hostvolume.ReasonInvalidInput},
		{"bad plugin id", func(r *structs.HostVolumeCreateRequest) { r.PluginID = "../x" }, hostvolume.ReasonInvalidInput},
		{"missing plugin", func(r *structs.HostVolumeCreateRequest) { r.PluginID = "absent" }, hvstate.ReasonPluginUnavailable},
	}
	for _, tc := range rejections {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(req)
			var resp structs.HostVolumeCreateResponse
			must.NoError(t, c.ClientRPC("HostVolume.Create", req, &resp))
			must.Eq(t, tc.reason, resp.Reason)
			must.Eq(t, "", resp.Status)
			must.Eq(t, 0, len(c.hostVolumeManager.Statuses()))
			must.Eq(t, 0, createCount(t, scratch))
		})
	}

	var resp structs.HostVolumeCreateResponse
	must.NoError(t, c.ClientRPC("HostVolume.Create", valid(), &resp))
	want := structs.HostVolumeCreateResponse{
		VolumeID: "vol-a", HostPath: filepath.Join(volumes, "vol-a"), CapacityBytes: 4096, Status: "ready",
	}
	must.Eq(t, want, resp)
	must.Eq(t, 1, createCount(t, scratch))
	must.Eq(t, "vol-a", c.GetConfig().Node.HostVolumes["data"].ID)

	t.Run("repeat runs nothing and answers the same", func(t *testing.T) {
		var again structs.HostVolumeCreateResponse
		must.NoError(t, c.ClientRPC("HostVolume.Create", valid(), &again))
		must.Eq(t, want, again)
		must.Eq(t, 1, createCount(t, scratch))
	})

	t.Run("mismatched repeat is refused", func(t *testing.T) {
		req := valid()
		req.Name = "other"
		var r structs.HostVolumeCreateResponse
		must.NoError(t, c.ClientRPC("HostVolume.Create", req, &r))
		must.Eq(t, hostvolume.ReasonCreateMismatch, r.Reason)
		must.Eq(t, "", r.Status)
		must.Eq(t, 1, createCount(t, scratch))
		must.Eq(t, 1, len(c.hostVolumeManager.Statuses()))
		must.Eq(t, "data", c.hostVolumeManager.Statuses()[0].Name)
	})
}

func TestHostVolume_Delete(t *testing.T) {
	ci.Parallel(t)

	c, volumes, scratch := newCreateClient(t)
	create := func(id, name string) {
		t.Helper()
		var resp structs.HostVolumeCreateResponse
		must.NoError(t, c.ClientRPC("HostVolume.Create", &structs.HostVolumeCreateRequest{
			VolumeID: id, Namespace: "default", Name: name, NodeID: c.NodeID(),
			NodePool: "default", PluginID: "plug",
		}, &resp))
		must.Eq(t, "", resp.Reason)
	}
	del := func(id string) structs.HostVolumeDeleteResponse {
		t.Helper()
		var resp structs.HostVolumeDeleteResponse
		must.NoError(t, c.ClientRPC("HostVolume.Delete", &structs.HostVolumeDeleteRequest{
			VolumeID: id, Namespace: "default", NodeID: c.NodeID(),
		}, &resp))
		return resp
	}

	create("vol-a", "data")
	must.Eq(t, "vol-a", c.GetConfig().Node.HostVolumes["data"].ID)

	t.Run("mismatch is a reason and runs nothing", func(t *testing.T) {
		var resp structs.HostVolumeDeleteResponse
		must.NoError(t, c.ClientRPC("HostVolume.Delete", &structs.HostVolumeDeleteRequest{
			VolumeID: "vol-a", Namespace: "other", NodeID: c.NodeID(),
		}, &resp))
		must.Eq(t, hostvolume.ReasonDeleteMismatch, resp.Reason)
		must.Eq(t, 0, deleteCount(t, scratch))
		must.Eq(t, 1, len(c.hostVolumeManager.Statuses()))
	})

	t.Run("other node is a reason", func(t *testing.T) {
		var resp structs.HostVolumeDeleteResponse
		must.NoError(t, c.ClientRPC("HostVolume.Delete", &structs.HostVolumeDeleteRequest{
			VolumeID: "vol-a", Namespace: "default", NodeID: "node-synthetic-other",
		}, &resp))
		must.Eq(t, hvstate.ReasonNodeMismatch, resp.Reason)
		must.Eq(t, 0, deleteCount(t, scratch))
	})

	t.Run("plugin failure keeps the record and is a reason", func(t *testing.T) {
		must.NoError(t, os.WriteFile(filepath.Join(scratch, "faildelete"), nil, 0o600))
		resp := del("vol-a")
		must.Eq(t, hostvolume.ReasonNonZeroExit, resp.Reason)
		must.Eq(t, 1, deleteCount(t, scratch))
		must.Eq(t, 1, len(c.hostVolumeManager.Statuses()))
		must.NoError(t, os.Remove(filepath.Join(scratch, "faildelete")))
	})

	t.Run("delete then repeat", func(t *testing.T) {
		before := deleteCount(t, scratch)
		must.Eq(t, structs.HostVolumeDeleteResponse{VolumeID: "vol-a"}, del("vol-a"))
		must.Eq(t, before+1, deleteCount(t, scratch))
		must.MapLen(t, 0, c.GetConfig().Node.HostVolumes)
		_, statErr := os.Stat(filepath.Join(volumes, "vol-a"))
		must.True(t, os.IsNotExist(statErr))

		must.Eq(t, structs.HostVolumeDeleteResponse{VolumeID: "vol-a"}, del("vol-a"))
		must.Eq(t, before+1, deleteCount(t, scratch))
	})

	t.Run("registered volume is refused", func(t *testing.T) {
		dir := t.TempDir()
		var reg structs.HostVolumeRegisterResponse
		must.NoError(t, c.ClientRPC("HostVolume.Register", &structs.HostVolumeRegisterRequest{
			VolumeID: "vol-r", Namespace: "default", Name: "ext", NodeID: c.NodeID(), HostPath: dir,
		}, &reg))
		must.Eq(t, "ready", reg.Status)
		before := deleteCount(t, scratch)
		must.Eq(t, hostvolume.ReasonVolumeKindConflict, del("vol-r").Reason)
		must.Eq(t, before, deleteCount(t, scratch))
		must.Eq(t, "vol-r", c.GetConfig().Node.HostVolumes["ext"].ID)
		_, statErr := os.Stat(dir)
		must.NoError(t, statErr)
	})
}

func TestHostVolume_Deregister(t *testing.T) {
	ci.Parallel(t)

	c, _, scratch := newCreateClient(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	must.NoError(t, os.WriteFile(marker, []byte("x"), 0o600))
	var reg structs.HostVolumeRegisterResponse
	must.NoError(t, c.ClientRPC("HostVolume.Register", &structs.HostVolumeRegisterRequest{
		VolumeID: "vol-r", Namespace: "default", Name: "ext", NodeID: c.NodeID(), HostPath: dir,
	}, &reg))
	must.Eq(t, "ready", reg.Status)
	var created structs.HostVolumeCreateResponse
	must.NoError(t, c.ClientRPC("HostVolume.Create", &structs.HostVolumeCreateRequest{
		VolumeID: "vol-a", Namespace: "default", Name: "data", NodeID: c.NodeID(), PluginID: "plug",
	}, &created))

	call := func(id, ns string) structs.HostVolumeDeregisterResponse {
		t.Helper()
		var resp structs.HostVolumeDeregisterResponse
		must.NoError(t, c.ClientRPC("HostVolume.Deregister", &structs.HostVolumeDeregisterRequest{
			VolumeID: id, Namespace: ns, NodeID: c.NodeID(),
		}, &resp))
		return resp
	}

	must.Eq(t, hostvolume.ReasonVolumeKindConflict, call("vol-a", "default").Reason)
	must.Eq(t, hostvolume.ReasonDeleteMismatch, call("vol-r", "other").Reason)
	must.Eq(t, "vol-r", c.GetConfig().Node.HostVolumes["ext"].ID)
	must.Eq(t, "vol-a", c.GetConfig().Node.HostVolumes["data"].ID)

	must.Eq(t, structs.HostVolumeDeregisterResponse{VolumeID: "vol-r"}, call("vol-r", "default"))
	_, ok := c.GetConfig().Node.HostVolumes["ext"]
	must.False(t, ok)
	_, err := os.Stat(marker)
	must.NoError(t, err)
	must.Eq(t, 0, deleteCount(t, scratch))

	must.Eq(t, structs.HostVolumeDeregisterResponse{VolumeID: "vol-r"}, call("vol-r", "default"))
	must.Eq(t, 0, deleteCount(t, scratch))
}

// A delete that is waiting for the volume lock when the client shuts down
// was not decided. It must return an error, not a classified rejection.
func TestHostVolume_Delete_CanceledWaitIsAnError(t *testing.T) {
	ci.Parallel(t)

	const node = "node-synthetic-1"
	dir := t.TempDir()
	store := &blockingPutStore{
		Store:   state.NewMemDB(testlog.HCLogger(t)),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	m, err := hostvolume.NewManager(hostvolume.ManagerConfig{
		Store: store, Ops: &hostvolume.Ops{}, NodeID: node,
		Publish: func(map[string]*nstructs.ClientHostVolumeConfig) {},
	})
	must.NoError(t, err)
	c := &Client{shutdownCh: make(chan struct{}), hostVolumeManager: m}

	// A registration holds the volume lock and stops in the store.
	firstErr := make(chan error, 1)
	go func() {
		var resp structs.HostVolumeRegisterResponse
		firstErr <- (&HostVolume{c}).Register(&structs.HostVolumeRegisterRequest{
			VolumeID: "vol-a", Namespace: "default", Name: "data",
			NodeID: node, NodePool: "default", HostPath: dir,
		}, &resp)
	}()
	select {
	case <-store.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the registration never reached the store")
	}

	req := &structs.HostVolumeDeleteRequest{VolumeID: "vol-a", Namespace: "default", NodeID: node}
	type out struct {
		method string
		err    error
		empty  bool
	}
	done := make(chan out, 2)
	go func() {
		var resp structs.HostVolumeDeleteResponse
		err := (&HostVolume{c}).Delete(req, &resp)
		done <- out{"Delete", err, resp == structs.HostVolumeDeleteResponse{}}
	}()
	go func() {
		var resp structs.HostVolumeDeregisterResponse
		err := (&HostVolume{c}).Deregister((*structs.HostVolumeDeregisterRequest)(req), &resp)
		done <- out{"Deregister", err, resp == structs.HostVolumeDeregisterResponse{}}
	}()
	select {
	case o := <-done:
		t.Fatalf("%s did not wait for the lock: %v", o.method, o.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(c.shutdownCh)
	for range 2 {
		select {
		case o := <-done:
			must.ErrorContains(t, o.err, "HostVolume."+o.method)
			must.True(t, o.empty)
		case <-time.After(10 * time.Second):
			t.Fatal("a call did not return after shutdown")
		}
	}
	close(store.release)
	<-firstErr
}

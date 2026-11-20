// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"path/filepath"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/hostvolume"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/open-wander/wander/client/structs"
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

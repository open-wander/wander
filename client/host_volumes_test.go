// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/client/hostvolume"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
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

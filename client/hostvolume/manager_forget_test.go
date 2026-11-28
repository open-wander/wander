// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package hostvolume

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/shoenig/test/must"
)

// inUseHook returns an InUse hook that reports a fixed answer and records the
// namespace and name it was asked about.
func inUseHook(answer *atomic.Bool, asked *[]string) func(namespace, name string) bool {
	return func(namespace, name string) bool {
		*asked = append(*asked, namespace+"/"+name)
		return answer.Load()
	}
}

func TestManager_Forget(t *testing.T) {
	t.Run("plugin volume keeps the directory and runs no plugin", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		dir := filepath.Join(e.volumes, "vol-a")

		must.NoError(t, m.Forget(ctx5(t), delReq("vol-a")))
		must.Nil(t, e.stored("vol-a"))
		must.MapLen(t, 0, e.published())
		_, ok := m.Status("vol-a")
		must.False(t, ok)
		must.Eq(t, 0, e.lines("deletes"))
		_, err = os.Stat(dir)
		must.NoError(t, err)
		must.NotNil(t, e.tombstone("vol-a"))
	})

	t.Run("registered volume keeps the directory", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		dir := t.TempDir()
		must.NoError(t, os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0o600))
		m := e.manager()
		_, err := m.AddRegistered(ctx5(t), RegisterRequest{NodeID: testNode, Namespace: "ns-synthetic", VolumeID: "vol-r", Name: "reg", Path: dir})
		must.NoError(t, err)

		must.NoError(t, m.Forget(ctx5(t), delReq("vol-r")))
		must.Nil(t, e.stored("vol-r"))
		must.MapLen(t, 0, e.published())
		_, err = os.Stat(filepath.Join(dir, "marker"))
		must.NoError(t, err)
		must.NotNil(t, e.tombstone("vol-r"))
	})

	t.Run("a volume not held writes the tombstone and succeeds", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		must.NoError(t, m.Forget(ctx5(t), delReq("vol-never")))
		must.NotNil(t, e.tombstone("vol-never"))
		must.NoError(t, m.Forget(ctx5(t), delReq("vol-never")))
		must.Eq(t, 0, e.lines("deletes"))
	})

	t.Run("the tombstone refuses a late create", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		must.NoError(t, m.Forget(ctx5(t), delReq("vol-a")))
		_, err = m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.Error(t, err)
	})

	t.Run("mismatch changes nothing", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		req := delReq("vol-a")
		req.Namespace = "other"
		asOpError(t, m.Forget(ctx5(t), req), OperationForget, ReasonDeleteMismatch)
		must.NotNil(t, e.stored("vol-a"))
		must.Nil(t, e.tombstone("vol-a"))
	})
}

// A live allocation that mounts the volume stops a delete, a deregistration
// and a forget, and changes nothing: no tombstone, no plugin run, the record
// and the published entry stay.
func TestManager_InUseRefusal(t *testing.T) {
	type call struct {
		name   string
		op     string
		volume string
	}
	calls := []call{
		{"delete", OperationDelete, "plugin"},
		{"deregister", OperationDeregister, "registered"},
		{"forget plugin volume", OperationForget, "plugin"},
		{"forget registered volume", OperationForget, "registered"},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			e := newMgrEnv(t)
			e.installPlugin()
			var live atomic.Bool
			var asked []string
			m := e.manager(func(c *ManagerConfig) { c.InUse = inUseHook(&live, &asked) })
			if tc.volume == "plugin" {
				_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
				must.NoError(t, err)
			} else {
				_, err := m.AddRegistered(ctx5(t), RegisterRequest{NodeID: testNode, Namespace: "ns-synthetic", VolumeID: "vol-a", Name: "data", Path: t.TempDir()})
				must.NoError(t, err)
			}
			ctx := ctx5(t)
			run := func() error {
				switch tc.name {
				case "delete":
					return m.Delete(ctx, delReq("vol-a"))
				case "deregister":
					return m.Deregister(ctx, delReq("vol-a"))
				}
				return m.Forget(ctx, delReq("vol-a"))
			}

			live.Store(true)
			asOpError(t, run(), tc.op, ReasonVolumeInUse)
			must.Eq(t, []string{"ns-synthetic/data"}, asked)
			must.NotNil(t, e.stored("vol-a"))
			must.Nil(t, e.tombstone("vol-a"))
			must.Eq(t, 0, e.lines("deletes"))
			must.Eq(t, "vol-a", e.published()["data"].ID)

			// Once the allocation is gone the same call goes through.
			live.Store(false)
			must.NoError(t, run())
			must.Nil(t, e.stored("vol-a"))
			must.NotNil(t, e.tombstone("vol-a"))
		})
	}

	t.Run("a nil hook means no check", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		must.NoError(t, m.Delete(ctx5(t), delReq("vol-a")))
	})

	t.Run("a volume not held is not checked", func(t *testing.T) {
		e := newMgrEnv(t)
		var live atomic.Bool
		live.Store(true)
		var asked []string
		m := e.manager(func(c *ManagerConfig) { c.InUse = inUseHook(&live, &asked) })
		must.NoError(t, m.Forget(ctx5(t), delReq("vol-never")))
		must.Len(t, 0, asked)
	})

	t.Run("a kind conflict wins over in use", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		var live atomic.Bool
		live.Store(true)
		var asked []string
		m := e.manager(func(c *ManagerConfig) { c.InUse = inUseHook(&live, &asked) })
		_, err := m.AddRegistered(ctx5(t), RegisterRequest{NodeID: testNode, Namespace: "ns-synthetic", VolumeID: "vol-r", Name: "reg", Path: t.TempDir()})
		must.NoError(t, err)
		err = m.Delete(ctx5(t), delReq("vol-r"))
		must.ErrorIs(t, err, ErrRegisteredVolume)
	})
}

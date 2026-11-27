// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package hostvolume

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/shoenig/test/must"
)

// tombstoneStore wraps a Store and can fail the tombstone calls.
type tombstoneStore struct {
	Store
	mu          sync.Mutex
	failPut     error
	failGet     error
	extraErrs   map[string]error
	tombstonePu int
}

func (s *tombstoneStore) PutHostVolumeTombstone(t *hvstate.Tombstone) error {
	s.mu.Lock()
	s.tombstonePu++
	err := s.failPut
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.Store.PutHostVolumeTombstone(t)
}

func (s *tombstoneStore) GetHostVolumeTombstones() ([]*hvstate.Tombstone, map[string]error, error) {
	s.mu.Lock()
	err, extra := s.failGet, s.extraErrs
	s.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	ts, errs, gerr := s.Store.GetHostVolumeTombstones()
	if errs == nil {
		errs = map[string]error{}
	}
	for k, v := range extra {
		errs[k] = v
	}
	return ts, errs, gerr
}

func (s *tombstoneStore) set(f func(*tombstoneStore)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s)
}

func (e *mgrEnv) tombstone(id string) *hvstate.Tombstone {
	e.t.Helper()
	ts, _, err := e.store.GetHostVolumeTombstones()
	must.NoError(e.t, err)
	for _, x := range ts {
		if x.VolumeID == id {
			return x
		}
	}
	return nil
}

// fakeClock is a clock the test moves.
type fakeClock struct{ now atomic.Int64 }

func newFakeClock(t time.Time) *fakeClock {
	c := &fakeClock{}
	c.now.Store(t.UnixNano())
	return c
}
func (c *fakeClock) Now() time.Time          { return time.Unix(0, c.now.Load()).UTC() }
func (c *fakeClock) Advance(d time.Duration) { c.now.Add(int64(d)) }

func TestManager_TombstoneAfterRemoval(t *testing.T) {
	type action func(e *mgrEnv, m *Manager) error
	create := func(e *mgrEnv, m *Manager) error {
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		return err
	}
	register := func(e *mgrEnv, m *Manager) error {
		_, err := m.AddRegistered(ctx5(t), e.regReq("vol-a", "data", t.TempDir()))
		return err
	}
	for _, tc := range []struct {
		name   string
		setup  action
		remove func(m *Manager) error
		later  action
		op     string
	}{
		{"delete of a held volume", create, func(m *Manager) error { return m.Delete(ctx5(t), delReq("vol-a")) }, create, OperationCreate},
		{"delete of a volume not held", func(*mgrEnv, *Manager) error { return nil },
			func(m *Manager) error { return m.Delete(ctx5(t), delReq("vol-a")) }, create, OperationCreate},
		{"deregister of a held volume", register, func(m *Manager) error { return m.Deregister(ctx5(t), delReq("vol-a")) }, register, "register"},
		{"deregister of a volume not held", func(*mgrEnv, *Manager) error { return nil },
			func(m *Manager) error { return m.Deregister(ctx5(t), delReq("vol-a")) }, register, "register"},
		{"deregister then create", register, func(m *Manager) error { return m.Deregister(ctx5(t), delReq("vol-a")) }, create, OperationCreate},
		{"delete then register", create, func(m *Manager) error { return m.Delete(ctx5(t), delReq("vol-a")) }, register, "register"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newMgrEnv(t)
			e.installPlugin()
			m := e.manager()
			must.NoError(t, tc.setup(e, m))
			must.NoError(t, tc.remove(m))
			must.NotNil(t, e.tombstone("vol-a"))
			creates, deletes, puts := e.lines("creates"), e.lines("deletes"), e.store.putCount()
			pubs := e.pubCount()

			// The refusal comes before any plugin runs and before anything is
			// written or published.
			asOpError(t, tc.later(e, m), tc.op, hvstate.ReasonVolumeDeleted)
			must.Eq(t, creates, e.lines("creates"))
			must.Eq(t, deletes, e.lines("deletes"))
			must.Eq(t, puts, e.store.putCount())
			must.Eq(t, pubs, e.pubCount())
			must.Nil(t, e.stored("vol-a"))
			_, held := m.Status("vol-a")
			must.False(t, held)

			// The refusal survives a client restart.
			m2 := e.manager()
			asOpError(t, tc.later(e, m2), tc.op, hvstate.ReasonVolumeDeleted)
			must.Eq(t, creates, e.lines("creates"))
			must.Eq(t, puts, e.store.putCount())

			// Another ID is not affected.
			other := e.req("vol-b", "other")
			_, err := m2.Create(ctx5(t), "plug", other)
			must.NoError(t, err)
		})
	}
}

// The refusal comes before the answer for a held volume and before the lookup
// of the plugin.
func TestManager_TombstoneRefusesBeforeLookup(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	var lookups atomic.Int32
	m := e.manager(func(c *ManagerConfig) {
		base := (&Manager{cfg: ManagerConfig{Ops: e.ops}}).defaultLookup
		c.Lookup = func(ctx context.Context, id string) (Candidate, error) {
			lookups.Add(1)
			return base(ctx, id)
		}
	})
	must.NoError(t, m.Delete(ctx5(t), delReq("vol-a")))
	asOpError(t, func() error { _, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data")); return err }(),
		OperationCreate, hvstate.ReasonVolumeDeleted)
	must.Eq(t, int32(0), lookups.Load())
}

func TestManager_TombstoneKept(t *testing.T) {
	t.Run("a failed plugin delete keeps the tombstone", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		e.flag("faildelete", true)
		must.Error(t, m.Delete(ctx5(t), delReq("vol-a")))
		must.NotNil(t, e.tombstone("vol-a"))
		must.NotNil(t, e.stored("vol-a"))
	})

	t.Run("refusals that change nothing leave no tombstone", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		_, err = m.AddRegistered(ctx5(t), e.regReq("vol-r", "reg", t.TempDir()))
		must.NoError(t, err)

		mismatch := delReq("vol-a")
		mismatch.Namespace = "other"
		asOpError(t, m.Delete(ctx5(t), mismatch), OperationDelete, ReasonDeleteMismatch)
		asOpError(t, m.Deregister(ctx5(t), mismatch), OperationDeregister, ReasonDeleteMismatch)
		must.ErrorIs(t, m.Delete(ctx5(t), delReq("vol-r")), ErrRegisteredVolume)
		asOpError(t, m.Deregister(ctx5(t), delReq("vol-a")), OperationDeregister, ReasonVolumeKindConflict)
		bad := delReq("vol-a")
		bad.NodeID = otherTestNode
		asOpError(t, m.Delete(ctx5(t), bad), OperationDelete, hvstate.ReasonNodeMismatch)

		ts, _, err := e.store.GetHostVolumeTombstones()
		must.NoError(t, err)
		must.Len(t, 0, ts)
	})

	t.Run("a repeat refreshes the time", func(t *testing.T) {
		e := newMgrEnv(t)
		clock := newFakeClock(time.Unix(1700000000, 0))
		m := e.manager(func(c *ManagerConfig) { c.Clock = clock.Now })
		must.NoError(t, m.Delete(ctx5(t), delReq("vol-a")))
		first := e.tombstone("vol-a").DeletedAt
		clock.Advance(time.Hour)
		must.NoError(t, m.Delete(ctx5(t), delReq("vol-a")))
		must.Eq(t, first.Add(time.Hour), e.tombstone("vol-a").DeletedAt)
	})
}

func TestManager_TombstoneWriteFailureFailsDelete(t *testing.T) {
	for _, tc := range []struct {
		name string
		held bool
		kind hvstate.Kind
		call func(m *Manager) error
	}{
		{"delete held", true, hvstate.KindPlugin, func(m *Manager) error { return m.Delete(ctx5(t), delReq("vol-a")) }},
		{"delete not held", false, hvstate.KindPlugin, func(m *Manager) error { return m.Delete(ctx5(t), delReq("vol-a")) }},
		{"deregister held", true, hvstate.KindRegistered, func(m *Manager) error { return m.Deregister(ctx5(t), delReq("vol-a")) }},
		{"deregister not held", false, hvstate.KindRegistered, func(m *Manager) error { return m.Deregister(ctx5(t), delReq("vol-a")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newMgrEnv(t)
			e.installPlugin()
			store := &tombstoneStore{Store: e.store}
			m := e.manager(func(c *ManagerConfig) { c.Store = store })
			if tc.held {
				if tc.kind == hvstate.KindPlugin {
					_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
					must.NoError(t, err)
				} else {
					_, err := m.AddRegistered(ctx5(t), e.regReq("vol-a", "data", t.TempDir()))
					must.NoError(t, err)
				}
			}
			store.set(func(s *tombstoneStore) { s.failPut = errors.New("synthetic disk failure") })
			err := tc.call(m)
			must.Error(t, err)
			var oe *OpError
			must.False(t, errors.As(err, &oe))
			// The plugin did not run and the record is still held.
			must.Eq(t, 0, e.lines("deletes"))
			must.Nil(t, e.tombstone("vol-a"))
			if tc.held {
				must.NotNil(t, e.stored("vol-a"))
			}

			// The server retries: once the store works the delete finishes.
			store.set(func(s *tombstoneStore) { s.failPut = nil })
			must.NoError(t, tc.call(m))
			must.NotNil(t, e.tombstone("vol-a"))
			must.Nil(t, e.stored("vol-a"))
		})
	}
}

func TestManager_TombstoneRetention(t *testing.T) {
	t.Run("pruned at load", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		clock := newFakeClock(time.Unix(1700000000, 0))
		cfg := func(c *ManagerConfig) { c.Clock = clock.Now; c.TombstoneRetention = time.Hour }
		must.NoError(t, e.manager(cfg).Delete(ctx5(t), delReq("vol-a")))
		must.NotNil(t, e.tombstone("vol-a"))

		clock.Advance(30 * time.Minute)
		m := e.manager(cfg)
		asOpError(t, func() error { _, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data")); return err }(),
			OperationCreate, hvstate.ReasonVolumeDeleted)

		clock.Advance(31 * time.Minute)
		m = e.manager(cfg)
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		must.Nil(t, e.tombstone("vol-a"))
	})

	t.Run("pruned on a tick of Run", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		clock := newFakeClock(time.Unix(1700000000, 0))
		m := e.manager(func(c *ManagerConfig) {
			c.Clock = clock.Now
			c.TombstoneRetention = time.Hour
			c.Interval = 10 * time.Millisecond
		})
		must.NoError(t, m.Delete(ctx5(t), delReq("vol-a")))
		done := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		go func() { m.Run(ctx); close(done) }()
		t.Cleanup(func() { cancel(); <-done })

		time.Sleep(50 * time.Millisecond)
		must.NotNil(t, e.tombstone("vol-a"))
		clock.Advance(2 * time.Hour)
		deadline := time.Now().Add(5 * time.Second)
		for e.tombstone("vol-a") != nil && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		must.Nil(t, e.tombstone("vol-a"))
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
	})

	t.Run("the default is a day", func(t *testing.T) {
		must.Eq(t, 24*time.Hour, DefaultTombstoneRetention)
	})
}

// A tombstone written for every ID that the delete accepts reloads valid, so
// it is pruned when it expires and does not refuse its ID for ever.
func TestManager_TombstoneForDottedIDExpires(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	clock := newFakeClock(time.Unix(1700000000, 0))
	cfg := func(c *ManagerConfig) { c.Clock = clock.Now; c.TombstoneRetention = time.Hour }
	must.NoError(t, e.manager(cfg).Delete(ctx5(t), delReq("vol.1")))
	must.NotNil(t, e.tombstone("vol.1"))

	clock.Advance(2 * time.Hour)
	m := e.manager(cfg)
	m.ensureLoaded()
	must.Nil(t, e.tombstone("vol.1"))
}

func TestManager_TombstoneLoadFailures(t *testing.T) {
	t.Run("a failed record load refuses create and register", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		e.store.setGetError(errors.New("disk read"))
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.ErrorIs(t, err, errStoreUnreadable)
		_, err = m.AddRegistered(ctx5(t), e.regReq("vol-b", "reg", t.TempDir()))
		must.ErrorIs(t, err, errStoreUnreadable)
		must.Eq(t, 0, e.lines("creates"))
		must.Eq(t, 0, e.store.putCount())
	})

	t.Run("a failed tombstone load refuses create and register", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		store := &tombstoneStore{Store: e.store, failGet: errors.New("disk read")}
		m := e.manager(func(c *ManagerConfig) { c.Store = store })
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.ErrorIs(t, err, errStoreUnreadable)
		_, err = m.AddRegistered(ctx5(t), e.regReq("vol-b", "reg", t.TempDir()))
		must.ErrorIs(t, err, errStoreUnreadable)
		must.Eq(t, 0, e.lines("creates"))
		must.Eq(t, 0, e.store.putCount())
	})

	t.Run("an undecodable tombstone still refuses its own ID", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		store := &tombstoneStore{Store: e.store, extraErrs: map[string]error{"vol-a": errors.New("synthetic decode failure")}}
		m := e.manager(func(c *ManagerConfig) { c.Store = store })
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		asOpError(t, err, OperationCreate, hvstate.ReasonVolumeDeleted)
		_, err = m.Create(ctx5(t), "plug", e.req("vol-b", "other"))
		must.NoError(t, err)
		must.Eq(t, 1, e.lines("creates"))
	})

	t.Run("an invalid tombstone counts too", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		// A zero time is invalid, and the ID is the one a create would use.
		must.NoError(t, e.store.PutHostVolumeTombstone(&hvstate.Tombstone{VolumeID: "vol-a"}))
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		asOpError(t, err, OperationCreate, hvstate.ReasonVolumeDeleted)
	})
}

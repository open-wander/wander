// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package hostvolume

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	hvstate "github.com/open-wander/wander/client/hostvolume/state"
	"github.com/open-wander/wander/client/state"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/shoenig/test/must"
)

const (
	testNode      = "node-synthetic-1"
	otherTestNode = "node-synthetic-2"
)

// faultStore wraps a Store and can fail writes.
type faultStore struct {
	Store
	mu      sync.Mutex
	failPut error
}

func (f *faultStore) PutHostVolume(r *hvstate.Record) error {
	f.mu.Lock()
	err := f.failPut
	f.mu.Unlock()
	if err != nil {
		return err
	}
	return f.Store.PutHostVolume(r)
}

func (f *faultStore) setPutError(err error) {
	f.mu.Lock()
	f.failPut = err
	f.mu.Unlock()
}

type mgrEnv struct {
	t       *testing.T
	store   *faultStore
	ops     *Ops
	plugins string
	volumes string
	scratch string
	logs    *bytes.Buffer
	logMu   *sync.Mutex

	mu     sync.Mutex
	pubs   []map[string]*structs.ClientHostVolumeConfig
	static map[string]bool
}

// syncBuffer lets the test read the logs while the manager writes them.
type syncBuffer struct {
	mu  *sync.Mutex
	buf *bytes.Buffer
}

func (b syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func newMgrEnv(t *testing.T) *mgrEnv {
	t.Helper()
	db, err := state.NewBoltStateDB(testlog.HCLogger(t), t.TempDir())
	must.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	e := &mgrEnv{
		t: t, store: &faultStore{Store: db},
		plugins: t.TempDir(), volumes: t.TempDir(), scratch: t.TempDir(),
		logs: &bytes.Buffer{}, logMu: &sync.Mutex{}, static: map[string]bool{},
	}
	e.ops = &Ops{PluginDir: e.plugins, VolumesDir: e.volumes}
	return e
}

func (e *mgrEnv) logText() string {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	return e.logs.String()
}

// installPlugin installs the synthetic plugin. Files in the scratch
// directory switch its behaviour: unhealthy, fail, faildelete, slow, sleep
// and otherpath (whose content is the path create reports).
func (e *mgrEnv) installPlugin() {
	e.t.Helper()
	s := e.scratch
	body := `S='` + s + `'
case "$1" in
fingerprint)
  [ -f "$S/unhealthy" ] && exit 1
  echo '{"version":"1.0.0"}' ;;
create)
  echo "$DHV_VOLUME_ID" >> "$S/creates"
  echo "$DHV_VOLUME_ID|$DHV_NAMESPACE|$DHV_VOLUME_NAME|$DHV_NODE_ID|$DHV_CAPACITY_MIN_BYTES|$DHV_CAPACITY_MAX_BYTES|$DHV_PARAMETERS" >> "$S/requests"
  if [ -f "$S/fail" ]; then echo '{"error":"create failed"}'; exit 1; fi
  if [ -f "$S/sleep" ]; then echo $$ > "$S/pid"; sleep 30; fi
  mkdir "$S/lock" 2>/dev/null || echo overlap >> "$S/overlap"
  d="$DHV_VOLUMES_DIR/$DHV_VOLUME_ID"
  mkdir -p "$d"
  [ -f "$S/slow" ] && sleep 0.1
  rmdir "$S/lock" 2>/dev/null
  [ -f "$S/otherpath" ] && d="$(cat "$S/otherpath")"
  printf '{"path":"%s"}\n' "$d" ;;
delete)
  echo "$DHV_VOLUME_ID" >> "$S/deletes"
  [ -f "$S/faildelete" ] && exit 1
  rm -rf "$DHV_CREATED_PATH" ;;
esac`
	writePlugin(e.t, e.plugins, "plug", body, 0o755)
}

func (e *mgrEnv) flag(name string, on bool) {
	e.t.Helper()
	p := filepath.Join(e.scratch, name)
	if on {
		must.NoError(e.t, os.WriteFile(p, nil, 0o600))
		return
	}
	_ = os.Remove(p)
}

func (e *mgrEnv) lines(name string) int {
	b, err := os.ReadFile(filepath.Join(e.scratch, name))
	if err != nil {
		return 0
	}
	return strings.Count(string(b), "\n")
}

type mgrOpt func(*ManagerConfig)

// manager returns a new manager over the shared store and ops, as after a
// client restart.
func (e *mgrEnv) manager(opts ...mgrOpt) *Manager {
	e.t.Helper()
	cfg := ManagerConfig{
		Logger: hclog.New(&hclog.LoggerOptions{Output: syncBuffer{e.logMu, e.logs}, Level: hclog.Trace}),
		Store:  e.store, Ops: e.ops, NodeID: testNode,
		StaticName: func(n string) bool {
			e.mu.Lock()
			defer e.mu.Unlock()
			return e.static[n]
		},
		Publish: func(m map[string]*structs.ClientHostVolumeConfig) {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.pubs = append(e.pubs, m)
		},
		Clock: time.Now,
	}
	for _, o := range opts {
		o(&cfg)
	}
	m, err := NewManager(cfg)
	must.NoError(e.t, err)
	return m
}

func (e *mgrEnv) published() map[string]*structs.ClientHostVolumeConfig {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.pubs) == 0 {
		return nil
	}
	return e.pubs[len(e.pubs)-1]
}

func (e *mgrEnv) pubCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.pubs)
}

func (e *mgrEnv) setStatic(name string, on bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.static[name] = on
}

func (e *mgrEnv) req(id, name string) OpRequest {
	return OpRequest{VolumeID: id, Namespace: "ns-synthetic", Name: name, NodeID: testNode,
		CapacityMinBytes: 1, CapacityMaxBytes: 2, Parameters: map[string]string{"k": "v"}}
}

// seed stores a record directly, as an earlier run of the client would have.
func (e *mgrEnv) seed(r *hvstate.Record) {
	e.t.Helper()
	must.NoError(e.t, e.store.Store.PutHostVolume(r))
}

func (e *mgrEnv) pluginRec(id, name string) *hvstate.Record {
	return &hvstate.Record{
		VolumeID: id, Namespace: "ns-synthetic", Name: name, NodeID: testNode,
		Kind: hvstate.KindPlugin, PluginID: "plug",
		Parameters: map[string]string{"k": "v"},
		Path:       filepath.Join(e.volumes, id), Status: hvstate.StatusReady,
	}
}

func (e *mgrEnv) registeredRec(id, name, path string) *hvstate.Record {
	return &hvstate.Record{
		VolumeID: id, Namespace: "ns-synthetic", Name: name, NodeID: testNode,
		Kind: hvstate.KindRegistered, Path: path, Status: hvstate.StatusReady,
	}
}

func (e *mgrEnv) stored(id string) *hvstate.Record {
	e.t.Helper()
	recs, _, err := e.store.GetHostVolumes()
	must.NoError(e.t, err)
	for _, r := range recs {
		if r.VolumeID == id {
			return r
		}
	}
	return nil
}

func ctx5(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func mustStatus(t *testing.T, m *Manager, id string) VolumeStatus {
	t.Helper()
	st, ok := m.Status(id)
	must.True(t, ok, must.Sprintf("no status for %s", id))
	return st
}

func TestManager_RegisteredSurvivesRestart(t *testing.T) {
	e := newMgrEnv(t)
	dir := t.TempDir()
	markerPath := filepath.Join(dir, "marker")
	must.NoError(t, os.WriteFile(markerPath, []byte("keep"), 0o600))
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	must.NoError(t, os.Chtimes(markerPath, old, old))

	m1 := e.manager()
	_, err := m1.AddRegistered(ctx5(t), RegisterRequest{VolumeID: "vol-reg", Namespace: "ns", Name: "data", Path: dir})
	must.NoError(t, err)

	m2 := e.manager()
	m2.Restore(ctx5(t))
	st := mustStatus(t, m2, "vol-reg")
	must.Eq(t, hvstate.StatusReady, st.Status)
	must.Eq(t, dir, st.Path)
	must.Eq(t, &structs.ClientHostVolumeConfig{Name: "data", Path: dir, ID: "vol-reg"}, e.published()["data"])

	ents, err := os.ReadDir(dir)
	must.NoError(t, err)
	must.Len(t, 1, ents)
	fi, err := os.Stat(markerPath)
	must.NoError(t, err)
	must.True(t, fi.ModTime().Equal(old))
	b, _ := os.ReadFile(markerPath)
	must.Eq(t, "keep", string(b))
}

func TestManager_PluginSurvivesRestart(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()

	m1 := e.manager()
	st, err := m1.Create(ctx5(t), "plug", e.req("vol-ext", "data"))
	must.NoError(t, err)
	must.Eq(t, filepath.Join(e.volumes, "vol-ext"), st.Path)
	must.Eq(t, 1, e.lines("creates"))

	m2 := e.manager()
	m2.Restore(ctx5(t))
	must.Eq(t, 2, e.lines("creates"), must.Sprint("restore runs create exactly once"))
	// Restore passes the persisted volume ID and request, not a rebuilt one.
	b, err := os.ReadFile(filepath.Join(e.scratch, "requests"))
	must.NoError(t, err)
	want := `vol-ext|ns-synthetic|data|` + testNode + `|1|2|{"k":"v"}`
	must.Eq(t, []string{want, want}, strings.Split(strings.TrimSpace(string(b)), "\n"))
	st2 := mustStatus(t, m2, "vol-ext")
	must.Eq(t, hvstate.StatusReady, st2.Status)
	must.Eq(t, st.Path, st2.Path)
	must.Eq(t, "vol-ext", e.published()["data"].ID)
	ents, err := os.ReadDir(e.volumes)
	must.NoError(t, err)
	must.Len(t, 1, ents)
}

func TestManager_BuiltinSurvivesRestartAndPathChange(t *testing.T) {
	if !BuiltinSupported() {
		t.Skip("built-in plugin is not supported here")
	}
	e := newMgrEnv(t)

	m1 := e.manager()
	req := e.req("vol-builtin", "data")
	req.Parameters = nil
	st, err := m1.Create(ctx5(t), BuiltinMkdirID, req)
	must.NoError(t, err)
	must.Eq(t, filepath.Join(e.volumes, "vol-builtin"), st.Path)

	m2 := e.manager()
	m2.Restore(ctx5(t))
	st2 := mustStatus(t, m2, "vol-builtin")
	must.Eq(t, hvstate.StatusReady, st2.Status)
	must.Eq(t, st.Path, st2.Path)
	ents, _ := os.ReadDir(e.volumes)
	must.Len(t, 1, ents)

	// A different volumes directory must not create a second directory.
	other := t.TempDir()
	e.ops = &Ops{PluginDir: e.plugins, VolumesDir: other}
	m3 := e.manager()
	m3.Restore(ctx5(t))
	st3 := mustStatus(t, m3, "vol-builtin")
	must.Eq(t, hvstate.ReasonPathChanged, st3.Reason)
	must.Eq(t, st.Path, st3.Path)
	ents, _ = os.ReadDir(other)
	must.Len(t, 0, ents)
}

func TestManager_UnavailableCases(t *testing.T) {

	type tc struct {
		name   string
		reason string
		setup  func(e *mgrEnv) // seeds records and conditions; the subject is "vol-b"
	}
	cases := []tc{
		{"missing path", hvstate.ReasonMissingPath, func(e *mgrEnv) {
			e.seed(e.registeredRec("vol-b", "data", filepath.Join(t.TempDir(), "gone")))
		}},
		{"plugin missing", hvstate.ReasonPluginUnavailable, func(e *mgrEnv) {
			e.seed(e.pluginRec("vol-b", "data"))
			must.NoError(t, os.Remove(filepath.Join(e.plugins, "plug")))
		}},
		{"plugin unhealthy", hvstate.ReasonPluginUnavailable, func(e *mgrEnv) {
			e.seed(e.pluginRec("vol-b", "data"))
			e.flag("unhealthy", true)
		}},
		{"create fails", hvstate.ReasonRestoreFailed, func(e *mgrEnv) {
			e.seed(e.pluginRec("vol-b", "data"))
			e.flag("fail", true)
		}},
		{"different path", hvstate.ReasonPathChanged, func(e *mgrEnv) {
			e.seed(e.pluginRec("vol-b", "data"))
			other := filepath.Join(t.TempDir(), "elsewhere")
			must.NoError(t, os.Mkdir(other, 0o700))
			must.NoError(t, os.WriteFile(filepath.Join(e.scratch, "otherpath"), []byte(other), 0o600))
		}},
		{"other node", hvstate.ReasonNodeMismatch, func(e *mgrEnv) {
			dir := t.TempDir()
			r := e.registeredRec("vol-b", "data", dir)
			r.NodeID = otherTestNode
			e.seed(r)
		}},
		{"static name", hvstate.ReasonNameConflict, func(e *mgrEnv) {
			e.seed(e.registeredRec("vol-b", "data", t.TempDir()))
			e.setStatic("data", true)
		}},
		{"two dynamic records", hvstate.ReasonNameConflict, func(e *mgrEnv) {
			e.seed(e.registeredRec("vol-a", "data", t.TempDir()))
			e.seed(e.registeredRec("vol-b", "data", t.TempDir()))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newMgrEnv(t)
			e.installPlugin()
			c.setup(e)

			m := e.manager()
			m.Restore(ctx5(t))
			st := mustStatus(t, m, "vol-b")
			must.Eq(t, hvstate.StatusUnavailable, st.Status)
			must.Eq(t, c.reason, st.Reason)
			must.NotEq(t, "", st.Message)
			must.True(t, len(st.Message) <= hvstate.MaxMessageLen)
			must.False(t, st.CheckedAt.IsZero())
			for _, v := range e.published() {
				must.NotEq(t, "vol-b", v.ID)
			}

			// The diagnostic is stored and still there after another restart.
			rec := e.stored("vol-b")
			must.Eq(t, c.reason, rec.Reason)
			must.Eq(t, hvstate.StatusUnavailable, rec.Status)
			m2 := e.manager()
			m2.Restore(ctx5(t))
			st2 := mustStatus(t, m2, "vol-b")
			must.Eq(t, c.reason, st2.Reason)
			must.Eq(t, st.Message, st2.Message)
		})
	}
}

func TestManager_DynamicNameConflictLowestIDWins(t *testing.T) {
	e := newMgrEnv(t)
	e.seed(e.registeredRec("vol-b", "data", t.TempDir()))
	a := e.registeredRec("vol-a", "data", t.TempDir())
	a.Namespace = "another-namespace"
	e.seed(a)

	m := e.manager()
	m.Restore(ctx5(t))
	must.Eq(t, hvstate.StatusReady, mustStatus(t, m, "vol-a").Status)
	must.Eq(t, hvstate.ReasonNameConflict, mustStatus(t, m, "vol-b").Reason)
	must.Eq(t, "vol-a", e.published()["data"].ID)
	must.Eq(t, 1, len(e.published()))
}

func TestManager_StaticVolumesNeverReplaced(t *testing.T) {
	e := newMgrEnv(t)
	e.setStatic("data", true)
	e.seed(e.registeredRec("vol-a", "data", t.TempDir()))
	e.seed(e.registeredRec("vol-b", "other", t.TempDir()))
	m := e.manager()
	m.Restore(ctx5(t))
	pub := e.published()
	must.MapLen(t, 1, pub)
	must.Eq(t, "vol-b", pub["other"].ID)
}

func TestManager_Recovery(t *testing.T) {

	t.Run("path recreated", func(t *testing.T) {
		e := newMgrEnv(t)
		path := filepath.Join(t.TempDir(), "later")
		e.seed(e.registeredRec("vol-a", "data", path))
		m := e.manager()
		m.Restore(ctx5(t))
		must.Eq(t, hvstate.ReasonMissingPath, mustStatus(t, m, "vol-a").Reason)
		must.Eq(t, 0, e.pubCount())

		m.reconcile(ctx5(t), false)
		must.Eq(t, 0, e.pubCount())

		must.NoError(t, os.Mkdir(path, 0o700))
		m.reconcile(ctx5(t), false)
		st := mustStatus(t, m, "vol-a")
		must.Eq(t, hvstate.StatusReady, st.Status)
		must.Eq(t, "", st.Reason)
		must.Eq(t, 1, e.pubCount())
		must.Eq(t, "vol-a", e.published()["data"].ID)
		must.Eq(t, hvstate.StatusReady, e.stored("vol-a").Status)

		m.reconcile(ctx5(t), false)
		m.reconcile(ctx5(t), false)
		must.Eq(t, 1, e.pubCount(), must.Sprint("a pass without change must not publish"))

		// A ready volume whose path disappears becomes unavailable and create
		// is not run.
		must.NoError(t, os.Remove(path))
		m.reconcile(ctx5(t), false)
		must.Eq(t, hvstate.ReasonMissingPath, mustStatus(t, m, "vol-a").Reason)
		must.Eq(t, 2, e.pubCount())
		must.MapLen(t, 0, e.published())
	})

	t.Run("plugin returns", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		e.seed(e.pluginRec("vol-a", "data"))
		e.flag("unhealthy", true)
		m := e.manager()
		m.Restore(ctx5(t))
		must.Eq(t, hvstate.ReasonPluginUnavailable, mustStatus(t, m, "vol-a").Reason)
		must.Eq(t, 0, e.lines("creates"))

		e.flag("unhealthy", false)
		m.reconcile(ctx5(t), false)
		st := mustStatus(t, m, "vol-a")
		must.Eq(t, hvstate.StatusReady, st.Status)
		must.Eq(t, filepath.Join(e.volumes, "vol-a"), st.Path)
		must.Eq(t, 1, e.pubCount())
		must.Eq(t, 1, e.lines("creates"))

		// Ready volumes are not created again.
		m.reconcile(ctx5(t), false)
		m.reconcile(ctx5(t), false)
		must.Eq(t, 1, e.lines("creates"))
		must.Eq(t, 1, e.pubCount())

		// Its directory disappears: unavailable, and create still not re-run
		// in that pass.
		must.NoError(t, os.RemoveAll(filepath.Join(e.volumes, "vol-a")))
		m.reconcile(ctx5(t), false)
		must.Eq(t, hvstate.ReasonMissingPath, mustStatus(t, m, "vol-a").Reason)
		must.Eq(t, 1, e.lines("creates"))
		// The next pass retries create and the volume returns.
		m.reconcile(ctx5(t), false)
		must.Eq(t, hvstate.StatusReady, mustStatus(t, m, "vol-a").Status)
		must.Eq(t, 2, e.lines("creates"))
	})

	t.Run("plugin installed later", func(t *testing.T) {
		e := newMgrEnv(t)
		e.seed(e.pluginRec("vol-a", "data"))
		m := e.manager()
		m.Restore(ctx5(t))
		must.Eq(t, hvstate.ReasonPluginUnavailable, mustStatus(t, m, "vol-a").Reason)
		must.Eq(t, 0, e.pubCount())

		e.installPlugin()
		m.reconcile(ctx5(t), false)
		st := mustStatus(t, m, "vol-a")
		must.Eq(t, hvstate.StatusReady, st.Status)
		must.Eq(t, "vol-a", e.published()["data"].ID)
		must.Eq(t, 1, e.pubCount())
		must.Eq(t, 1, e.lines("creates"))
	})

	t.Run("name conflict clears", func(t *testing.T) {
		e := newMgrEnv(t)
		e.setStatic("data", true)
		e.seed(e.registeredRec("vol-a", "data", t.TempDir()))
		m := e.manager()
		m.Restore(ctx5(t))
		must.Eq(t, hvstate.ReasonNameConflict, mustStatus(t, m, "vol-a").Reason)
		m.reconcile(ctx5(t), false)
		must.Eq(t, 0, e.pubCount())
		e.setStatic("data", false)
		m.reconcile(ctx5(t), false)
		must.Eq(t, hvstate.StatusReady, mustStatus(t, m, "vol-a").Status)
		must.Eq(t, 1, e.pubCount())
	})
}

func TestManager_PublishedEntries(t *testing.T) {
	e := newMgrEnv(t)
	dir := t.TempDir()
	e.seed(e.registeredRec("vol-a", "data", dir))
	foreign := e.registeredRec("vol-z", "foreign", t.TempDir())
	foreign.NodeID = otherTestNode
	e.seed(foreign)

	m := e.manager()
	m.Restore(ctx5(t))
	pub := e.published()
	must.MapLen(t, 1, pub)
	must.Eq(t, structs.ClientHostVolumeConfig{Name: "data", Path: dir, ReadOnly: false, ID: "vol-a"}, *pub["data"])
	for _, v := range pub {
		must.NotEq(t, foreign.Path, v.Path)
	}
}

func TestManager_CreateAndDelete(t *testing.T) {

	t.Run("create persists and publishes", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		rec := e.stored("vol-a")
		must.NotNil(t, rec)
		must.Eq(t, hvstate.StatusReady, rec.Status)
		must.Eq(t, "plug", rec.PluginID)
		must.Eq(t, "v", rec.Parameters["k"])
		must.Eq(t, "vol-a", e.published()["data"].ID)
	})

	t.Run("failed create stores nothing", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		e.flag("fail", true)
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		asOpError(t, err, OperationCreate, ReasonNonZeroExit)
		must.Nil(t, e.stored("vol-a"))
		must.Eq(t, 0, e.pubCount())
		_, ok := m.Status("vol-a")
		must.False(t, ok)
	})

	t.Run("name in use", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		e.setStatic("data", true)
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		asOpError(t, err, OperationCreate, hvstate.ReasonNameConflict)
		must.Eq(t, 0, e.lines("creates"))
	})

	t.Run("persist failure rolls back", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		e.store.setPutError(errors.New("disk full"))
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.Error(t, err)
		must.Eq(t, 1, e.lines("deletes"))
		must.Eq(t, 0, e.pubCount())
		_, statErr := os.Stat(filepath.Join(e.volumes, "vol-a"))
		must.True(t, errors.Is(statErr, os.ErrNotExist))
	})

	t.Run("delete removes and unpublishes", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		must.NoError(t, m.Delete(ctx5(t), "vol-a"))
		must.Nil(t, e.stored("vol-a"))
		must.MapLen(t, 0, e.published())
		_, ok := m.Status("vol-a")
		must.False(t, ok)
		must.Eq(t, 1, e.lines("deletes"))
	})

	t.Run("failed delete keeps the record", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		m := e.manager()
		_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		must.NoError(t, err)
		e.flag("faildelete", true)
		err = m.Delete(ctx5(t), "vol-a")
		asOpError(t, err, OperationDelete, ReasonNonZeroExit)
		must.NotNil(t, e.stored("vol-a"))
		must.Eq(t, hvstate.StatusReady, mustStatus(t, m, "vol-a").Status)
		must.Eq(t, "vol-a", e.published()["data"].ID)
	})

	t.Run("registered record is not deleted", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		dir := t.TempDir()
		must.NoError(t, os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o600))
		m := e.manager()
		_, err := m.AddRegistered(ctx5(t), RegisterRequest{VolumeID: "vol-r", Name: "reg", Path: dir})
		must.NoError(t, err)
		err = m.Delete(ctx5(t), "vol-r")
		must.True(t, errors.Is(err, ErrRegisteredVolume))
		must.NotNil(t, e.stored("vol-r"))
		must.Eq(t, 0, e.lines("deletes"))
		_, statErr := os.Stat(filepath.Join(dir, "f"))
		must.NoError(t, statErr)
		must.Eq(t, "vol-r", e.published()["reg"].ID)
	})

	t.Run("registered path must exist", func(t *testing.T) {
		e := newMgrEnv(t)
		m := e.manager()
		_, err := m.AddRegistered(ctx5(t), RegisterRequest{VolumeID: "vol-r", Name: "reg", Path: filepath.Join(t.TempDir(), "gone")})
		asOpError(t, err, "register", hvstate.ReasonMissingPath)
		must.Nil(t, e.stored("vol-r"))
	})
}

func TestManager_ConcurrentOperationsNeverOverlap(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	e.flag("slow", true)
	m := e.manager()
	_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
	must.NoError(t, err)

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _ = m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
		}()
		go func() {
			defer wg.Done()
			m.reconcile(ctx5(t), true)
		}()
	}
	wg.Wait()
	must.Eq(t, 0, e.lines("overlap"), must.Sprint("two plugin operations ran at once"))
	must.Eq(t, 5, e.lines("creates"), must.Sprint("one create per Create call"))
}

func TestManager_DeleteWaitingPassDoesNotRecreate(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	m := e.manager()
	_, err := m.Create(ctx5(t), "plug", e.req("vol-a", "data"))
	must.NoError(t, err)
	before := e.lines("creates")

	// Hold the volume lock, start a pass that must wait, delete, release.
	must.NoError(t, m.locks.acquire(ctx5(t), "vol-a"))
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.reconcile(ctx5(t), true)
	}()
	eventually(t, func() bool { return m.locks.refs("vol-a") == 2 }, 5*time.Second)
	m.mu.Lock()
	must.NoError(t, m.cfg.Store.DeleteHostVolume("vol-a"))
	delete(m.entries, "vol-a")
	m.mu.Unlock()
	m.locks.release("vol-a")
	<-done
	must.Eq(t, before, e.lines("creates"))
	_, ok := m.Status("vol-a")
	must.False(t, ok)
}

func TestManager_Lifecycle(t *testing.T) {

	t.Run("run stops on cancel", func(t *testing.T) {
		e := newMgrEnv(t)
		dir := filepath.Join(t.TempDir(), "d")
		must.NoError(t, os.Mkdir(dir, 0o700))
		e.seed(e.registeredRec("vol-a", "data", dir))
		m := e.manager(func(c *ManagerConfig) { c.Interval = 5 * time.Millisecond })
		m.Restore(ctx5(t))
		must.Eq(t, 1, e.pubCount())

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			m.Run(ctx)
		}()
		must.NoError(t, os.Remove(dir))
		eventually(t, func() bool {
			st, _ := m.Status("vol-a")
			return st.Reason == hvstate.ReasonMissingPath
		}, 5*time.Second)
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Run did not return after cancel")
		}
	})

	t.Run("restore is bounded", func(t *testing.T) {
		e := newMgrEnv(t)
		e.installPlugin()
		e.seed(e.pluginRec("vol-a", "data"))
		e.flag("sleep", true)
		m := e.manager(func(c *ManagerConfig) { c.RestoreTimeout = 1500 * time.Millisecond })

		done := make(chan struct{})
		go func() {
			defer close(done)
			m.Restore(ctx5(t))
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("restore did not stop at its deadline")
		}
		st := mustStatus(t, m, "vol-a")
		must.Eq(t, hvstate.StatusUnavailable, st.Status)
		must.Eq(t, hvstate.ReasonRestoreFailed, st.Reason)

		// The plugin process is gone, not just ignored.
		b, err := os.ReadFile(filepath.Join(e.scratch, "pid"))
		must.NoError(t, err)
		pid := 0
		for _, c := range strings.TrimSpace(string(b)) {
			pid = pid*10 + int(c-'0')
		}
		eventually(t, func() bool { return errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) }, 5*time.Second)
		must.MapLen(t, 0, e.published())
	})

	t.Run("canceled restore publishes nothing", func(t *testing.T) {
		e := newMgrEnv(t)
		e.seed(e.registeredRec("vol-a", "data", t.TempDir()))
		m := e.manager()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		m.Restore(ctx)
		must.Eq(t, 0, e.pubCount())
	})
}

func TestManager_InvalidRecordsAreSkipped(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	good := e.registeredRec("vol-good", "good", t.TempDir())
	e.seed(good)

	badPath := e.registeredRec("vol-badpath", "badpath", "relative/path")
	badPlugin := e.pluginRec("vol-badplugin", "badplugin")
	badPlugin.PluginID = "../escape"
	badKind := e.registeredRec("vol-badkind", "badkind", t.TempDir())
	badKind.Kind = "weird"
	badStatus := e.registeredRec("vol-badstatus", "badstatus", t.TempDir())
	badStatus.Status = "weird"
	for _, r := range []*hvstate.Record{badPath, badPlugin, badKind, badStatus} {
		e.seed(r)
	}

	m := e.manager()
	m.Restore(ctx5(t))
	must.Len(t, 1, m.Statuses())
	must.Eq(t, "vol-good", e.published()["good"].ID)
	must.StrContains(t, e.logText(), "skipping invalid host volume record")
}

// unreadableStore reports one extra key whose value could not be decoded.
type unreadableStore struct {
	Store
	id string
}

func (u unreadableStore) GetHostVolumes() ([]*hvstate.Record, map[string]error, error) {
	recs, errs, err := u.Store.GetHostVolumes()
	if errs == nil {
		errs = map[string]error{}
	}
	errs[u.id] = errors.New("synthetic decode failure")
	return recs, errs, err
}

func TestManager_UnreadableRecordIsLoggedAndSkipped(t *testing.T) {
	e := newMgrEnv(t)
	e.seed(e.registeredRec("vol-good", "good", t.TempDir()))
	m := e.manager(func(c *ManagerConfig) { c.Store = unreadableStore{Store: e.store, id: "vol-corrupt"} })
	m.Restore(ctx5(t))

	must.Len(t, 1, m.Statuses())
	must.Eq(t, "vol-good", e.published()["good"].ID)
	logs := e.logText()
	must.StrContains(t, logs, "skipping unreadable host volume record")
	must.StrContains(t, logs, "vol-corrupt")
	must.StrNotContains(t, logs, "synthetic decode failure")
}

// A plugin that returns a different path is not asked again until the client
// restarts: every retry is another create.
func TestManager_PathChangedIsRetriedOnlyAtRestart(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	e.seed(e.pluginRec("vol-a", "data"))
	other := filepath.Join(t.TempDir(), "elsewhere")
	must.NoError(t, os.Mkdir(other, 0o700))
	otherpath := filepath.Join(e.scratch, "otherpath")
	must.NoError(t, os.WriteFile(otherpath, []byte(other), 0o600))

	m := e.manager()
	m.Restore(ctx5(t))
	must.Eq(t, hvstate.ReasonPathChanged, mustStatus(t, m, "vol-a").Reason)
	must.Eq(t, 1, e.lines("creates"))

	// The plugin behaves again, but periodic passes leave the volume alone.
	must.NoError(t, os.Remove(otherpath))
	m.reconcile(ctx5(t), false)
	m.reconcile(ctx5(t), false)
	st := mustStatus(t, m, "vol-a")
	must.Eq(t, hvstate.ReasonPathChanged, st.Reason)
	must.NotEq(t, "", st.Message)
	must.Eq(t, 1, e.lines("creates"))
	must.Eq(t, 0, e.pubCount())

	m2 := e.manager()
	m2.Restore(ctx5(t))
	must.Eq(t, hvstate.StatusReady, mustStatus(t, m2, "vol-a").Status)
	must.Eq(t, 2, e.lines("creates"))
	must.Eq(t, "vol-a", e.published()["data"].ID)
}

// Restore runs in the background, so an entry point can run first. It must
// see the stored records, and its own volume must be neither lost nor
// created a second time by the restore pass.
func TestManager_CreateBeforeRestore(t *testing.T) {
	e := newMgrEnv(t)
	e.installPlugin()
	e.seed(e.pluginRec("vol-old", "old"))

	m := e.manager()
	_, err := m.Create(ctx5(t), "plug", e.req("vol-new", "old"))
	var oe *OpError
	must.True(t, errors.As(err, &oe))
	must.Eq(t, hvstate.ReasonNameConflict, oe.Reason, must.Sprint("the stored record's name is already known"))

	_, err = m.Create(ctx5(t), "plug", e.req("vol-new", "new"))
	must.NoError(t, err)
	must.Eq(t, 1, e.lines("creates"))

	m.Restore(ctx5(t))
	must.Eq(t, hvstate.StatusReady, mustStatus(t, m, "vol-new").Status)
	must.Eq(t, hvstate.StatusReady, mustStatus(t, m, "vol-old").Status)
	must.Eq(t, 2, e.lines("creates"), must.Sprint("restore creates only the stored volume"))
	must.MapLen(t, 2, e.published())
}

func TestManager_ParameterValuesNeverLeak(t *testing.T) {
	const sentinel = "SENTINEL-param-9d41c7"
	e := newMgrEnv(t)
	body := `case "$1" in
fingerprint) echo '{"version":"1.0.0"}' ;;
create) echo '{"error":"cannot use ` + sentinel + ` here"}'; exit 1 ;;
esac`
	writePlugin(t, e.plugins, "plug", body, 0o755)

	req := e.req("vol-a", "data")
	req.Parameters = map[string]string{"token": sentinel}
	m := e.manager()
	_, err := m.Create(ctx5(t), "plug", req)
	must.Error(t, err)
	must.StrNotContains(t, err.Error(), sentinel)

	rec := e.pluginRec("vol-b", "other")
	rec.Parameters = map[string]string{"token": sentinel}
	e.seed(rec)
	m2 := e.manager()
	m2.Restore(ctx5(t))
	st := mustStatus(t, m2, "vol-b")
	must.Eq(t, hvstate.ReasonRestoreFailed, st.Reason)
	must.StrContains(t, st.Message, "[redacted]")
	must.StrNotContains(t, st.Message, sentinel)

	stored := e.stored("vol-b")
	must.StrNotContains(t, stored.Message, sentinel)
	must.StrNotContains(t, stored.Reason, sentinel)
	must.StrNotContains(t, e.logText(), sentinel)

	// A parameter value that is also a plugin error cut by the bound is
	// redacted before it is cut.
	long := strings.Repeat("x", hvstate.MaxMessageLen-3) + sentinel
	must.StrNotContains(t, boundMessage("failed: "+long, map[string]string{"token": long}), "xxxx")
}

func TestManager_ConcurrentStatusReads(t *testing.T) {
	e := newMgrEnv(t)
	e.seed(e.registeredRec("vol-a", "data", t.TempDir()))
	m := e.manager()
	m.Restore(ctx5(t))

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range 50 {
				m.reconcile(ctx5(t), false)
			}
		}()
		go func() {
			defer wg.Done()
			for range 50 {
				ss := m.Statuses()
				if len(ss) == 1 {
					ss[0].Name = "mutated"
				}
				_, _ = m.Status("vol-a")
			}
		}()
	}
	wg.Wait()
	must.Eq(t, "data", mustStatus(t, m, "vol-a").Name)
}

func TestNewManager_RequiredFields(t *testing.T) {
	e := newMgrEnv(t)
	ok := ManagerConfig{Store: e.store, Ops: e.ops, NodeID: "n", Publish: func(map[string]*structs.ClientHostVolumeConfig) {}}
	_, err := NewManager(ok)
	must.NoError(t, err)
	for name, mod := range map[string]func(*ManagerConfig){
		"store":   func(c *ManagerConfig) { c.Store = nil },
		"ops":     func(c *ManagerConfig) { c.Ops = nil },
		"node id": func(c *ManagerConfig) { c.NodeID = "" },
		"publish": func(c *ManagerConfig) { c.Publish = nil },
	} {
		c := ok
		mod(&c)
		_, err := NewManager(c)
		must.Error(t, err, must.Sprint(name))
	}
}

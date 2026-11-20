// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package hostvolume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/shoenig/test/must"
)

func bReq(id string, params map[string]string) OpRequest {
	return OpRequest{VolumeID: id, Namespace: "ns", Name: "data", NodeID: "n1", Parameters: params}
}

func modeOf(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Lstat(p)
	must.NoError(t, err)
	return fi.Mode()
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	must.NoError(t, err)
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// marker makes a directory outside the managed root with a marker file and
// returns the directory and the marker path.
func marker(t *testing.T, dir string) (string, string) {
	t.Helper()
	d := filepath.Join(dir, "outside")
	must.NoError(t, os.Mkdir(d, 0o750))
	m := filepath.Join(d, "marker")
	must.NoError(t, os.WriteFile(m, []byte("keep"), 0o600))
	return d, m
}

func TestBuiltinVersionParses(t *testing.T) {
	v, err := ParseVersion([]byte(`{"version":"` + BuiltinMkdirVersion + `"}`))
	must.NoError(t, err)
	must.Eq(t, BuiltinMkdirVersion, v)
	must.True(t, BuiltinSupported())
}

func TestBuiltin_InvalidIDs(t *testing.T) {
	hostile := []string{"a/b", "..", "/abs", "a\x00b", "", ".", ".hidden", "UPPER", "a\\b", "a b", "../x", strings.Repeat("a", 65), "é"}
	for _, id := range hostile {
		for _, op := range []string{"create", "delete"} {
			t.Run(op+"/"+strings.ToValidUTF8(id, "?"), func(t *testing.T) {
				e := newOpEnv(t)
				_, mk := marker(t, e.scratch)
				var err error
				if op == "create" {
					_, err = e.ops.CreateBuiltin(context.Background(), bReq(id, nil))
				} else {
					err = e.ops.DeleteBuiltin(context.Background(), bReq(id, nil))
				}
				asOpError(t, err, op, ReasonInvalidInput)
				must.SliceEmpty(t, entries(t, e.volumes))
				_, serr := os.Stat(mk)
				must.NoError(t, serr)
			})
		}
	}
}

func TestBuiltin_CreateBasics(t *testing.T) {
	e := newOpEnv(t)
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("vol-1", nil))
	must.NoError(t, err)
	must.Eq(t, CreateResult{Path: filepath.Join(e.volumes, "vol-1"), Bytes: 0}, res)
	must.Eq(t, []string{"vol-1"}, entries(t, e.volumes))
	m := modeOf(t, res.Path)
	must.True(t, m.IsDir())
	must.Eq(t, os.FileMode(0o700), m.Perm())
	must.Eq(t, os.FileMode(0), m&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky))
}

func TestBuiltin_CapacityIgnored(t *testing.T) {
	e := newOpEnv(t)
	req := bReq("vol-1", map[string]string{"unknown": "x"})
	req.CapacityMinBytes, req.CapacityMaxBytes = 100, 200
	res, err := e.ops.CreateBuiltin(context.Background(), req)
	must.NoError(t, err)
	must.Eq(t, int64(0), res.Bytes)
}

func TestBuiltin_ManagedRootCreated(t *testing.T) {
	e := newOpEnv(t)
	e.ops.VolumesDir = filepath.Join(e.volumes, "a", "host_volumes")
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	must.NoError(t, err)
	must.True(t, modeOf(t, res.Path).IsDir())
}

// Not parallel: it changes the process umask.
func TestBuiltin_Mode(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want os.FileMode
	}{
		{"0755", 0o755}, {"755", 0o755}, {"0750", 0o750}, {"0777", 0o777}, {"0700", 0o700}, {"0500", 0o500},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			old := syscall.Umask(0o077) // the explicit chmod must beat the umask
			defer syscall.Umask(old)
			e := newOpEnv(t)
			res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": tc.mode}))
			must.NoError(t, err)
			must.Eq(t, tc.want, modeOf(t, res.Path).Perm())
			must.NoError(t, os.Chmod(res.Path, 0o700)) // let TempDir clean up
		})
	}
	for _, bad := range []string{"", "8", "0o755", "+755", "-755", " 755", "755 ", "07555", "rwx", "4755", "2755", "1755", "0x1ed", "1.0", "\x00"} {
		t.Run("bad/"+strings.ToValidUTF8(bad, "?"), func(t *testing.T) {
			e := newOpEnv(t)
			_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": bad}))
			asOpError(t, err, "create", ReasonInvalidInput)
			must.SliceEmpty(t, entries(t, e.volumes))
		})
	}
}

func TestBuiltin_UIDGIDValidation(t *testing.T) {
	for _, key := range []string{"uid", "gid"} {
		for _, bad := range []string{"", "-1", "+1", "1.5", "abc", " 1", "1 ", "2147483648", "4294967295", "4294967296", "99999999999999999999", "0x10", "1e3"} {
			t.Run(key+"/"+bad, func(t *testing.T) {
				e := newOpEnv(t)
				_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{key: bad}))
				asOpError(t, err, "create", ReasonInvalidInput)
				must.SliceEmpty(t, entries(t, e.volumes))
			})
		}
	}
}

func TestBuiltin_ChownCurrentIDs(t *testing.T) {
	e := newOpEnv(t)
	uid, gid := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"uid": uid, "gid": gid}))
	must.NoError(t, err)
	st := statOf(t, res.Path)
	must.Eq(t, uint32(os.Getuid()), st.Uid)
	must.Eq(t, uint32(os.Getgid()), st.Gid)
}

func TestBuiltin_ChownOtherUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root")
	}
	e := newOpEnv(t)
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"uid": "12345", "gid": "23456"}))
	must.NoError(t, err)
	st := statOf(t, res.Path)
	must.Eq(t, uint32(12345), st.Uid)
	must.Eq(t, uint32(23456), st.Gid)
}

// Not parallel: it swaps the package-level applyOwner.
func TestBuiltin_ChownFailureRemovesNewDirectory(t *testing.T) {
	old := applyOwner
	defer func() { applyOwner = old }()
	applyOwner = func(*os.File, int, int) error { return errors.New("boom " + secretValue) }

	e := newOpEnv(t)
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"uid": "1"}))
	oe := asOpError(t, err, "create", ReasonFilesystem)
	must.Eq(t, CreateResult{}, res)
	must.StrNotContains(t, err.Error(), secretValue)
	must.StrNotContains(t, oe.Message, "boom")
	must.SliceEmpty(t, entries(t, e.volumes))

	// an existing directory is left in place, with its mode
	applyOwner = old
	_, err = e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0750"}))
	must.NoError(t, err)
	applyOwner = func(*os.File, int, int) error { return errors.New("boom") }
	_, err = e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0700"}))
	asOpError(t, err, "create", ReasonFilesystem)
	must.Eq(t, os.FileMode(0o750), modeOf(t, filepath.Join(e.volumes, "v")).Perm())
}

// Not parallel: it swaps the package-level applyMode.
func TestBuiltin_ModeFailureRemovesNewDirectory(t *testing.T) {
	old := applyMode
	defer func() { applyMode = old }()
	applyMode = func(*os.File, os.FileMode) error { return errors.New("boom " + secretValue) }

	e := newOpEnv(t)
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0755"}))
	oe := asOpError(t, err, "create", ReasonFilesystem)
	must.Eq(t, CreateResult{}, res)
	must.StrNotContains(t, err.Error(), secretValue)
	must.StrNotContains(t, oe.Message, "boom")
	must.SliceEmpty(t, entries(t, e.volumes))

	// an existing directory is left in place, with its mode
	applyMode = old
	_, err = e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0750"}))
	must.NoError(t, err)
	applyMode = func(*os.File, os.FileMode) error { return errors.New("boom") }
	_, err = e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0700"}))
	asOpError(t, err, "create", ReasonFilesystem)
	must.Eq(t, os.FileMode(0o750), modeOf(t, filepath.Join(e.volumes, "v")).Perm())
}

func TestBuiltin_ChownFailsNaturallyWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can chown anywhere")
	}
	e := newOpEnv(t)
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"uid": "0", "gid": "0"}))
	asOpError(t, err, "create", ReasonFilesystem)
	must.SliceEmpty(t, entries(t, e.volumes))
}

func TestBuiltin_Idempotent(t *testing.T) {
	e := newOpEnv(t)
	r1, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0750"}))
	must.NoError(t, err)
	fi1, _ := os.Stat(r1.Path)
	must.NoError(t, os.WriteFile(filepath.Join(r1.Path, "data"), []byte("x"), 0o600))
	r2, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0755"}))
	must.NoError(t, err)
	fi2, _ := os.Stat(r2.Path)
	must.Eq(t, r1, r2)
	must.True(t, os.SameFile(fi1, fi2))
	must.Eq(t, os.FileMode(0o755), modeOf(t, r2.Path).Perm())
	must.Eq(t, []string{"v"}, entries(t, e.volumes))
	must.Eq(t, []string{"data"}, entries(t, r2.Path))
}

func TestBuiltin_RootIsSymlink(t *testing.T) {
	e := newOpEnv(t)
	real := filepath.Join(e.scratch, "real")
	must.NoError(t, os.Mkdir(real, 0o700))
	link := filepath.Join(e.scratch, "link")
	must.NoError(t, os.Symlink(real, link))
	e.ops.VolumesDir = link
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	asOpError(t, err, "create", ReasonInvalidPath)
	asOpError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)), "delete", ReasonInvalidPath)
	must.SliceEmpty(t, entries(t, real))
}

func TestBuiltin_RootNotDirectory(t *testing.T) {
	e := newOpEnv(t)
	f := filepath.Join(e.scratch, "file")
	must.NoError(t, os.WriteFile(f, nil, 0o600))
	e.ops.VolumesDir = f
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	asOpError(t, err, "create", ReasonInvalidPath)
}

func TestBuiltin_VolumesDirInvalid(t *testing.T) {
	for _, d := range []string{"", "relative/dir", "/a/../b"} {
		o := &Ops{VolumesDir: d}
		_, err := o.CreateBuiltin(context.Background(), bReq("v", nil))
		asOpError(t, err, "create", ReasonInvalidInput)
	}
}

// TestBuiltin_RegularFileAtVolumeName proves a plain file where the volume
// directory belongs is rejected and left untouched.
func TestBuiltin_RegularFileAtVolumeName(t *testing.T) {
	e := newOpEnv(t)
	f := filepath.Join(e.volumes, "v")
	must.NoError(t, os.WriteFile(f, []byte("keep"), 0o600))
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0777"}))
	asOpError(t, err, "create", ReasonInvalidPath)
	raw, rerr := os.ReadFile(f)
	must.NoError(t, rerr)
	must.Eq(t, "keep", string(raw))
	must.Eq(t, os.FileMode(0o600), modeOf(t, f).Perm())
}

// The relative kinds matter on their own: a link with an absolute target is
// refused by the managed root itself, while a relative link that stays inside
// the root is followed by it and is stopped only by the checks in create.
func TestBuiltin_PlantedSymlinkAtVolumeName(t *testing.T) {
	for _, kind := range []string{"outside", "sibling", "file", "dangling", "relative sibling", "relative nested"} {
		t.Run(kind, func(t *testing.T) {
			e := newOpEnv(t)
			outside, mk := marker(t, e.scratch)
			var target, link string // link is the text of the symlink when it differs from target
			switch kind {
			case "outside":
				target = outside
			case "sibling":
				target = filepath.Join(e.volumes, "other")
				must.NoError(t, os.Mkdir(target, 0o750))
			case "file":
				target = mk
			case "dangling":
				target = filepath.Join(e.scratch, "nowhere")
			case "relative sibling":
				target, link = filepath.Join(e.volumes, "other"), "other"
				must.NoError(t, os.Mkdir(target, 0o750))
			case "relative nested":
				target, link = filepath.Join(e.volumes, "other", "sub"), "other/sub"
				must.NoError(t, os.MkdirAll(target, 0o750))
			}
			if link == "" {
				link = target
			}
			must.NoError(t, os.Symlink(link, filepath.Join(e.volumes, "v")))
			var before *syscall.Stat_t
			if kind != "dangling" {
				before = statOf(t, target)
			}
			_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{"mode": "0777"}))
			asOpError(t, err, "create", ReasonInvalidPath)
			if kind != "dangling" {
				after := statOf(t, target)
				must.Eq(t, before.Mode, after.Mode)
				must.Eq(t, before.Uid, after.Uid)
			}
			// still a symlink, and the target was not created or removed
			must.Eq(t, os.ModeSymlink, modeOf(t, filepath.Join(e.volumes, "v")).Type())
			if kind == "dangling" {
				_, serr := os.Lstat(target)
				must.True(t, errors.Is(serr, os.ErrNotExist))
			}
		})
	}
}

func TestBuiltin_DeleteRemovesOnlyVolume(t *testing.T) {
	e := newOpEnv(t)
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	must.NoError(t, err)
	_, err = e.ops.CreateBuiltin(context.Background(), bReq("w", nil))
	must.NoError(t, err)
	must.NoError(t, os.MkdirAll(filepath.Join(res.Path, "sub", "deep"), 0o700))
	must.NoError(t, os.WriteFile(filepath.Join(res.Path, "sub", "deep", "f"), []byte("x"), 0o600))

	must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
	must.Eq(t, []string{"w"}, entries(t, e.volumes))
	// already gone
	must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
}

func TestBuiltin_DeleteMissingRoot(t *testing.T) {
	e := newOpEnv(t)
	e.ops.VolumesDir = filepath.Join(e.volumes, "missing")
	must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
	_, err := os.Lstat(e.ops.VolumesDir)
	must.True(t, errors.Is(err, os.ErrNotExist)) // delete does not create the root
}

// A managed root that cannot be inspected is not a missing root: delete must
// not report success. The parent is a regular file, which fails for root too.
func TestBuiltin_RootInspectFailureIsNotMissing(t *testing.T) {
	e := newOpEnv(t)
	file := filepath.Join(e.scratch, "file")
	must.NoError(t, os.WriteFile(file, nil, 0o600))
	e.ops.VolumesDir = filepath.Join(file, "host_volumes")

	asOpError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)), "delete", ReasonFilesystem)
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	asOpError(t, err, "create", ReasonFilesystem)
}

// Not parallel: it swaps the package-level inspectVolume.
func TestBuiltin_DeleteInspectFailureIsNotMissing(t *testing.T) {
	old := inspectVolume
	defer func() { inspectVolume = old }()

	e := newOpEnv(t)
	res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	must.NoError(t, err)
	data := filepath.Join(res.Path, "data")
	must.NoError(t, os.WriteFile(data, []byte("x"), 0o600))

	inspectVolume = func(*os.Root, string) (os.FileInfo, error) { return nil, errors.New("boom " + secretValue) }
	err = e.ops.DeleteBuiltin(context.Background(), bReq("v", nil))
	oe := asOpError(t, err, "delete", ReasonFilesystem)
	must.StrNotContains(t, err.Error(), secretValue)
	must.StrNotContains(t, oe.Message, "boom")
	_, serr := os.Stat(data)
	must.NoError(t, serr)
}

// Not parallel: it swaps the package-level removeTree.
func TestBuiltin_DeleteRemoveFailure(t *testing.T) {
	old := removeTree
	defer func() { removeTree = old }()

	e := newOpEnv(t)
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	must.NoError(t, err)

	removeTree = func(*os.Root, string) error { return errors.New("boom " + secretValue) }
	err = e.ops.DeleteBuiltin(context.Background(), bReq("v", nil))
	oe := asOpError(t, err, "delete", ReasonFilesystem)
	must.StrNotContains(t, err.Error(), secretValue)
	must.StrNotContains(t, oe.Message, "boom")
	must.Eq(t, []string{"v"}, entries(t, e.volumes))

	// removed by something else in between: already gone is success
	removeTree = func(*os.Root, string) error { return os.ErrNotExist }
	must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
}

// Create and delete hold the volume lock for all of their filesystem work,
// not only while they validate. Not parallel: it swaps package-level steps.
func TestBuiltin_HoldsVolumeLockDuringWork(t *testing.T) {
	oldOwner, oldRemove := applyOwner, removeTree
	defer func() { applyOwner, removeTree = oldOwner, oldRemove }()

	e := newOpEnv(t)
	held := -1
	applyOwner = func(f *os.File, uid, gid int) error {
		held = e.ops.locks.refs("v")
		return oldOwner(f, uid, gid)
	}
	_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
	must.NoError(t, err)
	must.Eq(t, 1, held, must.Sprint("create released the volume lock before its filesystem work"))

	held = -1
	removeTree = func(root *os.Root, name string) error {
		held = e.ops.locks.refs("v")
		return oldRemove(root, name)
	}
	must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
	must.Eq(t, 1, held, must.Sprint("delete released the volume lock before its filesystem work"))
	must.Eq(t, 0, e.ops.locks.len())
}

// The volume directory is replaced by a FIFO after it was inspected. Opening
// a FIFO for reading waits for a writer, so create must refuse it at the open
// and never wait while it holds the volume lock. Not parallel: it swaps the
// package-level inspectVolume.
func TestBuiltin_FifoSwappedInAfterInspection(t *testing.T) {
	old := inspectVolume
	defer func() { inspectVolume = old }()

	e := newOpEnv(t)
	v := filepath.Join(e.volumes, "v")
	inspectVolume = func(root *os.Root, name string) (os.FileInfo, error) {
		fi, err := old(root, name)
		if err != nil {
			return nil, err
		}
		if err := os.Remove(v); err != nil {
			return nil, err
		}
		return fi, syscall.Mkfifo(v, 0o600)
	}

	done := make(chan error, 1)
	go func() {
		_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
		done <- err
	}()
	select {
	case err := <-done:
		asOpError(t, err, "create", ReasonInvalidPath)
	case <-time.After(5 * time.Second):
		// give the waiting open its writer so the goroutine can end
		if w, err := os.OpenFile(v, os.O_RDWR, 0); err == nil {
			defer w.Close()
		}
		<-done
		t.Fatal("create waited on a FIFO at the volume name")
	}
	must.Eq(t, 0, e.ops.locks.len())
}

func TestBuiltin_DeleteNeverFollowsSymlinks(t *testing.T) {
	t.Run("inside the volume", func(t *testing.T) {
		e := newOpEnv(t)
		res, err := e.ops.CreateBuiltin(context.Background(), bReq("v", nil))
		must.NoError(t, err)
		sibling := filepath.Join(e.volumes, "sibling")
		must.NoError(t, os.Mkdir(sibling, 0o700))
		smk := filepath.Join(sibling, "marker")
		must.NoError(t, os.WriteFile(smk, []byte("k"), 0o600))
		outside, omk := marker(t, e.scratch)
		must.NoError(t, os.Symlink(sibling, filepath.Join(res.Path, "to-sibling")))
		must.NoError(t, os.Symlink(outside, filepath.Join(res.Path, "to-outside")))

		must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
		for _, p := range []string{smk, omk} {
			_, err := os.Stat(p)
			must.NoError(t, err)
		}
		must.Eq(t, []string{"sibling"}, entries(t, e.volumes))
	})
	t.Run("at the volume name", func(t *testing.T) {
		e := newOpEnv(t)
		outside, omk := marker(t, e.scratch)
		must.NoError(t, os.Symlink(outside, filepath.Join(e.volumes, "v")))
		must.NoError(t, e.ops.DeleteBuiltin(context.Background(), bReq("v", nil)))
		_, err := os.Stat(omk)
		must.NoError(t, err)
		must.SliceEmpty(t, entries(t, e.volumes)) // the link itself is removed
	})
}

func TestBuiltin_DeleteHostileIDsKeepOutside(t *testing.T) {
	e := newOpEnv(t)
	_, omk := marker(t, e.scratch)
	rel, err := filepath.Rel(e.volumes, filepath.Dir(omk))
	must.NoError(t, err)
	for _, id := range []string{rel, filepath.Dir(omk), "..", ".", "", e.volumes, "../" + filepath.Base(e.volumes)} {
		err := e.ops.DeleteBuiltin(context.Background(), bReq(id, nil))
		asOpError(t, err, "delete", ReasonInvalidInput)
	}
	_, serr := os.Stat(omk)
	must.NoError(t, serr)
	_, serr = os.Stat(e.volumes)
	must.NoError(t, serr)
}

// A registered external directory is only reachable by path, and the
// built-in delete takes none: it only ever derives VolumesDir/<id>.
func TestBuiltin_DeleteDoesNotTouchExternalDirectory(t *testing.T) {
	e := newOpEnv(t)
	ext, emk := marker(t, e.scratch)
	req := bReq("v", map[string]string{"path": ext, "created_path": ext})
	must.NoError(t, e.ops.DeleteBuiltin(context.Background(), req))
	_, err := os.Stat(emk)
	must.NoError(t, err)
}

func TestBuiltin_ErrorsLeakNoParameterValue(t *testing.T) {
	e := newOpEnv(t)
	for _, k := range []string{"mode", "uid", "gid"} {
		_, err := e.ops.CreateBuiltin(context.Background(), bReq("v", map[string]string{k: secretValue}))
		asOpError(t, err, "create", ReasonInvalidInput)
		must.StrNotContains(t, err.Error(), secretValue)
	}
}

func TestBuiltin_SharesLockWithExternal(t *testing.T) {
	e := newOpEnv(t)
	gate := filepath.Join(e.scratch, "gate")
	c := e.plugin(t, `while [ ! -e '`+gate+`' ]; do sleep 0.02; done; echo '{"path":"/srv/x"}'`)

	extDone := make(chan error, 1)
	go func() {
		_, err := e.ops.Create(context.Background(), c, bReq("vol-x", nil))
		extDone <- err
	}()
	eventually(t, func() bool { return e.ops.locks.refs("vol-x") == 1 }, 5*time.Second)

	biDone := make(chan error, 1)
	go func() {
		_, err := e.ops.CreateBuiltin(context.Background(), bReq("vol-x", nil))
		biDone <- err
	}()
	eventually(t, func() bool { return e.ops.locks.refs("vol-x") == 2 }, 5*time.Second)
	select {
	case <-biDone:
		t.Fatal("built-in create ran while the external plugin held the lock")
	case <-time.After(150 * time.Millisecond):
	}
	_, serr := os.Lstat(filepath.Join(e.volumes, "vol-x"))
	must.True(t, errors.Is(serr, os.ErrNotExist))

	must.NoError(t, os.WriteFile(gate, nil, 0o600))
	must.NoError(t, <-extDone)
	must.NoError(t, <-biDone)
	must.Eq(t, 0, e.ops.locks.len())
}

func TestBuiltin_CanceledWhileWaiting(t *testing.T) {
	e := newOpEnv(t)
	must.NoError(t, e.ops.locks.acquire(context.Background(), "vol-x"))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := e.ops.CreateBuiltin(ctx, bReq("vol-x", nil))
		done <- err
	}()
	eventually(t, func() bool { return e.ops.locks.refs("vol-x") == 2 }, 5*time.Second)
	cancel()
	asOpError(t, <-done, "create", ReasonCanceled)
	e.ops.locks.release("vol-x")
	must.SliceEmpty(t, entries(t, e.volumes))
	must.Eq(t, 0, e.ops.locks.len())

	// an already canceled context never reaches the filesystem
	err := e.ops.DeleteBuiltin(ctx, bReq("vol-x", nil))
	asOpError(t, err, "delete", ReasonCanceled)
}

func TestBuiltin_ReservedIDStillRejectedByExternalOps(t *testing.T) {
	e := newOpEnv(t)
	_, err := e.ops.Create(context.Background(), Candidate{ID: BuiltinMkdirID, Path: "/bin/true"}, bReq("v", nil))
	asOpError(t, err, "create", ReasonReservedID)
	err = e.ops.Delete(context.Background(), Candidate{ID: BuiltinMkdirID, Path: "/bin/true"}, bReq("v", nil), "/srv/x")
	asOpError(t, err, "delete", ReasonReservedID)
}

func statOf(t *testing.T, p string) *syscall.Stat_t {
	t.Helper()
	fi, err := os.Stat(p)
	must.NoError(t, err)
	return fi.Sys().(*syscall.Stat_t)
}

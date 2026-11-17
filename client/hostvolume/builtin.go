// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package hostvolume

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

// BuiltinMkdirVersion is the fixed version advertised for the built-in
// mkdir plugin (Wander policy, see the provenance document).
const BuiltinMkdirVersion = "1.0.0"

// ReasonFilesystem classifies a filesystem failure of a built-in operation.
// Its message is fixed text: raw OS errors can carry names and are dropped.
const ReasonFilesystem = "filesystem_failed"

const (
	// DefaultBuiltinMode is the recorded default mode (DHV-B-104).
	DefaultBuiltinMode os.FileMode = 0o700

	// MaxBuiltinIDLen bounds the volume ID accepted by the built-in plugin.
	MaxBuiltinIDLen = 64

	// managedRootMode is the mode of a managed root the built-in plugin has
	// to create. It is traversable by others, as the volumes inside have
	// their own modes.
	managedRootMode os.FileMode = 0o711
)

// BuiltinSupported reports whether the built-in mkdir plugin works here.
func BuiltinSupported() bool { return builtinSupported }

// applyOwner is the ownership step; tests replace it to force a failure.
var applyOwner = func(f *os.File, uid, gid int) error { return f.Chown(uid, gid) }

type builtinParams struct {
	mode     os.FileMode
	uid, gid int
}

// validBuiltinID reports whether id is one safe path element: 1 to 64 bytes
// of [a-z0-9_-]. That excludes separators, NUL, "." and "..", a leading dot,
// absolute paths and Windows device names structurally. Upper case is
// excluded because the lock is keyed on the exact string while some
// filesystems fold case.
func validBuiltinID(id string) bool {
	if id == "" || len(id) > MaxBuiltinIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// parseMode accepts 1 to 4 octal digits and permission bits 0777 only, so
// setuid, setgid and sticky are rejected.
func parseMode(s string) (os.FileMode, bool) {
	if len(s) < 1 || len(s) > 4 {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '7' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil || n > 0o777 {
		return 0, false
	}
	return os.FileMode(n), true
}

// parseID accepts decimal digits only, in 0..MaxInt32. This rejects
// 4294967295, which chown reads as "no change".
func parseID(s string) (int, bool) {
	if s == "" || len(s) > 10 {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n > math.MaxInt32 {
		return 0, false
	}
	return int(n), true
}

// parseBuiltinParams reads mode, uid and gid. Unknown keys and capacity are
// ignored. Messages never include a value.
func parseBuiltinParams(p map[string]string) (builtinParams, *OpError) {
	out := builtinParams{mode: DefaultBuiltinMode, uid: os.Getuid(), gid: os.Getgid()}
	if v, ok := p["mode"]; ok {
		m, ok := parseMode(v)
		if !ok {
			return out, opErr(OperationCreate, ReasonInvalidInput, "mode is not a valid octal permission value")
		}
		out.mode = m
	}
	if v, ok := p["uid"]; ok {
		n, ok := parseID(v)
		if !ok {
			return out, opErr(OperationCreate, ReasonInvalidInput, "uid is not a valid user id")
		}
		out.uid = n
	}
	if v, ok := p["gid"]; ok {
		n, ok := parseID(v)
		if !ok {
			return out, opErr(OperationCreate, ReasonInvalidInput, "gid is not a valid group id")
		}
		out.gid = n
	}
	return out, nil
}

// validateBuiltin checks everything before the lock or any filesystem call.
func (o *Ops) validateBuiltin(op string, req OpRequest) *OpError {
	if !builtinSupported {
		return opErr(op, ReasonUnsupportedOS, "")
	}
	if !validBuiltinID(req.VolumeID) {
		return opErr(op, ReasonInvalidInput, "volume id is not a valid built-in volume id")
	}
	if _, err := validateFields(op, req); err != nil {
		return err
	}
	if err := validatePath(o.VolumesDir); err != nil {
		return opErr(op, ReasonInvalidInput, "volumes directory: "+err.Error())
	}
	return nil
}

// lockVolume takes the per-volume lock shared with external operations.
func (o *Ops) lockVolume(ctx context.Context, op, id string) *OpError {
	if err := o.locks.acquire(ctx, id); err != nil {
		return opErr(op, ReasonCanceled, "")
	}
	if ctx.Err() != nil {
		o.locks.release(id)
		return opErr(op, ReasonCanceled, "")
	}
	return nil
}

// openManagedRoot checks that the root's final element is a real directory
// and opens it, then verifies the opened directory is the one checked. Only
// the final element is checked: ancestors belong to the operator.
func (o *Ops) openManagedRoot(op string) (*os.Root, *OpError) {
	li, err := os.Lstat(o.VolumesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, opErr(op, ReasonFilesystem, "managed volumes directory cannot be inspected")
	}
	if !li.IsDir() {
		return nil, opErr(op, ReasonInvalidPath, "managed volumes directory is not a plain directory")
	}
	root, err := os.OpenRoot(o.VolumesDir)
	if err != nil {
		return nil, opErr(op, ReasonFilesystem, "managed volumes directory cannot be opened")
	}
	ri, err := root.Stat(".")
	if err != nil || !os.SameFile(li, ri) {
		root.Close()
		return nil, opErr(op, ReasonInvalidPath, "managed volumes directory changed while opening")
	}
	return root, nil
}

// CreateBuiltin creates, or reuses, the directory VolumesDir/<volume id>
// and applies ownership and mode. Capacity is ignored and Bytes is 0. If
// applying ownership or mode fails, a directory this call created is
// removed; a directory that existed before is left in place. Either way the
// result is the zero value.
func (o *Ops) CreateBuiltin(ctx context.Context, req OpRequest) (CreateResult, error) {
	const op = OperationCreate
	if oerr := o.validateBuiltin(op, req); oerr != nil {
		return CreateResult{}, oerr
	}
	bp, oerr := parseBuiltinParams(req.Parameters)
	if oerr != nil {
		return CreateResult{}, oerr
	}
	if oerr := o.lockVolume(ctx, op, req.VolumeID); oerr != nil {
		return CreateResult{}, oerr
	}
	defer o.locks.release(req.VolumeID)

	root, oerr := o.openManagedRoot(op)
	if oerr != nil {
		return CreateResult{}, oerr
	}
	if root == nil {
		// The root does not exist yet, as on a fresh agent: create it once.
		if err := os.MkdirAll(o.VolumesDir, managedRootMode); err != nil {
			return CreateResult{}, opErr(op, ReasonFilesystem, "managed volumes directory cannot be created")
		}
		if root, oerr = o.openManagedRoot(op); oerr != nil {
			return CreateResult{}, oerr
		}
		if root == nil {
			return CreateResult{}, opErr(op, ReasonFilesystem, "managed volumes directory is missing")
		}
	}
	defer root.Close()

	id := req.VolumeID
	created := true
	if err := root.Mkdir(id, 0o700); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return CreateResult{}, opErr(op, ReasonFilesystem, "volume directory cannot be created")
		}
		created = false
	}
	fail := func(e *OpError) (CreateResult, error) {
		if created {
			if err := root.Remove(id); err != nil && !errors.Is(err, fs.ErrNotExist) {
				e.Message += "; the new directory could not be removed"
			}
		}
		return CreateResult{}, e
	}

	li, err := root.Lstat(id)
	if err != nil {
		return fail(opErr(op, ReasonFilesystem, "volume directory cannot be inspected"))
	}
	if !li.IsDir() {
		return CreateResult{}, opErr(op, ReasonInvalidPath, "volume path is not a plain directory")
	}
	// Root follows symlinks that stay inside it, so open without following
	// and compare with the Lstat result: this is the real defence.
	f, err := root.OpenFile(id, os.O_RDONLY|openDirFlags, 0)
	if err != nil {
		return fail(opErr(op, ReasonInvalidPath, "volume directory cannot be opened without following links"))
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !os.SameFile(li, fi) {
		return fail(opErr(op, ReasonInvalidPath, "volume directory changed while opening"))
	}
	// Always chown: on BSD and macOS a new directory takes the parent's
	// group, not the process group.
	if err := applyOwner(f, bp.uid, bp.gid); err != nil {
		return fail(opErr(op, ReasonFilesystem, "ownership of the volume directory cannot be set"))
	}
	if err := f.Chmod(bp.mode); err != nil {
		return fail(opErr(op, ReasonFilesystem, "mode of the volume directory cannot be set"))
	}
	return CreateResult{Path: filepath.Join(o.VolumesDir, id), Bytes: 0}, nil
}

// DeleteBuiltin removes VolumesDir/<volume id> and its contents. It takes no
// path: the path is derived here, so a registered external directory can
// never be reached. A missing root or directory is success. Symlinks are
// never followed; one found at the volume name is itself removed.
func (o *Ops) DeleteBuiltin(ctx context.Context, req OpRequest) error {
	const op = OperationDelete
	if oerr := o.validateBuiltin(op, req); oerr != nil {
		return oerr
	}
	if oerr := o.lockVolume(ctx, op, req.VolumeID); oerr != nil {
		return oerr
	}
	defer o.locks.release(req.VolumeID)

	root, oerr := o.openManagedRoot(op)
	if oerr != nil {
		return oerr
	}
	if root == nil {
		return nil
	}
	defer root.Close()

	id := req.VolumeID
	// Defensive: validation already excludes these.
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id {
		return opErr(op, ReasonInvalidInput, "volume id is not a valid built-in volume id")
	}
	if _, err := root.Lstat(id); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return opErr(op, ReasonFilesystem, "volume directory cannot be inspected")
	}
	if err := root.RemoveAll(id); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return opErr(op, ReasonFilesystem, "volume directory cannot be removed")
	}
	return nil
}

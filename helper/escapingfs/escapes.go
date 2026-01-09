// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package escapingfs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// PathEscapesAllocViaRelative returns if the given path escapes the allocation
// directory using relative paths.
//
// Only for use in server-side validation, where the real filesystem is not available.
// For client-side validation use PathEscapesAllocDir, which includes symlink validation
// as well.
//
// The prefix is joined to the path (e.g. "task/local"), and this function
// checks if path escapes the alloc dir, NOT the prefix directory within the alloc dir.
// With prefix="task/local", it will return false for "../secret", but
// true for "../../../../../../root" path; only the latter escapes the alloc dir.
func PathEscapesAllocViaRelative(prefix, path string) (bool, error) {
	// Verify the destination does not escape the task's directory. The "alloc-dir"
	// and "alloc-id" here are just placeholders; on a real filesystem they will
	// have different names. The names are not important, but rather the number of levels
	// in the path they represent.
	alloc, err := filepath.Abs(filepath.Join("/", "alloc-dir/", "alloc-id/"))
	if err != nil {
		return false, err
	}
	abs, err := filepath.Abs(filepath.Join(alloc, prefix, path))
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(alloc, abs)
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(rel, ".."), nil
}

// pathEscapesBaseViaSymlink returns if path escapes dir, taking into account evaluation
// of symlinks.
//
// The base directory must be an absolute path.
func pathEscapesBaseViaSymlink(base, full string) (bool, error) {
	// Resolve symlinks in both paths to handle cases where base itself
	// contains symlinks (e.g., /var -> /private/var on macOS)
	resolvedBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return false, err
	}

	resolvedFull, err := filepath.EvalSymlinks(full)
	if err != nil {
		return false, err
	}

	// Compute relative path from resolved base to the resolved full path.
	// If the relative path starts with "..", the full path is outside base.
	rel, err := filepath.Rel(resolvedBase, resolvedFull)
	if err != nil {
		return true, nil
	}

	escapes := strings.HasPrefix(rel, "..")
	return escapes, nil
}

// PathEscapesAllocDir returns true if base/prefix/path escapes the given base directory.
//
// Escaping a directory can be done with relative paths (e.g. ../../ etc.) or by
// using symlinks. This checks both methods.
//
// The base directory must be an absolute path.
func PathEscapesAllocDir(base, prefix, path string) (bool, error) {
	full := filepath.Join(base, prefix, path)

	// If base is not an absolute path, the caller passed in the wrong thing.
	if !filepath.IsAbs(base) {
		return false, errors.New("alloc dir must be absolute")
	}

	// Check path does not escape the alloc dir using relative paths.
	if escapes, err := PathEscapesAllocViaRelative(prefix, path); err != nil {
		return false, err
	} else if escapes {
		return true, nil
	}

	// Check path does not escape the alloc dir using symlinks.
	if escapes, err := pathEscapesBaseViaSymlink(base, full); err != nil {
		if os.IsNotExist(err) {
			// Treat non-existent files as non-errors; perhaps not ideal but we
			// have existing features (log-follow) that depend on this. Still safe,
			// because we do the symlink check on every ReadAt call also.
			return false, nil
		}
		return false, err
	} else if escapes {
		return true, nil
	}

	return false, nil
}

// PathEscapesSandbox returns whether previously cleaned path inside the
// sandbox directory (typically this will be the allocation directory)
// escapes.
func PathEscapesSandbox(sandboxDir, path string) bool {
	rel, err := filepath.Rel(sandboxDir, path)
	if err != nil {
		return true
	}
	if strings.HasPrefix(rel, "..") {
		return true
	}
	return false
}

// EnsurePath is used to make sure a path exists
func EnsurePath(path string, dir bool) error {
	if !dir {
		path = filepath.Dir(path)
	}
	return os.MkdirAll(path, 0755)
}

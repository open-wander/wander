// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package hostvolume

import "syscall"

// builtinSupported reports whether the built-in mkdir plugin works on this
// platform.
const builtinSupported = true

// openDirFlags make opening the volume directory refuse a symlink and a
// non-directory at the final element.
const openDirFlags = syscall.O_NOFOLLOW | syscall.O_DIRECTORY

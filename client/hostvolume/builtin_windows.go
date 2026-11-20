// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package hostvolume

// builtinSupported is false on Windows. External plugins are disabled there
// already, and the symlink, junction and reparse-point semantics together
// with the read-only attribute make the recorded owner-writable-bit-only
// behaviour (DHV-B-104) hard to confine and to test. Both built-in
// operations return unsupported_platform and nothing is advertised.
const builtinSupported = false

// openDirFlags is unused where the built-in plugin is disabled.
const openDirFlags = 0

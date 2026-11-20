// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package hostvolume

import "os/exec"

// execBitMeaningful is false on Windows: the execute bit does not exist, so
// no external plugins are discovered there.
const execBitMeaningful = false

// configureProcess keeps the default kill behaviour on Windows.
func configureProcess(cmd *exec.Cmd) {}

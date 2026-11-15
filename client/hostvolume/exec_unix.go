// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package hostvolume

import (
	"os/exec"
	"syscall"
)

// execBitMeaningful reports whether the file mode execute bit decides if a
// file is a runnable plugin.
const execBitMeaningful = true

// configureProcess puts the plugin in its own process group and makes
// cancellation kill the whole group, so a shell plugin's children cannot
// outlive the deadline.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

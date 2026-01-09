// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package subproc

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var (
	// executable is the executable of this process
	executable string
)

func init() {
	s, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("failed to detect executable: %v", err))
	}

	// when running tests, we need to use the real wander binary,
	// and make sure you recompile between changes!
	if strings.HasSuffix(s, ".test") {
		if s, err = exec.LookPath("wander"); err != nil {
			panic(fmt.Sprintf("failed to find wander binary: %v", err))
		}
	}
	executable = s
}

// Self returns the path to the executable of this process.
func Self() string {
	return executable
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package discover

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// WanderExecutable checks the current executable, then $GOPATH/bin, and finally
// the CWD, in that order. If it can't be found, an error is returned.
func WanderExecutable() (string, error) {
	wanderExe := "wander"
	if runtime.GOOS == "windows" {
		wanderExe = "wander.exe"
	}

	// Check the current executable.
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("Failed to determine the wander executable: %v", err)
	}

	if _, err := os.Stat(bin); err == nil && isWander(bin, wanderExe) {
		return bin, nil
	}

	// Check the $PATH
	if bin, err := exec.LookPath(wanderExe); err == nil {
		return bin, nil
	}

	// Check the $GOPATH.
	bin = filepath.Join(os.Getenv("GOPATH"), "bin", wanderExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	// Check the CWD.
	pwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("Could not find Wander executable (%v): %v", wanderExe, err)
	}

	bin = filepath.Join(pwd, wanderExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	// Check CWD/bin
	bin = filepath.Join(pwd, "bin", wanderExe)
	if _, err := os.Stat(bin); err == nil {
		return bin, nil
	}

	return "", fmt.Errorf("Could not find Wander executable (%v)", wanderExe)
}

// NomadExecutable is an alias for WanderExecutable for backwards compatibility.
func NomadExecutable() (string, error) {
	return WanderExecutable()
}

func isWander(path, wanderExe string) bool {
	if strings.HasSuffix(path, ".test") || strings.HasSuffix(path, ".test.exe") {
		return false
	}
	return true
}

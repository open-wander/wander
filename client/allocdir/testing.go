// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package allocdir

import (
	"os"
	"runtime"

	hclog "github.com/hashicorp/go-hclog"
	testing "github.com/mitchellh/go-testing-interface"
)

// TestAllocDir returns a built alloc dir in a temporary directory and cleanup
// func.
func TestAllocDir(t testing.T, l hclog.Logger, prefix, id string) (*AllocDir, func()) {
	tempRoot := ""
	if runtime.GOOS == "darwin" {
		// Darwin limits Unix socket paths to 104 bytes. Its default temporary
		// directory is long enough that allocation socket tests can exceed the
		// limit before appending the allocation ID and socket name.
		tempRoot = "/tmp"
	}

	dir, err := os.MkdirTemp(tempRoot, prefix)
	if err != nil {
		t.Fatalf("Couldn't create temp dir: %v", err)
	}

	allocDir := NewAllocDir(l, dir, id)

	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("error cleaning up alloc dir %q: %v", prefix, err)
		}

		if err := allocDir.Destroy(); err != nil {
			t.Logf("error cleaning up alloc dir %q: %v", prefix, err)
		}
	}

	if err := allocDir.Build(); err != nil {
		cleanup()
		t.Fatalf("error building alloc dir %q: %v", prefix, err)
	}

	return allocDir, cleanup
}

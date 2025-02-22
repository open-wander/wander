// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows

package docker

import (
	"testing"

	"github.com/open-wander/wander/client/allocdir"
	"github.com/open-wander/wander/testutil"
)

func newTaskConfig(variant string, command []string) TaskConfig {
	// busyboxImageID is an id of an image containing nanoserver windows and
	// a busybox exe.
	busyboxImageID := testutil.TestBusyboxImage()

	return TaskConfig{
		Image:            busyboxImageID,
		ImagePullTimeout: "5m",
		Command:          command[0],
		Args:             command[1:],
	}
}

// No-op on windows because we don't load images.
func copyImage(t *testing.T, taskDir *allocdir.TaskDir, image string) {
}

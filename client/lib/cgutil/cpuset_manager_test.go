// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cgutil

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/stretchr/testify/require"
)

func TestUtil_SplitPath(t *testing.T) {
	ci.Parallel(t)

	try := func(input, expParent, expCgroup string) {
		parent, cgroup := SplitPath(input)
		require.Equal(t, expParent, parent)
		require.Equal(t, expCgroup, cgroup)
	}

	// foo, /bar
	try("foo/bar", "foo", "/bar")
	try("/foo/bar/", "foo", "/bar")
	try("/sys/fs/cgroup/foo/bar", "foo", "/bar")

	// foo, /bar/baz
	try("/foo/bar/baz/", "foo", "/bar/baz")
	try("foo/bar/baz", "foo", "/bar/baz")
	try("/sys/fs/cgroup/foo/bar/baz", "foo", "/bar/baz")
}

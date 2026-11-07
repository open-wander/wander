// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"testing"

	"github.com/shoenig/test/must"
)

func Test_isCoveredOne(t *testing.T) {
	try := func(p string, exp bool) {
		result := isCoveredOne(p, "foo/bar")
		must.Eq(t, exp, result)
	}
	try("baz", false)
	try("foo", false)
	try("foo/bar/baz", false)
	try("foo/bar", true)
	try("foo/bar/...", true)
	try("foo/...", true)
	try("abc/...", false)
}

func TestGeneratedReleaseNoticePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "pkg/darwin_arm64/.release-notices", want: true},
		{path: "pkg/linux_amd64/.release-notices/licenses/go/example.com/module", want: true},
		{path: "pkg/linux_amd64/wander", want: false},
		{path: "pkg/.release-notices", want: false},
		{path: "other/linux_amd64/.release-notices", want: false},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			must.Eq(t, test.want, generatedReleaseNoticePath(test.path))
		})
	}
}

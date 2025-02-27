// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package docker

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/stretchr/testify/require"
)

func TestPublishedPorts_add(t *testing.T) {
	ci.Parallel(t)

	p := newPublishedPorts(testlog.HCLogger(t))
	p.add("label", "10.0.0.1", 1234, 80)
	p.add("label", "10.0.0.1", 5678, 80)
	for _, bindings := range p.publishedPorts {
		require.Len(t, bindings, 2)
	}
	require.Len(t, p.exposedPorts, 2)
}

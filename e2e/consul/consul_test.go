// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"testing"

	"github.com/open-wander/wander/e2e/e2eutil"
)

func TestConsul(t *testing.T) {
	// todo: migrate the remaining consul tests

	nomad := e2eutil.NomadClient(t)

	e2eutil.WaitForLeader(t, nomad)
	e2eutil.WaitForNodesReady(t, nomad, 1)

	t.Run("testServiceReversion", testServiceReversion)
	t.Run("testAllocRestart", testAllocRestart)
}

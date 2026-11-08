// Copyright (c) Open Wander contributors
// SPDX-License-Identifier: MPL-2.0

package version

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestPublicAndCompatibilityVersionsAreIndependent(t *testing.T) {
	must.Eq(t, "0.1.1-dev", GetVersion().VersionNumber())
	must.Eq(t, "1.6.5", ClusterCompatibilityVersion)
}

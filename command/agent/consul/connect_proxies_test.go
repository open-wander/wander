// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/stretchr/testify/require"
)

func TestConnectProxies_Proxies(t *testing.T) {
	ci.Parallel(t)

	pc := NewConnectProxiesClient(NewMockAgent(ossFeatures))

	proxies, err := pc.Proxies()
	require.NoError(t, err)
	require.Equal(t, map[string][]string{
		"envoy": {"1.14.2", "1.13.2", "1.12.4", "1.11.2"},
	}, proxies)
}

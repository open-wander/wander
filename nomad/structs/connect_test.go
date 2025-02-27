// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/stretchr/testify/require"
)

func TestTaskKind_IsAnyConnectGateway(t *testing.T) {
	ci.Parallel(t)

	t.Run("gateways", func(t *testing.T) {
		require.True(t, NewTaskKind(ConnectIngressPrefix, "foo").IsAnyConnectGateway())
		require.True(t, NewTaskKind(ConnectTerminatingPrefix, "foo").IsAnyConnectGateway())
		require.True(t, NewTaskKind(ConnectMeshPrefix, "foo").IsAnyConnectGateway())
	})

	t.Run("not gateways", func(t *testing.T) {
		require.False(t, NewTaskKind(ConnectProxyPrefix, "foo").IsAnyConnectGateway())
		require.False(t, NewTaskKind(ConnectNativePrefix, "foo").IsAnyConnectGateway())
		require.False(t, NewTaskKind("", "foo").IsAnyConnectGateway())
	})
}

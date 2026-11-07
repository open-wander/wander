// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/client/config"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func runFingerprintManager(t *testing.T, options map[string]string) *structs.Node {
	t.Helper()

	testClient, cleanup := TestClient(t, func(c *config.Config) {
		c.Options = options
	})
	t.Cleanup(func() { require.NoError(t, cleanup()) })

	fm := NewFingerprintManager(
		testClient.config.PluginSingletonLoader,
		testClient.GetConfig,
		testClient.config.Node,
		testClient.shutdownCh,
		testClient.updateNodeFromFingerprint,
		testClient.logger,
	)
	require.NoError(t, fm.Run())

	return testClient.config.Node
}

func TestFingerprintManager_Run_ResourcesFingerprint(t *testing.T) {
	ci.Parallel(t)
	node := runFingerprintManager(t, nil)

	require.NotEqual(t, 0, node.Resources.CPU)
	require.NotEqual(t, 0, node.Resources.MemoryMB)
	require.NotZero(t, node.Resources.DiskMB)
}

func TestFingerprintManager_Run_FilterLists(t *testing.T) {
	ci.Parallel(t)

	testCases := []struct {
		name    string
		options map[string]string
		present []string
		absent  []string
	}{
		{
			name: "default allowlist",
			options: map[string]string{
				"test.shutdown_periodic_after":    "true",
				"test.shutdown_periodic_duration": "2",
			},
			present: []string{"cpu.totalcompute"},
		},
		{
			name: "denylist",
			options: map[string]string{
				"fingerprint.allowlist": "  arch,memory,foo,bar\t",
				"fingerprint.denylist":  "  cpu\t",
			},
			present: []string{"memory.totalbytes"},
			absent:  []string{"cpu.totalcompute"},
		},
		{
			name: "allowlist and denylist",
			options: map[string]string{
				"fingerprint.allowlist": "  arch,cpu,memory,foo,bar\t",
				"fingerprint.denylist":  "  memory,host\t",
			},
			present: []string{"cpu.totalcompute", "cpu.arch"},
			absent:  []string{"memory.totalbytes", "os.name"},
		},
		{
			name: "legacy whitelist and blacklist",
			options: map[string]string{
				"fingerprint.whitelist": "  arch,cpu,memory,foo,bar\t",
				"fingerprint.blacklist": "  memory,host\t",
			},
			present: []string{"cpu.totalcompute", "cpu.arch"},
			absent:  []string{"memory.totalbytes", "os.name"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ci.Parallel(t)
			node := runFingerprintManager(t, testCase.options)

			for _, attribute := range testCase.present {
				require.NotEmpty(t, node.Attributes[attribute], attribute)
			}
			for _, attribute := range testCase.absent {
				require.NotContains(t, node.Attributes, attribute)
			}
		})
	}
}

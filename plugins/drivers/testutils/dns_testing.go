// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package testutils

import (
	"strings"
	"testing"

	"github.com/open-wander/wander/drivers/shared/resolvconf"
	"github.com/open-wander/wander/plugins/drivers"
	"github.com/shoenig/test/must"
)

// TestTaskDNSConfig asserts that a task is running with the given DNSConfig
func TestTaskDNSConfig(t *testing.T, driver *DriverHarness, taskID string, dns *drivers.DNSConfig) {
	t.Run("dns_config", func(t *testing.T) {
		caps, err := driver.Capabilities()
		must.NoError(t, err)

		// FS isolation is used here as a proxy for network isolation.
		// This is true for the current built-in drivers but it is not necessarily so.
		isolated := caps.FSIsolation != drivers.FSIsolationNone
		usesHostNetwork := caps.FSIsolation != drivers.FSIsolationImage

		if !isolated {
			t.Skip("dns config not supported on non isolated drivers")
		}

		// write to a file and check it presence in host
		r := execTask(t, driver, taskID, `cat /etc/resolv.conf`,
			false, "")
		must.Zero(t, r.exitCode)

		actual := resolvconf.Parse([]byte(strings.TrimSpace(r.stdout)))

		if dns != nil {
			if len(dns.Servers) > 0 {
				must.SliceContainsAll(t, dns.Servers, actual.Nameservers)
			}
			if len(dns.Searches) > 0 {
				must.SliceContainsAll(t, dns.Searches, actual.Searches)
			}
			if len(dns.Options) > 0 {
				must.SliceContainsAll(t, dns.Options, actual.Options)
			}
		} else {
			systemPath := "/etc/resolv.conf"
			if !usesHostNetwork {
				systemPath = resolvconf.SystemPath()
			}

			system, specificErr := resolvconf.Read(systemPath)
			must.NoError(t, specificErr)
			must.SliceContainsAll(t, system.Nameservers, actual.Nameservers)
			must.SliceContainsAll(t, system.Searches, actual.Searches)
			must.SliceContainsAll(t, system.Options, actual.Options)
		}
	})
}

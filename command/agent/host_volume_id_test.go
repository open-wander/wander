// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
)

// The ID and namespace of a host volume are set by the client for dynamic
// volumes. An agent host_volume block must not be able to set it, in HCL or in
// JSON.
func TestHostVolumeBlock_CannotSetID(t *testing.T) {
	ci.Parallel(t)

	hcl := `client {
  enabled = true
  host_volume "data" {
    path = "/synthetic/data"
    id   = "vol-forged"
    ID   = "vol-forged-2"
    namespace = "ns-forged"
  }
}`
	json := `{"client": {"enabled": true, "host_volume": {"data": {"path": "/synthetic/data", "id": "vol-forged", "namespace": "ns-forged"}}}}`

	for name, tc := range map[string]struct{ file, body string }{
		"hcl":  {"agent.hcl", hcl},
		"json": {"agent.json", json},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			must.NoError(t, os.WriteFile(path, []byte(tc.body), 0o600))

			parsed, err := ParseConfigFile(path)
			must.NoError(t, err)
			must.Len(t, 1, parsed.Client.HostVolumes)
			must.Eq(t, "/synthetic/data", parsed.Client.HostVolumes[0].Path)

			conf := DefaultConfig()
			conf.Client.Enabled = true
			conf.Client.HostVolumes = parsed.Client.HostVolumes
			must.Eq(t, "", parsed.Client.HostVolumes[0].ID)
			must.Eq(t, "", parsed.Client.HostVolumes[0].Namespace)
			// Even if the ID were set, the conversion clears it.
			conf.Client.HostVolumes[0].ID = "vol-forged"
			conf.Client.HostVolumes[0].Namespace = "ns-forged"
			c, err := convertClientConfig(conf)
			must.NoError(t, err)
			must.Eq(t, "", c.HostVolumes["data"].ID)
			must.Eq(t, "", c.HostVolumes["data"].Namespace)
			must.Eq(t, "/synthetic/data", c.HostVolumes["data"].Path)
		})
	}
}

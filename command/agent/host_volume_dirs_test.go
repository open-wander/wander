// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
)

func TestClientConfig_Merge_HostVolumeDirs(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name                 string
		a, b                 ClientConfig
		wantPlugin, wantVols string
	}{
		{"set in A, empty in B", ClientConfig{HostVolumePluginDir: "/a/p", HostVolumesDir: "/a/v"}, ClientConfig{}, "/a/p", "/a/v"},
		{"empty in A, set in B", ClientConfig{}, ClientConfig{HostVolumePluginDir: "/b/p", HostVolumesDir: "/b/v"}, "/b/p", "/b/v"},
		{"both set, B wins", ClientConfig{HostVolumePluginDir: "/a/p", HostVolumesDir: "/a/v"}, ClientConfig{HostVolumePluginDir: "/b/p", HostVolumesDir: "/b/v"}, "/b/p", "/b/v"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.a.Merge(&tc.b)
			must.Eq(t, tc.wantPlugin, got.HostVolumePluginDir)
			must.Eq(t, tc.wantVols, got.HostVolumesDir)
		})
	}
}

func TestClientConfig_validateHostVolumeDirs(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name    string
		cfg     ClientConfig
		wantErr string
	}{
		{"unset", ClientConfig{}, ""},
		{"absolute", ClientConfig{HostVolumePluginDir: "/opt/p", HostVolumesDir: "/opt/v"}, ""},
		{"relative plugin dir", ClientConfig{HostVolumePluginDir: "plugins"}, "client.host_volume_plugin_dir must be given as an absolute path"},
		{"relative volumes dir", ClientConfig{HostVolumesDir: "./vols"}, "client.host_volumes_dir must be given as an absolute path"},
		{"equal dirs", ClientConfig{HostVolumePluginDir: "/opt/x", HostVolumesDir: "/opt/x"}, "must be different directories"},
		{"equal after clean", ClientConfig{HostVolumePluginDir: "/opt/x/", HostVolumesDir: "/opt/y/../x"}, "must be different directories"},
		{"only one set", ClientConfig{HostVolumesDir: "/opt/x"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.validateHostVolumeDirs()
			if tc.wantErr == "" {
				must.NoError(t, err)
				return
			}
			must.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestAgent_ClientConfig_HostVolumeDirs(t *testing.T) {
	ci.Parallel(t)

	cases := []struct {
		name                 string
		dataDir              string
		client               ClientConfig
		wantPlugin, wantVols string
	}{
		{"data_dir only", "/data", ClientConfig{}, "/data/host_volume_plugins", "/data/host_volumes"},
		{"explicit override", "/data", ClientConfig{HostVolumePluginDir: "/x/p", HostVolumesDir: "/x/v"}, "/x/p", "/x/v"},
		{"partial override", "/data", ClientConfig{HostVolumesDir: "/x/v"}, "/data/host_volume_plugins", "/x/v"},
		{"nothing set", "", ClientConfig{}, "", ""},
		{"explicit without data_dir", "", ClientConfig{HostVolumePluginDir: "/x/p"}, "/x/p", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			conf := DevConfig(nil)
			conf.DataDir = tc.dataDir
			conf.Client.HostVolumePluginDir = tc.client.HostVolumePluginDir
			conf.Client.HostVolumesDir = tc.client.HostVolumesDir
			must.NoError(t, conf.normalizeAddrs())
			c, err := (&Agent{config: conf}).clientConfig()
			must.NoError(t, err)
			must.Eq(t, tc.wantPlugin, c.HostVolumePluginDir)
			must.Eq(t, tc.wantVols, c.HostVolumesDir)
		})
	}
}

// A reload rebuilds the client config from the merged agent config. The
// settings must survive a reload config that omits them, and a changed value
// is carried into the converted config (the running client applies it only
// after a restart).
func TestAgent_ClientConfig_HostVolumeDirs_Reload(t *testing.T) {
	ci.Parallel(t)

	base := DevConfig(nil)
	base.DataDir = "/data"
	base.Client.HostVolumesDir = "/x/v"
	must.NoError(t, base.normalizeAddrs())

	omit := base.Merge(&Config{Client: &ClientConfig{}})
	c, err := (&Agent{config: omit}).clientConfig()
	must.NoError(t, err)
	must.Eq(t, "/x/v", c.HostVolumesDir)
	must.Eq(t, "/data/host_volume_plugins", c.HostVolumePluginDir)

	changed := base.Merge(&Config{Client: &ClientConfig{HostVolumesDir: "/y/v"}})
	c, err = (&Agent{config: changed}).clientConfig()
	must.NoError(t, err)
	must.Eq(t, "/y/v", c.HostVolumesDir)
}

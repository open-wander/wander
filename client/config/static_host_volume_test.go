// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/stretchr/testify/require"
)

func TestConfig_StaticHostVolume(t *testing.T) {
	ci.Parallel(t)

	vol := &structs.ClientHostVolumeConfig{Name: "data", Path: "/srv/data"}
	withVol := &Config{HostVolumes: map[string]*structs.ClientHostVolumeConfig{"data": vol}}
	cases := []struct {
		name   string
		cfg    *Config
		lookup string
		want   bool
	}{
		{"hit", withVol, "data", true},
		{"miss", withVol, "other", false},
		{"nil map", &Config{}, "data", false},
		{"nil config", nil, "data", false},
		{"case sensitive", withVol, "Data", false},
		{"nil entry", &Config{HostVolumes: map[string]*structs.ClientHostVolumeConfig{"data": nil}}, "data", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tc.cfg.StaticHostVolume(tc.lookup)
			require.Equal(t, tc.want, ok)
			if tc.want {
				require.Same(t, vol, got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientHostVolumeConfig_IDRoundTrip(t *testing.T) {
	in := &Node{HostVolumes: map[string]*ClientHostVolumeConfig{
		"dyn":    {Name: "dyn", Path: "/synthetic/dyn", ID: "vol-synthetic-1"},
		"static": {Name: "static", Path: "/synthetic/static", ReadOnly: true},
	}}

	buf, err := Encode(NodeRegisterRequestType, in)
	require.NoError(t, err)

	var out Node
	require.NoError(t, Decode(buf[1:], &out))
	require.Equal(t, "vol-synthetic-1", out.HostVolumes["dyn"].ID)
	require.Equal(t, "", out.HostVolumes["static"].ID)
	require.True(t, out.HostVolumes["static"].ReadOnly)
}

func TestClientHostVolumeConfig_CopyCarriesID(t *testing.T) {
	c := (&ClientHostVolumeConfig{Name: "n", Path: "/synthetic/n", ID: "vol-synthetic-1"}).Copy()
	require.Equal(t, "vol-synthetic-1", c.ID)

	m := CopyMapStringClientHostVolumeConfig(map[string]*ClientHostVolumeConfig{"n": c})
	require.Equal(t, "vol-synthetic-1", m["n"].ID)
}

// A payload from a build without the ID field decodes with an empty ID.
func TestClientHostVolumeConfig_DecodeWithoutID(t *testing.T) {
	type oldConfig struct {
		Name     string
		Path     string
		ReadOnly bool
	}
	buf, err := Encode(NodeRegisterRequestType, &oldConfig{Name: "n", Path: "/synthetic/n", ReadOnly: true})
	require.NoError(t, err)

	var out ClientHostVolumeConfig
	require.NoError(t, Decode(buf[1:], &out))
	require.Equal(t, "n", out.Name)
	require.Equal(t, "/synthetic/n", out.Path)
	require.True(t, out.ReadOnly)
	require.Equal(t, "", out.ID)
}

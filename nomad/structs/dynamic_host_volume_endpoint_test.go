// Copyright (c) Open Wander contributors.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"testing"

	"github.com/open-wander/wander/ci"
	"github.com/shoenig/test/must"
)

func TestDynamicHostVolume_StubAndGetters(t *testing.T) {
	ci.Parallel(t)

	vol := &DynamicHostVolume{
		ID: "vol-a", Name: "data", Namespace: "ns-a", Type: VolumeTypeHost,
		NodeID: "node-a", NodePool: "pool-a", HostPath: "/synthetic/path", PluginID: "plug",
		CapacityBytes: 42, State: DynamicHostVolumeStateReady,
		Parameters:  map[string]string{"k": "v"},
		Attachments: []*DynamicHostVolumeAttachment{{AllocationID: "alloc"}},
		CreateTime:  10, ModifyTime: 11, CreateIndex: 5, ModifyIndex: 6,
	}

	must.Eq(t, &DynamicHostVolumeStub{
		ID: "vol-a", Name: "data", Namespace: "ns-a", PluginID: "plug",
		NodeID: "node-a", NodePool: "pool-a", CapacityBytes: 42,
		State: DynamicHostVolumeStateReady, CreateIndex: 5, ModifyIndex: 6,
		CreateTime: 10, ModifyTime: 11,
	}, vol.Stub())

	must.Eq(t, "vol-a", vol.GetID())
	must.Eq(t, "ns-a", vol.GetNamespace())
	must.Eq(t, uint64(5), vol.GetCreateIndex())

	var none *DynamicHostVolume
	must.Nil(t, none.Stub())
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package interfaces

import (
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/plugins/device"
)

type Client interface {
	AllocStateHandler
}

// AllocStateHandler exposes a handler to be called when an allocation's state changes
type AllocStateHandler interface {
	// AllocStateUpdated is used to emit an updated allocation. This allocation
	// is stripped to only include client settable fields.
	AllocStateUpdated(alloc *structs.Allocation)

	// PutAllocation is used to persist an updated allocation in the local state store.
	PutAllocation(*structs.Allocation) error
}

// DeviceStatsReporter gives access to the latest resource usage
// for devices
type DeviceStatsReporter interface {
	LatestDeviceResourceStats([]*structs.AllocatedDeviceResource) []*device.DeviceGroupStats
}

// EnvReplacer is an interface which can interpolate environment variables and
// is usually satisfied by taskenv.TaskEnv.
type EnvReplacer interface {
	ReplaceEnv(string) string
	ClientPath(string, bool) (string, bool)
}

// ArtifactGetter is an interface satisfied by the getter package.
type ArtifactGetter interface {
	// Get artifact and put it in the task directory.
	Get(taskEnv EnvReplacer, artifact *structs.TaskArtifact) error
}

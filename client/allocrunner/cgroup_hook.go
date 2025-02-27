// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package allocrunner

import (
	"github.com/open-wander/wander/client/lib/cgutil"
	"github.com/open-wander/wander/nomad/structs"
)

func newCgroupHook(alloc *structs.Allocation, man cgutil.CpusetManager) *cgroupHook {
	return &cgroupHook{
		alloc:         alloc,
		cpusetManager: man,
	}
}

type cgroupHook struct {
	alloc         *structs.Allocation
	cpusetManager cgutil.CpusetManager
}

func (c *cgroupHook) Name() string {
	return "cgroup"
}

func (c *cgroupHook) Prerun() error {
	c.cpusetManager.AddAlloc(c.alloc)
	return nil
}

func (c *cgroupHook) Postrun() error {
	c.cpusetManager.RemoveAlloc(c.alloc.ID)
	return nil
}

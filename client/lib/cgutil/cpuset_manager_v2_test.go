// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build linux

package cgutil

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-wander/wander/client/testutil"
	"github.com/open-wander/wander/helper/testlog"
	"github.com/open-wander/wander/helper/uuid"
	"github.com/open-wander/wander/lib/cpuset"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/stretchr/testify/require"
)

// Note: these tests need to run on GitHub Actions runners with only 2 cores.
// It is not possible to write more cores to a cpuset than are actually available,
// so make sure tests account for that by setting systemCores as the full set of
// usable cores.
var systemCores = []uint16{0, 1}

func TestCpusetManager_V2_AddAlloc(t *testing.T) {
	testutil.CgroupsCompatibleV2(t)
	testutil.MinimumCores(t, 2)

	logger := testlog.HCLogger(t)
	parent := uuid.Short() + ".scope"
	create(t, parent)
	cleanup(t, parent)

	// setup the cpuset manager
	manager := NewCpusetManagerV2(parent, systemCores, logger)
	manager.Init()

	// add our first alloc, isolating 1 core
	t.Run("first", func(t *testing.T) {
		alloc := mock.Alloc()
		alloc.AllocatedResources.Tasks["web"].Cpu.ReservedCores = cpuset.New(0).ToSlice()
		manager.AddAlloc(alloc)
		cpusetIs(t, "0-1", parent, alloc.ID, "web")
	})

	// add second alloc, isolating 1 core
	t.Run("second", func(t *testing.T) {
		alloc := mock.Alloc()
		alloc.AllocatedResources.Tasks["web"].Cpu.ReservedCores = cpuset.New(1).ToSlice()
		manager.AddAlloc(alloc)
		cpusetIs(t, "1", parent, alloc.ID, "web")
	})

	// note that the scheduler, not the cpuset manager, is what prevents over-subscription
	// and as such no logic exists here to prevent that
}

func cpusetIs(t *testing.T, exp, parent, allocID, task string) {
	scope := makeScope(makeID(allocID, task))
	value, err := cgroups.ReadFile(filepath.Join(CgroupRoot, parent, scope), "cpuset.cpus")
	require.NoError(t, err)
	require.Equal(t, exp, strings.TrimSpace(value))
}

func TestCpusetManager_V2_RemoveAlloc(t *testing.T) {
	testutil.CgroupsCompatibleV2(t)
	testutil.MinimumCores(t, 2)

	logger := testlog.HCLogger(t)
	parent := uuid.Short() + ".scope"
	create(t, parent)
	cleanup(t, parent)

	// setup the cpuset manager
	manager := NewCpusetManagerV2(parent, systemCores, logger)
	manager.Init()

	// alloc1 gets core 0
	alloc1 := mock.Alloc()
	alloc1.AllocatedResources.Tasks["web"].Cpu.ReservedCores = cpuset.New(0).ToSlice()
	manager.AddAlloc(alloc1)

	// alloc2 gets core 1
	alloc2 := mock.Alloc()
	alloc2.AllocatedResources.Tasks["web"].Cpu.ReservedCores = cpuset.New(1).ToSlice()
	manager.AddAlloc(alloc2)
	cpusetIs(t, "1", parent, alloc2.ID, "web")

	// with alloc1 gone, alloc2 gets the now shared core
	manager.RemoveAlloc(alloc1.ID)
	cpusetIs(t, "0-1", parent, alloc2.ID, "web")
}

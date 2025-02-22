// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"github.com/open-wander/wander/client/serviceregistration"
	"github.com/open-wander/wander/client/taskenv"
	"github.com/open-wander/wander/nomad/mock"
	"github.com/open-wander/wander/nomad/structs"
	"github.com/open-wander/wander/plugins/drivers"
)

func BuildAllocServices(
	node *structs.Node, alloc *structs.Allocation, restarter serviceregistration.WorkloadRestarter) *serviceregistration.WorkloadServices {

	//TODO(schmichael) only support one network for now
	net := alloc.AllocatedResources.Shared.Networks[0]

	tg := alloc.Job.LookupTaskGroup(alloc.TaskGroup)

	ws := &serviceregistration.WorkloadServices{
		AllocInfo: structs.AllocInfo{
			AllocID: alloc.ID,
			Group:   alloc.TaskGroup,
		},
		Services: taskenv.InterpolateServices(taskenv.NewBuilder(mock.Node(), alloc, nil, alloc.Job.Region).Build(), tg.Services),
		Networks: alloc.AllocatedResources.Shared.Networks,

		//TODO(schmichael) there's probably a better way than hacking driver network
		DriverNetwork: &drivers.DriverNetwork{
			AutoAdvertise: true,
			IP:            net.IP,
			// Copy PortLabels from group network
			PortMap: net.PortLabels(),
		},

		Restarter:  restarter,
		DriverExec: nil,
	}

	if alloc.DeploymentStatus != nil {
		ws.Canary = alloc.DeploymentStatus.Canary
	}

	return ws
}

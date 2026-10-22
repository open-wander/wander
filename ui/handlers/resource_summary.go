package handlers

import "github.com/open-wander/wander/api"

const bytesPerMegabyte = int64(1024 * 1024)

type nodeAllocationResources struct {
	Count       int
	CPU         int64
	MemoryBytes int64
	DiskBytes   int64
}

func resourceQueryOptions(token string) *api.QueryOptions {
	return &api.QueryOptions{
		AuthToken: token,
		Params:    map[string]string{"resources": "true"},
	}
}

func summarizeNodeAllocations(allocations []*api.AllocationListStub) map[string]nodeAllocationResources {
	byNode := make(map[string]nodeAllocationResources)
	for _, allocation := range allocations {
		if allocation == nil || !isActiveAllocation(allocation.ClientStatus) {
			continue
		}

		summary := byNode[allocation.NodeID]
		summary.Count++
		if allocation.AllocatedResources != nil {
			for _, task := range allocation.AllocatedResources.Tasks {
				if task == nil {
					continue
				}
				summary.CPU += task.Cpu.CpuShares
				summary.MemoryBytes += task.Memory.MemoryMB * bytesPerMegabyte
			}
			summary.DiskBytes += allocation.AllocatedResources.Shared.DiskMB * bytesPerMegabyte
		}
		byNode[allocation.NodeID] = summary
	}
	return byNode
}

func isActiveAllocation(status string) bool {
	return status == api.AllocClientStatusPending || status == api.AllocClientStatusRunning
}

func nodeCapacity(node *api.NodeListStub) (cpu int64, memoryBytes, diskBytes int64, available bool) {
	if node == nil || node.NodeResources == nil {
		return 0, 0, 0, false
	}

	cpu = node.NodeResources.Cpu.CpuShares
	memoryMB := node.NodeResources.Memory.MemoryMB
	diskMB := node.NodeResources.Disk.DiskMB
	if node.ReservedResources != nil {
		cpu = subtractReserved(cpu, node.ReservedResources.Cpu.CpuShares)
		memoryMB = subtractReserved(memoryMB, node.ReservedResources.Memory.MemoryMB)
		diskMB = subtractReserved(diskMB, node.ReservedResources.Disk.DiskMB)
	}
	return cpu, memoryMB * bytesPerMegabyte, diskMB * bytesPerMegabyte, true
}

func subtractReserved(total int64, reserved uint64) int64 {
	if total <= 0 || reserved >= uint64(total) {
		return 0
	}
	return total - int64(reserved)
}

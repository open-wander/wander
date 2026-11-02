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
		cpu, memoryBytes, diskBytes := allocationReservations(allocation)
		summary.CPU += cpu
		summary.MemoryBytes += memoryBytes
		summary.DiskBytes += diskBytes
		byNode[allocation.NodeID] = summary
	}
	return byNode
}

func allocationReservations(allocation *api.AllocationListStub) (cpu, memoryBytes, diskBytes int64) {
	if allocation == nil || allocation.AllocatedResources == nil {
		return 0, 0, 0
	}
	for _, task := range allocation.AllocatedResources.Tasks {
		if task == nil {
			continue
		}
		cpu += task.Cpu.CpuShares
		memoryBytes += task.Memory.MemoryMB * bytesPerMegabyte
	}
	return cpu, memoryBytes, allocation.AllocatedResources.Shared.DiskMB * bytesPerMegabyte
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
	return total - int64(reserved) // #nosec G115 -- reserved is proven smaller than the positive int64 total above.
}

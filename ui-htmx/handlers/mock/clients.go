package mock

import (
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/partials/clientspartials"
)

// GetClients returns mock client summary data.
func (p *Provider) GetClients() []clientspartials.ClientSummary {
	return []clientspartials.ClientSummary{
		{
			ID:          "c1a2b3c4-d5e6-7f8g-9h0i-1j2k3l4m5n6o",
			Name:        "client-node-1",
			Address:     "192.168.1.100:4646",
			Status:      "ready",
			Datacenter:  "dc1",
			NodeClass:   "compute",
			Allocations: 5,
			CPUUsed:     2400,
			CPUTotal:    8000,
			MemUsed:     8589934592,   // 8 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    32212254720,  // 30 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "d2e3f4g5-h6i7-8j9k-0l1m-2n3o4p5q6r7s",
			Name:        "client-node-2",
			Address:     "192.168.1.101:4646",
			Status:      "ready",
			Datacenter:  "dc1",
			NodeClass:   "compute",
			Allocations: 3,
			CPUUsed:     4200,
			CPUTotal:    8000,
			MemUsed:     12884901888,  // 12 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    53687091200,  // 50 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "e3f4g5h6-i7j8-9k0l-1m2n-3o4p5q6r7s8t",
			Name:        "client-node-3",
			Address:     "192.168.2.100:4646",
			Status:      "down",
			Datacenter:  "dc2",
			NodeClass:   "storage",
			Allocations: 0,
			CPUUsed:     0,
			CPUTotal:    8000,
			MemUsed:     0,
			MemTotal:    17179869184, // 16 GB in bytes
			DiskUsed:    0,
			DiskTotal:   214748364800, // 200 GB in bytes
		},
		{
			ID:          "f4g5h6i7-j8k9-0l1m-2n3o-4p5q6r7s8t9u",
			Name:        "client-node-4",
			Address:     "192.168.1.102:4646",
			Status:      "ready",
			Datacenter:  "dc1",
			NodeClass:   "high-memory",
			Allocations: 8,
			CPUUsed:     6800,
			CPUTotal:    16000,
			MemUsed:     25769803776,  // 24 GB in bytes
			MemTotal:    34359738368,  // 32 GB in bytes
			DiskUsed:    96636764160,  // 90 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "g5h6i7j8-k9l0-1m2n-3o4p-5q6r7s8t9u0v",
			Name:        "client-node-5",
			Address:     "192.168.2.101:4646",
			Status:      "draining",
			Datacenter:  "dc2",
			NodeClass:   "compute",
			Allocations: 2,
			CPUUsed:     1200,
			CPUTotal:    8000,
			MemUsed:     4294967296,   // 4 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    21474836480,  // 20 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
		{
			ID:          "h6i7j8k9-l0m1-2n3o-4p5q-6r7s8t9u0v1w",
			Name:        "client-node-6",
			Address:     "192.168.2.102:4646",
			Status:      "ready",
			Datacenter:  "dc2",
			NodeClass:   "gpu",
			Allocations: 6,
			CPUUsed:     5400,
			CPUTotal:    8000,
			MemUsed:     10737418240,  // 10 GB in bytes
			MemTotal:    17179869184,  // 16 GB in bytes
			DiskUsed:    80530636800,  // 75 GB in bytes
			DiskTotal:   107374182400, // 100 GB in bytes
		},
	}
}

// GetClientDetail returns mock client detail data.
func (p *Provider) GetClientDetail(clientID string) clientspartials.ClientDetail {
	now := time.Now()

	// Generate mock CPU history data (last 24 hours, every 30 minutes)
	cpuHistory := make([]clientspartials.ResourceDataPoint, 0, 48)
	memoryHistory := make([]clientspartials.ResourceDataPoint, 0, 48)
	for i := 47; i >= 0; i-- {
		t := now.Add(-time.Duration(i) * 30 * time.Minute)
		// Simulate varying CPU usage between 20-50%
		cpuValue := 25.0 + 15.0*float64((i+7)%10)/10.0 + 5.0*float64(i%5)
		if cpuValue > 50 {
			cpuValue = 50
		}
		cpuHistory = append(cpuHistory, clientspartials.ResourceDataPoint{
			Time:  t,
			Value: cpuValue,
		})
		// Simulate varying memory usage between 40-60%
		memValue := 45.0 + 10.0*float64((i+3)%8)/8.0 + 5.0*float64(i%4)
		if memValue > 60 {
			memValue = 60
		}
		memoryHistory = append(memoryHistory, clientspartials.ResourceDataPoint{
			Time:  t,
			Value: memValue,
		})
	}

	return clientspartials.ClientDetail{
		ID:         clientID,
		Name:       "client-node-1",
		Status:     "ready",
		Datacenter: "dc1",
		Address:    "192.168.1.100:4646",
		Version:    "1.7.2",
		Uptime:     "5d 12h 30m",
		NodeClass:  "compute",
		Drain:      false,
		Draining:   false,
		Eligible:   true,
		Resources: clientspartials.ClientResources{
			CPUUsed:   2400,
			CPUTotal:  8000,
			MemUsed:   8589934592,   // 8 GB in bytes
			MemTotal:  17179869184,  // 16 GB in bytes
			DiskUsed:  53687091200,  // 50 GB in bytes
			DiskTotal: 107374182400, // 100 GB in bytes
		},
		CPUHistory:    cpuHistory,
		MemoryHistory: memoryHistory,
		Allocations: []clientspartials.AllocationSummary{
			{
				ID:        "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				JobID:     "web-frontend",
				TaskGroup: "web",
				Status:    "running",
				CPUUsed:   500,
				MemUsed:   1073741824, // 1 GB
			},
			{
				ID:        "b2c3d4e5-f6g7-8901-bcde-f12345678901",
				JobID:     "api-gateway",
				TaskGroup: "api",
				Status:    "running",
				CPUUsed:   800,
				MemUsed:   2147483648, // 2 GB
			},
			{
				ID:        "c3d4e5f6-g7h8-9012-cdef-123456789012",
				JobID:     "cache-service",
				TaskGroup: "cache",
				Status:    "running",
				CPUUsed:   400,
				MemUsed:   1610612736, // 1.5 GB
			},
			{
				ID:        "d4e5f6g7-h8i9-0123-def1-234567890123",
				JobID:     "data-processor",
				TaskGroup: "processor",
				Status:    "running",
				CPUUsed:   600,
				MemUsed:   2684354560, // 2.5 GB
			},
			{
				ID:        "e5f6g7h8-i9j0-1234-ef12-345678901234",
				JobID:     "metrics-collector",
				TaskGroup: "metrics",
				Status:    "running",
				CPUUsed:   100,
				MemUsed:   536870912, // 512 MB
			},
		},
		Attributes: map[string]string{
			"kernel.name":               "linux",
			"kernel.version":            "5.15.0-91-generic",
			"os.name":                   "ubuntu",
			"os.version":                "22.04",
			"cpu.arch":                  "amd64",
			"cpu.numcores":              "8",
			"cpu.totalcompute":          "8000",
			"memory.totalbytes":         "17179869184",
			"unique.hostname":           "client-node-1",
			"unique.network.ip-address": "192.168.1.100",
			"driver.docker":             "1",
			"driver.docker.version":     "24.0.7",
			"driver.raw_exec":           "1",
		},
		Meta: map[string]string{
			"environment":    "production",
			"region":         "us-west-2",
			"zone":           "us-west-2a",
			"instance_type":  "m5.2xlarge",
			"cloud_provider": "aws",
		},
		Drivers: map[string]clientspartials.DriverInfo{
			"docker": {
				Detected: true,
				Healthy:  true,
				Version:  "24.0.7",
			},
			"raw_exec": {
				Detected: true,
				Healthy:  true,
				Version:  "",
			},
			"exec": {
				Detected: true,
				Healthy:  true,
				Version:  "",
			},
			"java": {
				Detected: false,
				Healthy:  false,
				Version:  "",
			},
		},
		Events: []clientspartials.ClientEvent{
			{
				Time:    now.Add(-5 * time.Minute),
				Type:    "Allocation placed",
				Message: "Allocation e5f6g7h8 placed on node",
			},
			{
				Time:    now.Add(-2 * time.Hour),
				Type:    "Allocation placed",
				Message: "Allocation d4e5f6g7 placed on node",
			},
			{
				Time:    now.Add(-1 * 24 * time.Hour),
				Type:    "Node eligibility changed",
				Message: "Node eligibility set to eligible",
			},
			{
				Time:    now.Add(-3 * 24 * time.Hour),
				Type:    "Node drain complete",
				Message: "Drain completed successfully",
			},
			{
				Time:    now.Add(-3*24*time.Hour - 30*time.Minute),
				Type:    "Node drain started",
				Message: "Drain initiated for maintenance",
			},
			{
				Time:    now.Add(-5 * 24 * time.Hour),
				Type:    "Node registered",
				Message: "Node successfully registered with cluster",
			},
		},
	}
}

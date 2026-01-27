package mock

import (
	"strings"
	"time"

	"github.com/open-wander/wander/ui/handlers"
	"github.com/open-wander/wander/ui/templates/partials/allocationspartials"
)

// GetAllocations returns mock allocation summary data.
func (p *Provider) GetAllocations() []handlers.AllocationSummary {
	now := time.Now()

	return []handlers.AllocationSummary{
		{
			ID:        "a1b2c3d4-e5f6-4a5b-9c8d-1e2f3a4b5c6d",
			JobID:     "web-frontend",
			TaskGroup: "web",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "running",
			Created:   now.Add(-24 * time.Hour),
		},
		{
			ID:        "b2c3d4e5-f6a7-5b6c-ad9e-2f3a4b5c6d7e",
			JobID:     "web-frontend",
			TaskGroup: "web",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "running",
			Created:   now.Add(-24 * time.Hour),
		},
		{
			ID:        "c3d4e5f6-a7b8-6c7d-be0f-3a4b5c6d7e8f",
			JobID:     "api-gateway",
			TaskGroup: "api",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "running",
			Created:   now.Add(-48 * time.Hour),
		},
		{
			ID:        "d4e5f6a7-b8c9-7d8e-cf1a-4b5c6d7e8f9a",
			JobID:     "data-processor",
			TaskGroup: "processor",
			NodeID:    "node-3",
			NodeName:  "worker-03.dc1",
			Status:    "pending",
			Created:   now.Add(-1 * time.Hour),
		},
		{
			ID:        "e5f6a7b8-c9da-8e9f-da2b-5c6d7e8f9a0b",
			JobID:     "cache-service",
			TaskGroup: "cache",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "running",
			Created:   now.Add(-72 * time.Hour),
		},
		{
			ID:        "f6a7b8c9-dae0-9fa0-eb3c-6d7e8f9a0b1c",
			JobID:     "backup-job",
			TaskGroup: "backup",
			NodeID:    "node-4",
			NodeName:  "worker-04.dc2",
			Status:    "failed",
			Created:   now.Add(-120 * time.Hour),
		},
		{
			ID:        "a7b8c9da-e0f1-0ab1-fc4d-7e8f9a0b1c2d",
			JobID:     "metrics-collector",
			TaskGroup: "collector",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "running",
			Created:   now.Add(-96 * time.Hour),
		},
		{
			ID:        "b8c9dae0-f1a2-1bc2-ad5e-8f9a0b1c2d3e",
			JobID:     "web-frontend",
			TaskGroup: "cache",
			NodeID:    "node-3",
			NodeName:  "worker-03.dc1",
			Status:    "complete",
			Created:   now.Add(-168 * time.Hour),
		},
		{
			ID:        "c9dae0f1-a2b3-2cd3-be6f-9a0b1c2d3e4f",
			JobID:     "log-shipper",
			TaskGroup: "shipper",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "running",
			Created:   now.Add(-36 * time.Hour),
		},
		{
			ID:        "dae0f1a2-b3c4-3de4-cf7a-0b1c2d3e4f5a",
			JobID:     "api-gateway",
			TaskGroup: "api",
			NodeID:    "node-4",
			NodeName:  "worker-04.dc2",
			Status:    "running",
			Created:   now.Add(-60 * time.Hour),
		},
		{
			ID:        "e0f1a2b3-c4d5-4ef5-da8b-1c2d3e4f5a6b",
			JobID:     "batch-processor",
			TaskGroup: "batch",
			NodeID:    "node-1",
			NodeName:  "worker-01.dc1",
			Status:    "complete",
			Created:   now.Add(-200 * time.Hour),
		},
		{
			ID:        "f1a2b3c4-d5e6-5fa6-eb9c-2d3e4f5a6b7c",
			JobID:     "data-processor",
			TaskGroup: "processor",
			NodeID:    "node-2",
			NodeName:  "worker-02.dc1",
			Status:    "failed",
			Created:   now.Add(-12 * time.Hour),
		},
	}
}

// GetAllocationDetail returns mock allocation detail data.
func (p *Provider) GetAllocationDetail(allocID string) allocationspartials.AllocationDetail {
	now := time.Now()

	return allocationspartials.AllocationDetail{
		ID:        allocID,
		JobID:     "web-frontend",
		TaskGroup: "web",
		NodeID:    "node-1234",
		NodeName:  "worker-01.example.com",
		Status:    "running",
		Created:   now.Add(-2 * time.Hour),
		Modified:  now.Add(-5 * time.Minute),
		Tasks: []allocationspartials.TaskStatus{
			{
				Name:       "nginx",
				State:      "running",
				Failed:     false,
				StartedAt:  now.Add(-2 * time.Hour),
				FinishedAt: time.Time{}, // Running, so no finish time
				Restarts:   0,
				Resources: allocationspartials.TaskResources{
					CPU:      256,
					Memory:   512,
					CPUUsage: 35,
					MemUsage: 62,
				},
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Started",
						Time:       now.Add(-2 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
					{
						Type:       "Task Setup",
						Time:       now.Add(-125 * time.Minute),
						DisplayMsg: "Building Task Directory",
						FailsTask:  false,
					},
					{
						Type:       "Driver",
						Time:       now.Add(-124 * time.Minute),
						DisplayMsg: "Downloading image nginx:1.25-alpine",
						FailsTask:  false,
					},
					{
						Type:       "Received",
						Time:       now.Add(-126 * time.Minute),
						DisplayMsg: "Task received by client",
						FailsTask:  false,
					},
				},
			},
			{
				Name:       "redis",
				State:      "running",
				Failed:     false,
				StartedAt:  now.Add(-2 * time.Hour),
				FinishedAt: time.Time{}, // Running, so no finish time
				Restarts:   2,
				Resources: allocationspartials.TaskResources{
					CPU:      256,
					Memory:   512,
					CPUUsage: 12,
					MemUsage: 78,
				},
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Started",
						Time:       now.Add(-30 * time.Minute),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
					{
						Type:       "Restarting",
						Time:       now.Add(-31 * time.Minute),
						DisplayMsg: "Task restarting in 10s",
						FailsTask:  false,
					},
					{
						Type:       "Terminated",
						Time:       now.Add(-32 * time.Minute),
						DisplayMsg: "Exit Code: 137, Signal: 9",
						FailsTask:  false,
					},
					{
						Type:       "Started",
						Time:       now.Add(-1 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
					{
						Type:       "Restarting",
						Time:       now.Add(-61 * time.Minute),
						DisplayMsg: "Task restarting in 10s",
						FailsTask:  false,
					},
					{
						Type:       "Terminated",
						Time:       now.Add(-62 * time.Minute),
						DisplayMsg: "Exit Code: 1, OOM Killed",
						FailsTask:  true,
					},
					{
						Type:       "Started",
						Time:       now.Add(-2 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
				},
			},
			{
				Name:       "sidecar",
				State:      "dead",
				Failed:     false,
				StartedAt:  now.Add(-2 * time.Hour),
				FinishedAt: now.Add(-1 * time.Hour),
				Restarts:   0,
				Resources: allocationspartials.TaskResources{
					CPU:      100,
					Memory:   128,
					CPUUsage: 0,
					MemUsage: 0,
				},
				Events: []allocationspartials.TaskEvent{
					{
						Type:       "Terminated",
						Time:       now.Add(-1 * time.Hour),
						DisplayMsg: "Exit Code: 0, Signal: 0",
						FailsTask:  false,
					},
					{
						Type:       "Started",
						Time:       now.Add(-2 * time.Hour),
						DisplayMsg: "Task started by client",
						FailsTask:  false,
					},
				},
			},
		},
		Resources: allocationspartials.ResourceUsage{
			CPU:    612,
			Memory: 1152,
			Disk:   2048,
		},
		Events: []allocationspartials.AllocationEvent{
			{
				Type:    "Allocation Healthy",
				Time:    now.Add(-115 * time.Minute),
				Message: "All tasks are running",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-119 * time.Minute),
				Message: "Task 'nginx' started",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-119 * time.Minute),
				Message: "Task 'redis' started",
			},
			{
				Type:    "Task Started",
				Time:    now.Add(-120 * time.Minute),
				Message: "Task 'sidecar' started",
			},
			{
				Type:    "Received",
				Time:    now.Add(-2 * time.Hour),
				Message: "Allocation received by client",
			},
		},
	}
}

// GetTasksForAllocation returns the list of tasks for an allocation.
func (p *Provider) GetTasksForAllocation(allocID string) []string {
	// Mock task list
	return []string{"nginx", "redis", "sidecar"}
}

// GetMockFiles returns mock file entries for testing.
func (p *Provider) GetMockFiles(allocID, task, path string) []allocationspartials.FileEntry {
	now := time.Now()

	// Root level files
	if path == "/" {
		return []allocationspartials.FileEntry{
			{
				Name:    "alloc",
				Path:    "/alloc",
				IsDir:   true,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "local",
				Path:    "/local",
				IsDir:   true,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "secrets",
				Path:    "/secrets",
				IsDir:   true,
				ModTime: now.Add(-2 * time.Hour),
			},
		}
	}

	// /alloc directory
	if path == "/alloc" {
		return []allocationspartials.FileEntry{
			{
				Name:    "logs",
				Path:    "/alloc/logs",
				IsDir:   true,
				ModTime: now.Add(-1 * time.Hour),
			},
			{
				Name:    "data",
				Path:    "/alloc/data",
				IsDir:   true,
				ModTime: now.Add(-30 * time.Minute),
			},
		}
	}

	// /alloc/logs directory
	if path == "/alloc/logs" {
		return []allocationspartials.FileEntry{
			{
				Name:    task + ".stdout.0",
				Path:    "/alloc/logs/" + task + ".stdout.0",
				IsDir:   false,
				Size:    4523,
				ModTime: now.Add(-5 * time.Minute),
			},
			{
				Name:    task + ".stderr.0",
				Path:    "/alloc/logs/" + task + ".stderr.0",
				IsDir:   false,
				Size:    1256,
				ModTime: now.Add(-5 * time.Minute),
			},
		}
	}

	// /alloc/data directory
	if path == "/alloc/data" {
		return []allocationspartials.FileEntry{
			{
				Name:    "cache",
				Path:    "/alloc/data/cache",
				IsDir:   true,
				ModTime: now.Add(-15 * time.Minute),
			},
			{
				Name:    "state.json",
				Path:    "/alloc/data/state.json",
				IsDir:   false,
				Size:    892,
				ModTime: now.Add(-10 * time.Minute),
			},
		}
	}

	// /alloc/data/cache directory
	if path == "/alloc/data/cache" {
		return []allocationspartials.FileEntry{
			{
				Name:    "index.db",
				Path:    "/alloc/data/cache/index.db",
				IsDir:   false,
				Size:    102400,
				ModTime: now.Add(-8 * time.Minute),
			},
		}
	}

	// /local directory
	if path == "/local" {
		return []allocationspartials.FileEntry{
			{
				Name:    "config.json",
				Path:    "/local/config.json",
				IsDir:   false,
				Size:    2048,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "run.sh",
				Path:    "/local/run.sh",
				IsDir:   false,
				Size:    512,
				ModTime: now.Add(-2 * time.Hour),
			},
		}
	}

	// /secrets directory
	if path == "/secrets" {
		return []allocationspartials.FileEntry{
			{
				Name:    "api_key",
				Path:    "/secrets/api_key",
				IsDir:   false,
				Size:    64,
				ModTime: now.Add(-2 * time.Hour),
			},
			{
				Name:    "cert.pem",
				Path:    "/secrets/cert.pem",
				IsDir:   false,
				Size:    1824,
				ModTime: now.Add(-2 * time.Hour),
			},
		}
	}

	// Empty for unknown paths
	return []allocationspartials.FileEntry{}
}

// GetMockFileContent returns mock file content for testing.
func (p *Provider) GetMockFileContent(path string) (content string, size int64, isText bool) {
	switch {
	case strings.HasSuffix(path, ".stdout.0"):
		return `[2026-01-26 10:00:00] Starting application...
[2026-01-26 10:00:01] Loading configuration from /local/config.json
[2026-01-26 10:00:01] Configuration loaded successfully
[2026-01-26 10:00:02] Initializing database connection
[2026-01-26 10:00:02] Database connected: localhost:5432
[2026-01-26 10:00:03] Starting HTTP server on :8080
[2026-01-26 10:00:03] Server is ready to accept connections
[2026-01-26 10:05:15] GET /health - 200 OK (2ms)
[2026-01-26 10:05:30] GET /api/v1/users - 200 OK (45ms)
[2026-01-26 10:06:00] POST /api/v1/tasks - 201 Created (123ms)`, 4523, true

	case strings.HasSuffix(path, ".stderr.0"):
		return `[WARN] 2026-01-26 10:00:02 - Connection pool size not specified, using default: 10
[WARN] 2026-01-26 10:05:45 - Slow query detected: SELECT * FROM tasks (took 234ms)
[ERROR] 2026-01-26 10:07:12 - Failed to parse request body: invalid JSON`, 1256, true

	case strings.HasSuffix(path, "config.json"):
		return `{
  "app": {
    "name": "web-frontend",
    "version": "1.2.3",
    "environment": "production"
  },
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "timeout": "30s"
  },
  "database": {
    "host": "localhost",
    "port": 5432,
    "name": "appdb"
  }
}`, 2048, true

	case strings.HasSuffix(path, "run.sh"):
		return `#!/bin/bash
set -e

echo "Starting application..."
exec /usr/local/bin/app serve --config /local/config.json`, 512, true

	case strings.HasSuffix(path, "state.json"):
		return `{
  "status": "healthy",
  "last_check": "2026-01-26T10:05:00Z",
  "uptime_seconds": 7200
}`, 892, true

	case strings.HasSuffix(path, "api_key"):
		return `sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`, 64, true

	case strings.HasSuffix(path, "cert.pem"):
		return `-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUXXXXXXXXXXXXXXXXXXXXXXXXXXXX
...certificate content...
-----END CERTIFICATE-----`, 1824, true

	case strings.HasSuffix(path, "index.db"):
		return "[binary data]", 102400, false

	default:
		return "File content not available", 0, true
	}
}

package mock

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/open-wander/wander/ui-htmx/handlers"
	"github.com/open-wander/wander/ui-htmx/templates/partials/jobspartials"
)

// GetJobs returns mock job summary data.
func (p *Provider) GetJobs() []handlers.JobSummary {
	now := time.Now()

	return []handlers.JobSummary{
		{
			ID:        "web-frontend",
			Name:      "web-frontend",
			Status:    "running",
			Type:      "service",
			Priority:  50,
			Submitted: now.Add(-24 * time.Hour),
		},
		{
			ID:        "api-gateway",
			Name:      "api-gateway",
			Status:    "running",
			Type:      "service",
			Priority:  75,
			Submitted: now.Add(-48 * time.Hour),
		},
		{
			ID:        "data-processor",
			Name:      "data-processor",
			Status:    "pending",
			Type:      "batch",
			Priority:  30,
			Submitted: now.Add(-1 * time.Hour),
		},
		{
			ID:        "cache-service",
			Name:      "cache-service",
			Status:    "running",
			Type:      "service",
			Priority:  60,
			Submitted: now.Add(-72 * time.Hour),
		},
		{
			ID:        "backup-job",
			Name:      "backup-job",
			Status:    "dead",
			Type:      "batch",
			Priority:  20,
			Submitted: now.Add(-120 * time.Hour),
		},
		{
			ID:        "metrics-collector",
			Name:      "metrics-collector",
			Status:    "running",
			Type:      "system",
			Priority:  80,
			Submitted: now.Add(-96 * time.Hour),
		},
		{
			ID:        "log-shipper",
			Name:      "log-shipper",
			Status:    "running",
			Type:      "system",
			Priority:  70,
			Submitted: now.Add(-36 * time.Hour),
		},
		{
			ID:        "batch-processor",
			Name:      "batch-processor",
			Status:    "complete",
			Type:      "batch",
			Priority:  40,
			Submitted: now.Add(-200 * time.Hour),
		},
	}
}

// GetJobDetail returns mock job detail data.
func (p *Provider) GetJobDetail(jobID string) jobspartials.JobDetail {
	return jobspartials.JobDetail{
		ID:          jobID,
		Name:        "example-job-" + jobID,
		Status:      "running",
		Type:        "service",
		Priority:    50,
		Datacenters: []string{"dc1", "dc2"},
		TaskGroups: []jobspartials.TaskGroupSummary{
			{
				Name:  "web",
				Count: 3,
				Tasks: []string{"nginx", "redis"},
			},
			{
				Name:  "cache",
				Count: 2,
				Tasks: []string{"redis"},
			},
		},
	}
}

// GetJobVersions returns mock job version data.
func (p *Provider) GetJobVersions(jobID string) []jobspartials.JobVersion {
	now := time.Now()

	return []jobspartials.JobVersion{
		{
			Version:    4,
			Stable:     true,
			SubmitTime: now.Add(-2 * time.Hour),
			Active:     true,
			Diff:       "/ui/jobs/" + jobID + "/versions/4/diff",
		},
		{
			Version:    3,
			Stable:     true,
			SubmitTime: now.Add(-24 * time.Hour),
			Active:     false,
			Diff:       "/ui/jobs/" + jobID + "/versions/3/diff",
		},
		{
			Version:    2,
			Stable:     false,
			SubmitTime: now.Add(-48 * time.Hour),
			Active:     false,
			Diff:       "/ui/jobs/" + jobID + "/versions/2/diff",
		},
		{
			Version:    1,
			Stable:     true,
			SubmitTime: now.Add(-72 * time.Hour),
			Active:     false,
			Diff:       "/ui/jobs/" + jobID + "/versions/1/diff",
		},
		{
			Version:    0,
			Stable:     false,
			SubmitTime: now.Add(-96 * time.Hour),
			Active:     false,
			Diff:       "", // No diff for version 0 (initial version)
		},
	}
}

// GetJobDefinition returns mock job definition data.
func (p *Provider) GetJobDefinition(jobID string) jobspartials.JobDefinition {
	// Mock HCL job definition
	hclContent := `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 50

  # Update strategy for rolling deployments
  update {
    max_parallel      = 1
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 0
  }

  # Task group for web servers
  group "web" {
    count = 3

    # Restart policy on task failure
    restart {
      attempts = 3
      interval = "5m"
      delay    = "25s"
      mode     = "delay"
    }

    # Network configuration
    network {
      port "http" {
        to = 8080
      }
      port "metrics" {
        to = 9090
      }
    }

    # Service registration for Consul
    service {
      name = "` + jobID + `-web"
      port = "http"
      tags = ["urlprefix-/"]

      check {
        name     = "health"
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "2s"
      }
    }

    # Main web task
    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]

        volumes = [
          "local/nginx.conf:/etc/nginx/nginx.conf",
        ]
      }

      # Resource requirements
      resources {
        cpu    = 500
        memory = 256
      }

      # Template for nginx config
      template {
        data = <<EOF
events {
  worker_connections 1024;
}

http {
  server {
    listen 8080;

    location / {
      root /usr/share/nginx/html;
      index index.html;
    }

    location /health {
      return 200 "healthy\n";
      add_header Content-Type text/plain;
    }
  }
}
EOF
        destination = "local/nginx.conf"
      }

      # Environment variables
      env {
        ENVIRONMENT = "production"
        LOG_LEVEL   = "info"
      }
    }

    # Redis sidecar task
    task "redis" {
      driver = "docker"

      config {
        image = "redis:7-alpine"
        ports = ["metrics"]
      }

      resources {
        cpu    = 250
        memory = 128
      }

      lifecycle {
        hook    = "prestart"
        sidecar = true
      }
    }
  }

  # Constraints
  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  constraint {
    operator  = "distinct_hosts"
    value     = "true"
  }
}`

	return jobspartials.JobDefinition{
		JobID:    jobID,
		Content:  hclContent,
		Format:   "hcl",
		Version:  4,
		ReadOnly: false,
	}
}

// GetJobDefinitionForVersion returns mock job definition for a specific version.
// This simulates version differences for demonstration purposes.
func (p *Provider) GetJobDefinitionForVersion(jobID string, version int) string {
	// Base definition
	base := `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 50

  update {
    max_parallel      = 1
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 0
  }

  group "web" {
    count = 3

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`

	// Simulate changes based on version
	switch version {
	case 0:
		// Initial version - minimal config
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 1

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.20-alpine"
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}`
	case 1:
		// Version 1 - added network config
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 2

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.20-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 256
        memory = 128
      }
    }
  }
}`
	case 2:
		// Version 2 - increased resources
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 2

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`
	case 3:
		// Version 3 - added second datacenter
		return `job "` + jobID + `" {
  region      = "global"
  datacenters = ["dc1", "dc2"]
  type        = "service"
  priority    = 50

  group "web" {
    count = 2

    network {
      port "http" {
        to = 8080
      }
    }

    task "nginx" {
      driver = "docker"

      config {
        image = "nginx:1.21-alpine"
        ports = ["http"]
      }

      resources {
        cpu    = 500
        memory = 256
      }
    }
  }
}`
	default:
		// Latest version (4+) - added update stanza and increased count
		return base
	}
}

// GetJobAllocations returns mock allocation data for a specific job.
func (p *Provider) GetJobAllocations(jobID string, page, perPage int) jobspartials.AllocationsTabData {
	now := time.Now()

	// Generate mock allocations
	allAllocations := p.GenerateMockAllocations(jobID, now)

	// Calculate pagination
	total := len(allAllocations)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	// Apply pagination
	start := (page - 1) * perPage
	end := start + perPage
	if start >= total {
		start = 0
		end = 0
	}
	if end > total {
		end = total
	}

	var paginatedAllocs []jobspartials.JobAllocation
	if start < total {
		paginatedAllocs = allAllocations[start:end]
	}

	return jobspartials.AllocationsTabData{
		JobID:       jobID,
		Allocations: paginatedAllocs,
		Pagination: jobspartials.JobAllocationsPagination{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}
}

// GenerateMockAllocations generates mock allocation data for testing.
func (p *Provider) GenerateMockAllocations(jobID string, now time.Time) []jobspartials.JobAllocation {
	// Task groups and statuses for variation
	taskGroups := []string{"web", "api", "cache", "worker"}
	statuses := []string{"running", "running", "running", "pending", "complete", "failed"}
	nodes := []struct {
		id   string
		name string
	}{
		{"node-1a2b3c4d", "worker-01.dc1"},
		{"node-2b3c4d5e", "worker-02.dc1"},
		{"node-3c4d5e6f", "worker-03.dc1"},
		{"node-4d5e6f7g", "worker-04.dc2"},
		{"node-5e6f7g8h", "worker-05.dc2"},
	}

	// Generate 12 allocations for this job
	allocations := make([]jobspartials.JobAllocation, 0, 12)

	// Use a deterministic seed based on job ID for consistent mock data
	r := rand.New(rand.NewSource(hashString(jobID)))

	for i := 0; i < 12; i++ {
		// Generate a pseudo-random allocation ID
		allocID := generateMockID(r)
		shortID := allocID[:8]

		// Select mock data
		tgIndex := r.Intn(len(taskGroups))
		statusIndex := r.Intn(len(statuses))
		nodeIndex := r.Intn(len(nodes))

		// Generate resource usage (random but realistic)
		cpuMax := 500 + r.Intn(500)
		cpuUsed := int(float64(cpuMax) * (0.2 + r.Float64()*0.7))
		memMax := 256 + r.Intn(768)
		memUsed := int(float64(memMax) * (0.3 + r.Float64()*0.6))

		// Vary creation time
		hoursAgo := r.Intn(168) + 1 // 1 to 168 hours (1 week)

		alloc := jobspartials.JobAllocation{
			ID:        allocID,
			ShortID:   shortID,
			TaskGroup: taskGroups[tgIndex],
			NodeID:    nodes[nodeIndex].id,
			NodeName:  nodes[nodeIndex].name,
			Status:    statuses[statusIndex],
			CPU:       cpuUsed,
			CPUMax:    cpuMax,
			Memory:    memUsed,
			MemoryMax: memMax,
			Created:   now.Add(-time.Duration(hoursAgo) * time.Hour),
		}

		allocations = append(allocations, alloc)
	}

	// Sort by created time (newest first)
	sort.Slice(allocations, func(i, j int) bool {
		return allocations[i].Created.After(allocations[j].Created)
	})

	return allocations
}

// hashString creates a simple hash from a string for deterministic random seeding.
func hashString(s string) int64 {
	var h int64
	for _, c := range s {
		h = h*31 + int64(c)
	}
	return h
}

// generateMockID generates a mock UUID-like ID.
func generateMockID(r *rand.Rand) string {
	const chars = "0123456789abcdef"
	id := make([]byte, 36)
	for i := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			id[i] = '-'
		} else {
			id[i] = chars[r.Intn(len(chars))]
		}
	}
	return string(id)
}

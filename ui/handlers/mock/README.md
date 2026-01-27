# Mock Data Package

This package provides mock data for testing UI handlers without requiring a live Nomad cluster.

## Structure

- `mock.go` - Main interface definition and provider
- `jobs.go` - Job and task group mock data
- `allocations.go` - Allocation, task, and file system mock data
- `clients.go` - Client/node and server mock data

## Usage

Create a mock provider and use it in handlers:

```go
package handlers

import "github.com/open-wander/wander/ui/handlers/mock"

// Create a provider
provider := mock.NewProvider()

// Get mock jobs
jobs := provider.GetJobs()

// Get job detail
detail := provider.GetJobDetail("web-frontend")

// Get allocations
allocs := provider.GetAllocations()
```

## Interface

The `DataProvider` interface defines all methods for accessing mock data:

### Jobs
- `GetJobs()` - List of job summaries
- `GetJobDetail(jobID)` - Job detail with task groups
- `GetJobVersions(jobID)` - Job version history
- `GetJobDefinition(jobID)` - HCL job definition
- `GetJobDefinitionForVersion(jobID, version)` - Version-specific definition
- `GetJobAllocations(jobID, page, perPage)` - Paginated allocations for a job
- `GenerateMockAllocations(jobID, now)` - Generate deterministic mock allocations

### Allocations
- `GetAllocations()` - List of allocation summaries
- `GetAllocationDetail(allocID)` - Allocation detail with tasks and events
- `GetTasksForAllocation(allocID)` - Task names for an allocation
- `GetMockFiles(allocID, task, path)` - File system entries
- `GetMockFileContent(path)` - File content for preview/download

### Clients
- `GetClients()` - List of client/node summaries
- `GetClientDetail(clientID)` - Client detail with resources, allocations, and events

## Mock Data Characteristics

### Jobs
- 8 sample jobs with varying types (service, batch, system)
- Multiple task groups per job
- Realistic HCL definitions
- Version history with simulated diffs

### Allocations
- 12 sample allocations across different jobs and nodes
- Varied statuses (running, pending, complete, failed)
- Task events and resource usage
- Deterministic random generation based on job ID

### Clients
- 6 sample client nodes across 2 datacenters
- Different node classes (compute, storage, high-memory, gpu)
- Resource usage history (CPU/memory over 24 hours)
- Driver information and node events

## Future Enhancements

This mock data layer can be replaced with real Nomad API calls by:

1. Creating a new implementation of `DataProvider` that calls Nomad API
2. Injecting the provider into handlers via dependency injection
3. Switching providers based on configuration (dev mode vs production)

## Testing

The mock package is designed to be easily testable and provides consistent data for UI development and testing.

Note: Due to an import cycle in the policies templates, the package cannot currently be compiled with `go build`. The mock data is syntactically correct and will compile once the import cycle is resolved.

package mock

import (
	"time"

	"github.com/open-wander/wander/ui/handlers"
	"github.com/open-wander/wander/ui/templates/partials/allocationspartials"
	"github.com/open-wander/wander/ui/templates/partials/clientspartials"
	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
)

// DataProvider defines the interface for mock data access.
// This allows for easy testing and future replacement with real API calls.
type DataProvider interface {
	// Jobs
	GetJobs() []jobspartials.JobSummary
	GetJobDetail(jobID string) jobspartials.JobDetail
	GetJobVersions(jobID string) []jobspartials.JobVersion
	GetJobDefinition(jobID string) jobspartials.JobDefinition
	GetJobDefinitionForVersion(jobID string, version int) string
	GetJobAllocations(jobID string, page, perPage int) jobspartials.AllocationsTabData
	GenerateMockAllocations(jobID string, now time.Time) []jobspartials.JobAllocation

	// Allocations
	GetAllocations() []handlers.AllocationSummary
	GetAllocationDetail(allocID string) allocationspartials.AllocationDetail
	GetTasksForAllocation(allocID string) []string
	GetMockFiles(allocID, task, path string) []allocationspartials.FileEntry
	GetMockFileContent(path string) (content string, size int64, isText bool)

	// Clients
	GetClients() []clientspartials.ClientSummary
	GetClientDetail(clientID string) clientspartials.ClientDetail
}

// Provider is the default implementation of DataProvider.
type Provider struct{}

// NewProvider creates a new mock data provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Verify that Provider implements DataProvider
var _ DataProvider = (*Provider)(nil)

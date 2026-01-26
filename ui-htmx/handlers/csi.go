package handlers

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/ui-htmx/templates/pages/csi"
	"github.com/open-wander/wander/ui-htmx/templates/partials/csipartials"
)

// VolumeSummary represents a summary view of a CSI volume for list display.
// This structure will eventually be populated from the Nomad API.
type VolumeSummary struct {
	ID            string
	Name          string
	PluginID      string
	Provider      string
	Schedulable   bool
	AccessMode    string
	AttachMode    string
	Allocations   int
	Created       time.Time
}

// CSIHandler handles requests for the CSI volumes pages.
type CSIHandler struct {
	handler *Handler
}

// NewCSIHandler creates a new CSI handler.
func NewCSIHandler(h *Handler) *CSIHandler {
	return &CSIHandler{
		handler: h,
	}
}

// ServeHTTP handles requests to the CSI pages.
// It routes to the volumes list page.
func (h *CSIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/csi/volumes exactly (list page)
	if path == "/ui/csi/volumes" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.VolumesList(w, r)
		return
	}

	http.NotFound(w, r)
}

// VolumesList handles the CSI volumes list page
func (h *CSIHandler) VolumesList(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("csi volumes list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	query := r.URL.Query()
	pluginFilter := query.Get("plugin")
	providerFilter := query.Get("provider")
	schedulableFilter := query.Get("schedulable")
	searchQuery := query.Get("search")
	sortColumn := query.Get("sort")
	sortDir := query.Get("dir")
	pageStr := query.Get("page")
	perPageStr := query.Get("per_page")

	// Default sort
	if sortColumn == "" {
		sortColumn = "name"
	}
	if sortDir == "" {
		sortDir = "asc"
	}

	// Default pagination
	page := 1
	perPage := 25
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if pp, err := strconv.Atoi(perPageStr); err == nil && pp > 0 && pp <= 100 {
		perPage = pp
	}

	// Get all volumes
	allVolumes := h.getVolumesList()

	// Extract filter options from all volumes (before filtering)
	filterOptions := h.extractFilterOptions(allVolumes)

	// Apply filters
	filteredVolumes := h.filterVolumes(allVolumes, pluginFilter, providerFilter, schedulableFilter, searchQuery)

	// Apply sorting
	h.sortVolumes(filteredVolumes, sortColumn, sortDir)

	// Calculate pagination
	total := len(filteredVolumes)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	// Apply pagination
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if endIdx > total {
		endIdx = total
	}
	if startIdx > total {
		startIdx = total
	}

	paginatedVolumes := filteredVolumes[startIdx:endIdx]

	// Convert to partial type
	volumesList := make([]csipartials.VolumeSummary, len(paginatedVolumes))
	for i, v := range paginatedVolumes {
		volumesList[i] = csipartials.VolumeSummary{
			ID:          v.ID,
			Name:        v.Name,
			PluginID:    v.PluginID,
			Provider:    v.Provider,
			Schedulable: v.Schedulable,
			AccessMode:  v.AccessMode,
			AttachMode:  v.AttachMode,
			Allocations: v.Allocations,
			Created:     v.Created,
		}
	}

	// Build props
	props := csipartials.ListProps{
		Volumes: volumesList,
		FilterOptions: csipartials.FilterOptions{
			Plugins:   filterOptions.Plugins,
			Providers: filterOptions.Providers,
		},
		Filters: csipartials.ListFilters{
			Plugin:      pluginFilter,
			Provider:    providerFilter,
			Schedulable: schedulableFilter,
			Search:      searchQuery,
		},
		Pagination: csipartials.PaginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
		SortColumn: sortColumn,
		SortDir:    sortDir,
	}

	// Check if this is an HTMX request for table update only
	if IsHTMXFromRequest(r) && r.Header.Get("HX-Target") == "volumes-table-container" {
		err := RenderPartial(w, r, csipartials.VolumesTable(props))
		if err != nil {
			h.handler.logger.Error("failed to render volumes table", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Render the volumes list page
	err := Render(w, r,
		csi.VolumesList(props),                // Full page with layout
		csipartials.VolumesListContent(props), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render csi volumes list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// FilterOptions holds the available filter options
type CSIFilterOptions struct {
	Plugins   []string
	Providers []string
}

// extractFilterOptions extracts unique values for filter dropdowns
func (h *CSIHandler) extractFilterOptions(volumes []VolumeSummary) CSIFilterOptions {
	pluginSet := make(map[string]bool)
	providerSet := make(map[string]bool)

	for _, v := range volumes {
		pluginSet[v.PluginID] = true
		providerSet[v.Provider] = true
	}

	// Convert to sorted slices
	plugins := make([]string, 0, len(pluginSet))
	for p := range pluginSet {
		plugins = append(plugins, p)
	}
	sort.Strings(plugins)

	providers := make([]string, 0, len(providerSet))
	for p := range providerSet {
		providers = append(providers, p)
	}
	sort.Strings(providers)

	return CSIFilterOptions{
		Plugins:   plugins,
		Providers: providers,
	}
}

// filterVolumes applies filters to the volumes list
func (h *CSIHandler) filterVolumes(volumes []VolumeSummary, plugin, provider, schedulable, search string) []VolumeSummary {
	filtered := make([]VolumeSummary, 0, len(volumes))

	for _, v := range volumes {
		// Plugin filter
		if plugin != "" && v.PluginID != plugin {
			continue
		}

		// Provider filter
		if provider != "" && v.Provider != provider {
			continue
		}

		// Schedulable filter
		if schedulable != "" {
			isSchedulable := schedulable == "true"
			if v.Schedulable != isSchedulable {
				continue
			}
		}

		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(v.ID), searchLower) &&
				!strings.Contains(strings.ToLower(v.Name), searchLower) &&
				!strings.Contains(strings.ToLower(v.PluginID), searchLower) &&
				!strings.Contains(strings.ToLower(v.Provider), searchLower) {
				continue
			}
		}

		filtered = append(filtered, v)
	}

	return filtered
}

// sortVolumes sorts the volumes list by the specified column
func (h *CSIHandler) sortVolumes(volumes []VolumeSummary, column, dir string) {
	sort.Slice(volumes, func(i, j int) bool {
		var less bool

		switch column {
		case "id":
			less = volumes[i].ID < volumes[j].ID
		case "name":
			less = volumes[i].Name < volumes[j].Name
		case "plugin":
			less = volumes[i].PluginID < volumes[j].PluginID
		case "provider":
			less = volumes[i].Provider < volumes[j].Provider
		case "schedulable":
			less = volumes[i].Schedulable && !volumes[j].Schedulable
		case "accessmode":
			less = volumes[i].AccessMode < volumes[j].AccessMode
		case "allocations":
			less = volumes[i].Allocations < volumes[j].Allocations
		case "created":
			less = volumes[i].Created.Before(volumes[j].Created)
		default:
			less = volumes[i].Name < volumes[j].Name
		}

		if dir == "desc" {
			return !less
		}
		return less
	})
}

// getVolumesList returns mock CSI volume data.
// This will be replaced with actual Nomad API calls in the future.
func (h *CSIHandler) getVolumesList() []VolumeSummary {
	now := time.Now()

	return []VolumeSummary{
		{
			ID:          "vol-01-postgres-data",
			Name:        "postgres-data",
			PluginID:    "aws-ebs0",
			Provider:    "ebs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "single-node-writer",
			AttachMode:  "file-system",
			Allocations: 1,
			Created:     now.Add(-168 * time.Hour), // 7 days ago
		},
		{
			ID:          "vol-02-redis-cache",
			Name:        "redis-cache",
			PluginID:    "aws-ebs0",
			Provider:    "ebs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "single-node-writer",
			AttachMode:  "file-system",
			Allocations: 1,
			Created:     now.Add(-120 * time.Hour), // 5 days ago
		},
		{
			ID:          "vol-03-shared-assets",
			Name:        "shared-assets",
			PluginID:    "aws-efs0",
			Provider:    "efs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "multi-node-multi-writer",
			AttachMode:  "file-system",
			Allocations: 3,
			Created:     now.Add(-240 * time.Hour), // 10 days ago
		},
		{
			ID:          "vol-04-mysql-data",
			Name:        "mysql-data",
			PluginID:    "aws-ebs0",
			Provider:    "ebs.csi.aws.com",
			Schedulable: false,
			AccessMode:  "single-node-writer",
			AttachMode:  "file-system",
			Allocations: 1,
			Created:     now.Add(-336 * time.Hour), // 14 days ago
		},
		{
			ID:          "vol-05-app-logs",
			Name:        "app-logs",
			PluginID:    "aws-efs0",
			Provider:    "efs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "multi-node-reader-only",
			AttachMode:  "file-system",
			Allocations: 5,
			Created:     now.Add(-72 * time.Hour), // 3 days ago
		},
		{
			ID:          "vol-06-elasticsearch-data",
			Name:        "elasticsearch-data",
			PluginID:    "ceph-rbd0",
			Provider:    "rbd.csi.ceph.com",
			Schedulable: true,
			AccessMode:  "single-node-writer",
			AttachMode:  "block-device",
			Allocations: 1,
			Created:     now.Add(-480 * time.Hour), // 20 days ago
		},
		{
			ID:          "vol-07-prometheus-metrics",
			Name:        "prometheus-metrics",
			PluginID:    "aws-ebs0",
			Provider:    "ebs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "single-node-writer",
			AttachMode:  "file-system",
			Allocations: 1,
			Created:     now.Add(-96 * time.Hour), // 4 days ago
		},
		{
			ID:          "vol-08-grafana-dashboards",
			Name:        "grafana-dashboards",
			PluginID:    "aws-efs0",
			Provider:    "efs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "multi-node-reader-only",
			AttachMode:  "file-system",
			Allocations: 2,
			Created:     now.Add(-192 * time.Hour), // 8 days ago
		},
		{
			ID:          "vol-09-backup-archive",
			Name:        "backup-archive",
			PluginID:    "ceph-rbd0",
			Provider:    "rbd.csi.ceph.com",
			Schedulable: false,
			AccessMode:  "single-node-writer",
			AttachMode:  "file-system",
			Allocations: 0,
			Created:     now.Add(-720 * time.Hour), // 30 days ago
		},
		{
			ID:          "vol-10-media-content",
			Name:        "media-content",
			PluginID:    "aws-efs0",
			Provider:    "efs.csi.aws.com",
			Schedulable: true,
			AccessMode:  "multi-node-multi-writer",
			AttachMode:  "file-system",
			Allocations: 4,
			Created:     now.Add(-144 * time.Hour), // 6 days ago
		},
	}
}

package handlers

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/csi"
	"github.com/open-wander/wander/ui/templates/partials/csipartials"
)

// VolumeSummary represents a summary view of a CSI volume for list display.
// This structure will eventually be populated from the Nomad API.
type VolumeSummary struct {
	ID          string
	Name        string
	PluginID    string
	Provider    string
	Schedulable bool
	AccessMode  string
	AttachMode  string
	Allocations int
	Created     time.Time
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
// It routes to the volumes list page, volume detail pages, and plugins list page.
func (h *CSIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Handle /ui/csi/plugins exactly (list page)
	if path == "/ui/csi/plugins" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.PluginsList(w, r)
		return
	}

	// Handle /ui/csi/volumes exactly (list page)
	if path == "/ui/csi/volumes" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.VolumesList(w, r)
		return
	}

	// Handle /ui/csi/volumes/{id} (detail page)
	if strings.HasPrefix(path, "/ui/csi/volumes/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Extract volume ID from path
		volumeID := strings.TrimPrefix(path, "/ui/csi/volumes/")
		// Remove any trailing slashes or additional path segments
		if idx := strings.Index(volumeID, "/"); idx != -1 {
			volumeID = volumeID[:idx]
		}
		h.VolumeDetail(w, r, volumeID)
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
	namespace := query.Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// Check permissions - need csi-list-volume capability
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanListCSI(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to list CSI volumes",
			Capability: "csi-list-volume",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'csi-list-volume' capability",
				"This capability allows viewing CSI volume information",
			},
		})
		return
	}

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

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get all volumes from Nomad API
	allVolumes := h.getVolumesList(namespace, token)

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

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"CSI Volumes - Wander",
		"View and manage CSI volumes",
		"/ui/csi/volumes",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Volumes", URL: ""},
		},
	)

	// Render the volumes list page
	err := Render(w, r,
		csi.VolumesList(layoutProps, props),                // Full page with layout
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

// getVolumesList fetches CSI volumes from the Nomad API.
// Falls back to empty list if API is unavailable.
func (h *CSIHandler) getVolumesList(namespace, token string) []VolumeSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty volumes list")
		return []VolumeSummary{}
	}

	// Build query options with namespace and ACL token
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching CSI volumes from API",
		"namespace", namespace,
		"hasToken", token != "")

	// Fetch volumes from the API
	volumes, _, err := client.CSIVolumes().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch CSI volumes from Nomad API", "error", err)
		return []VolumeSummary{}
	}

	// Convert API response to our summary type
	result := make([]VolumeSummary, 0, len(volumes))
	for _, vol := range volumes {
		// Calculate total allocations (readers + writers)
		totalAllocs := vol.CurrentReaders + vol.CurrentWriters

		// Convert CreateIndex to a timestamp (approximate - uses current time as base)
		// Note: Nomad doesn't expose actual creation time, so we use ModifyIndex as proxy
		created := time.Now()
		if vol.CreateIndex > 0 {
			// Rough approximation: subtract some time based on index difference
			// This is not accurate but gives relative ordering
			created = created.Add(-time.Duration(vol.ModifyIndex-vol.CreateIndex) * time.Minute)
		}

		result = append(result, VolumeSummary{
			ID:          vol.ID,
			Name:        vol.Name,
			PluginID:    vol.PluginID,
			Provider:    vol.Provider,
			Schedulable: vol.Schedulable,
			AccessMode:  string(vol.AccessMode),
			AttachMode:  string(vol.AttachmentMode),
			Allocations: totalAllocs,
			Created:     created,
		})
	}

	return result
}

// VolumeDetail represents detailed information about a CSI volume.
// This will eventually be populated from the Nomad API.
type VolumeDetail struct {
	ID                  string
	Name                string
	Namespace           string
	PluginID            string
	Provider            string
	Schedulable         bool
	AccessMode          string
	AttachMode          string
	MountOptions        string
	Capacity            int64 // in bytes
	AllocationsCount    int
	ReadAllocsCount     int
	WriteAllocsCount    int
	Created             time.Time
	ExternalID          string
	ControllerRequired  bool
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
	Allocations         []AllocationInfo
}

// AllocationInfo represents basic allocation information for volume detail
type AllocationInfo struct {
	ID        string
	Name      string
	JobID     string
	TaskGroup string
	NodeID    string
	NodeName  string
	Status    string
	ReadOnly  bool
	Created   time.Time
}

// VolumeDetail handles the CSI volume detail page
func (h *CSIHandler) VolumeDetail(w http.ResponseWriter, r *http.Request, volumeID string) {
	// Log the request
	h.handler.logger.Debug("csi volume detail request",
		"path", r.URL.Path,
		"volumeID", volumeID,
		"htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	query := r.URL.Query()
	namespace := query.Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// Check permissions - need csi-read-volume capability
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadCSI(namespace) {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view CSI volume details",
			Capability: "csi-read-volume",
			Namespace:  namespace,
			Suggestions: []string{
				"Contact your administrator to request the 'csi-read-volume' capability",
				"This capability allows viewing detailed CSI volume information",
			},
		})
		return
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get volume details
	volume := h.getVolumeDetail(volumeID, namespace, token)
	if volume == nil {
		http.NotFound(w, r)
		return
	}

	// Convert to partial type
	volumeDetail := csipartials.VolumeDetail{
		ID:                  volume.ID,
		Name:                volume.Name,
		Namespace:           volume.Namespace,
		PluginID:            volume.PluginID,
		Provider:            volume.Provider,
		Schedulable:         volume.Schedulable,
		AccessMode:          volume.AccessMode,
		AttachMode:          volume.AttachMode,
		MountOptions:        volume.MountOptions,
		Capacity:            volume.Capacity,
		AllocationsCount:    volume.AllocationsCount,
		ReadAllocsCount:     volume.ReadAllocsCount,
		WriteAllocsCount:    volume.WriteAllocsCount,
		Created:             volume.Created,
		ExternalID:          volume.ExternalID,
		ControllerRequired:  volume.ControllerRequired,
		ControllersHealthy:  volume.ControllersHealthy,
		ControllersExpected: volume.ControllersExpected,
		NodesHealthy:        volume.NodesHealthy,
		NodesExpected:       volume.NodesExpected,
	}

	// Convert allocations
	volumeDetail.Allocations = make([]csipartials.AllocationInfo, len(volume.Allocations))
	for i, a := range volume.Allocations {
		volumeDetail.Allocations[i] = csipartials.AllocationInfo{
			ID:        a.ID,
			Name:      a.Name,
			JobID:     a.JobID,
			TaskGroup: a.TaskGroup,
			NodeID:    a.NodeID,
			NodeName:  a.NodeName,
			Status:    a.Status,
			ReadOnly:  a.ReadOnly,
			Created:   a.Created,
		}
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		volumeDetail.Name+" - CSI Volume - Wander",
		"CSI Volume detail for "+volumeDetail.Name,
		"/ui/csi/volumes/"+volumeID,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Volumes", URL: "/ui/csi/volumes"},
			{Label: volumeDetail.Name, URL: ""},
		},
	)

	// Render the volume detail page
	err := Render(w, r,
		csi.VolumeDetail(layoutProps, volumeDetail),                // Full page with layout
		csipartials.VolumeDetailContent(volumeDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render csi volume detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getVolumeDetail fetches CSI volume details from the Nomad API.
// Returns nil if volume not found or API is unavailable.
func (h *CSIHandler) getVolumeDetail(volumeID, namespace, token string) *VolumeDetail {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, cannot fetch volume detail")
		return nil
	}

	// Build query options with namespace and ACL token
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching CSI volume detail from API",
		"volumeID", volumeID,
		"namespace", namespace,
		"hasToken", token != "")

	// Fetch volume from the API
	vol, _, err := client.CSIVolumes().Info(volumeID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch CSI volume from Nomad API",
			"volumeID", volumeID,
			"namespace", namespace,
			"error", err)
		return nil
	}

	// Convert timestamps - use CreateIndex as approximate creation time
	created := time.Now()
	if vol.CreateIndex > 0 {
		// Rough approximation: subtract some time based on index difference
		// This is not accurate but gives relative ordering
		created = created.Add(-time.Duration(vol.ModifyIndex-vol.CreateIndex) * time.Minute)
	}

	// Format mount options
	mountOptions := ""
	if vol.MountOptions != nil {
		if vol.MountOptions.FSType != "" {
			mountOptions = "fs_type=" + vol.MountOptions.FSType
		}
		if len(vol.MountOptions.MountFlags) > 0 {
			if mountOptions != "" {
				mountOptions += ", "
			}
			mountOptions += strings.Join(vol.MountOptions.MountFlags, ",")
		}
	}
	if mountOptions == "" {
		mountOptions = "defaults"
	}

	// Convert allocations from the Allocations list
	allocations := make([]AllocationInfo, 0, len(vol.Allocations))
	for _, alloc := range vol.Allocations {
		// Determine if this allocation is a reader or writer
		isReadOnly := false
		if vol.ReadAllocs != nil {
			_, isReadOnly = vol.ReadAllocs[alloc.ID]
		}

		// Convert allocation timestamps
		allocCreated := time.Time{}
		if alloc.CreateTime > 0 {
			allocCreated = time.Unix(0, alloc.CreateTime)
		}

		allocations = append(allocations, AllocationInfo{
			ID:        alloc.ID,
			Name:      alloc.Name,
			JobID:     alloc.JobID,
			TaskGroup: alloc.TaskGroup,
			NodeID:    alloc.NodeID,
			NodeName:  alloc.NodeName,
			Status:    alloc.ClientStatus,
			ReadOnly:  isReadOnly,
			Created:   allocCreated,
		})
	}

	// Calculate allocation counts
	readCount := len(vol.ReadAllocs)
	writeCount := len(vol.WriteAllocs)
	totalCount := len(vol.Allocations)

	return &VolumeDetail{
		ID:                  vol.ID,
		Name:                vol.Name,
		Namespace:           vol.Namespace,
		PluginID:            vol.PluginID,
		Provider:            vol.Provider,
		Schedulable:         vol.Schedulable,
		AccessMode:          string(vol.AccessMode),
		AttachMode:          string(vol.AttachmentMode),
		MountOptions:        mountOptions,
		Capacity:            vol.Capacity,
		AllocationsCount:    totalCount,
		ReadAllocsCount:     readCount,
		WriteAllocsCount:    writeCount,
		Created:             created,
		ExternalID:          vol.ExternalID,
		ControllerRequired:  vol.ControllerRequired,
		ControllersHealthy:  vol.ControllersHealthy,
		ControllersExpected: vol.ControllersExpected,
		NodesHealthy:        vol.NodesHealthy,
		NodesExpected:       vol.NodesExpected,
		Allocations:         allocations,
	}
}

// PluginSummary represents a summary view of a CSI plugin for list display.
// This structure will eventually be populated from the Nomad API.
type PluginSummary struct {
	ID                  string
	Provider            string
	ControllerRequired  bool
	ControllersHealthy  int
	ControllersExpected int
	NodesHealthy        int
	NodesExpected       int
}

// PluginsList handles the CSI plugins list page
func (h *CSIHandler) PluginsList(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("csi plugins list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Check permissions - CSI plugins require list capability
	// Note: CSI plugins are cluster-wide resources, not namespaced
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanListCSI("default") {
		h.handler.RenderForbidden(w, r, ForbiddenError{
			Message:    "You do not have permission to view CSI plugins",
			Capability: "csi-list-volume",
			Namespace:  "default",
			Suggestions: []string{
				"Contact your administrator to request the 'csi-list-volume' capability",
				"This capability allows viewing CSI plugin information",
			},
		})
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	searchQuery := query.Get("search")
	sortColumn := query.Get("sort")
	sortDir := query.Get("dir")

	// Default sort
	if sortColumn == "" {
		sortColumn = "id"
	}
	if sortDir == "" {
		sortDir = "asc"
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get all plugins from Nomad API
	allPlugins := h.getPluginsList(token)

	// Apply search filter
	filteredPlugins := h.filterPlugins(allPlugins, searchQuery)

	// Apply sorting
	h.sortPlugins(filteredPlugins, sortColumn, sortDir)

	// Convert to partial type
	pluginsList := make([]csipartials.PluginSummary, len(filteredPlugins))
	for i, p := range filteredPlugins {
		pluginsList[i] = csipartials.PluginSummary{
			ID:                  p.ID,
			Provider:            p.Provider,
			ControllerRequired:  p.ControllerRequired,
			ControllersHealthy:  p.ControllersHealthy,
			ControllersExpected: p.ControllersExpected,
			NodesHealthy:        p.NodesHealthy,
			NodesExpected:       p.NodesExpected,
		}
	}

	// Build props
	props := csipartials.PluginsListProps{
		Plugins:    pluginsList,
		Search:     searchQuery,
		SortColumn: sortColumn,
		SortDir:    sortDir,
	}

	// Check if this is an HTMX request for table update only
	if IsHTMXFromRequest(r) && r.Header.Get("HX-Target") == "plugins-table-container" {
		err := RenderPartial(w, r, csipartials.PluginsTable(props))
		if err != nil {
			h.handler.logger.Error("failed to render plugins table", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"CSI Plugins - Wander",
		"View CSI storage plugins",
		"/ui/csi/plugins",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Plugins", URL: ""},
		},
	)

	// Render the plugins list page
	err := Render(w, r,
		csi.PluginsList(layoutProps, props),                // Full page with layout
		csipartials.PluginsListContent(props), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render csi plugins list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// filterPlugins applies search filter to the plugins list
func (h *CSIHandler) filterPlugins(plugins []PluginSummary, search string) []PluginSummary {
	if search == "" {
		return plugins
	}

	filtered := make([]PluginSummary, 0, len(plugins))
	searchLower := strings.ToLower(search)

	for _, p := range plugins {
		if strings.Contains(strings.ToLower(p.ID), searchLower) ||
			strings.Contains(strings.ToLower(p.Provider), searchLower) {
			filtered = append(filtered, p)
		}
	}

	return filtered
}

// sortPlugins sorts the plugins list by the specified column
func (h *CSIHandler) sortPlugins(plugins []PluginSummary, column, dir string) {
	sort.Slice(plugins, func(i, j int) bool {
		var less bool

		switch column {
		case "id":
			less = plugins[i].ID < plugins[j].ID
		case "provider":
			less = plugins[i].Provider < plugins[j].Provider
		case "controllers":
			less = plugins[i].ControllersHealthy < plugins[j].ControllersHealthy
		case "nodes":
			less = plugins[i].NodesHealthy < plugins[j].NodesHealthy
		default:
			less = plugins[i].ID < plugins[j].ID
		}

		if dir == "desc" {
			return !less
		}
		return less
	})
}

// getPluginsList fetches CSI plugins from the Nomad API.
// Falls back to empty list if API is unavailable.
// Note: CSI plugins are cluster-wide resources, not namespaced.
func (h *CSIHandler) getPluginsList(token string) []PluginSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty plugins list")
		return []PluginSummary{}
	}

	// Build query options with ACL token (no namespace - plugins are cluster-wide)
	queryOpts := &api.QueryOptions{}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching CSI plugins from API",
		"hasToken", token != "")

	// Fetch plugins from the API
	plugins, _, err := client.CSIPlugins().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch CSI plugins from Nomad API", "error", err)
		return []PluginSummary{}
	}

	// Convert API response to our summary type
	result := make([]PluginSummary, 0, len(plugins))
	for _, plugin := range plugins {
		result = append(result, PluginSummary{
			ID:                  plugin.ID,
			Provider:            plugin.Provider,
			ControllerRequired:  plugin.ControllerRequired,
			ControllersHealthy:  plugin.ControllersHealthy,
			ControllersExpected: plugin.ControllersExpected,
			NodesHealthy:        plugin.NodesHealthy,
			NodesExpected:       plugin.NodesExpected,
		})
	}

	return result
}

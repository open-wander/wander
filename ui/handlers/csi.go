package handlers

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
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

	// Handle /ui/csi/plugins/{id} and /ui/csi/plugins/{id}/allocations.
	if strings.HasPrefix(path, "/ui/csi/plugins/") {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		remainder := strings.Trim(strings.TrimPrefix(path, "/ui/csi/plugins/"), "/")
		segments := strings.Split(remainder, "/")
		pluginID, err := url.PathUnescape(segments[0])
		if err != nil || pluginID == "" || len(segments) > 2 || (len(segments) == 2 && segments[1] != "allocations") {
			http.NotFound(w, r)
			return
		}
		if len(segments) == 2 {
			h.PluginAllocations(w, r, pluginID)
			return
		}
		h.PluginDetail(w, r, pluginID)
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

var errNomadClientUnavailable = errors.New("Nomad client unavailable")

// VolumesList handles the CSI volumes list page
func (h *CSIHandler) VolumesList(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("csi volumes list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	query := r.URL.Query()
	namespace := GetNamespaceFromRequest(r)

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
	page := positiveBoundedInt(query.Get("page"), 1, math.MaxInt)
	perPage := positiveBoundedInt(query.Get("per_page"), 25, 100)

	// Default sort
	if sortColumn == "" {
		sortColumn = "name"
	}
	if sortDir == "" {
		sortDir = "asc"
	}

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get all volumes from Nomad API
	allVolumes := h.getVolumesList(r, namespace, token)

	// Extract filter options from all volumes (before filtering)
	filterOptions := h.extractFilterOptions(allVolumes)

	// Apply filters
	filteredVolumes := h.filterVolumes(allVolumes, pluginFilter, providerFilter, schedulableFilter, searchQuery)

	// Apply sorting
	h.sortVolumes(filteredVolumes, sortColumn, sortDir)

	// Calculate pagination
	total := len(filteredVolumes)
	page, totalPages, startIdx, endIdx := paginationBounds(total, page, perPage)

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
	layoutProps := h.handler.BaseLayoutProps(r,
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
		csi.VolumesList(layoutProps, props),   // Full page with layout
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
func (h *CSIHandler) getVolumesList(r *http.Request, namespace, token string) []VolumeSummary {
	client := h.handler.NomadClientForRequest(r)
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

		result = append(result, VolumeSummary{
			ID:          vol.ID,
			Name:        vol.Name,
			PluginID:    vol.PluginID,
			Provider:    vol.Provider,
			Schedulable: vol.Schedulable,
			AccessMode:  string(vol.AccessMode),
			AttachMode:  string(vol.AttachmentMode),
			Allocations: totalAllocs,
			Created:     time.Time{},
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

	// Resolve the namespace from request-scoped selection.
	namespace := GetNamespaceFromRequest(r)

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
	volume := h.getVolumeDetail(r, volumeID, namespace, token)
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
	layoutProps := h.handler.BaseLayoutProps(r,
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
		csi.VolumeDetail(layoutProps, volumeDetail),   // Full page with layout
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
func (h *CSIHandler) getVolumeDetail(r *http.Request, volumeID, namespace, token string) *VolumeDetail {
	client := h.handler.NomadClientForRequest(r)
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

	created := time.Time{}

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
	page := positiveBoundedInt(query.Get("page"), 1, math.MaxInt)
	perPage := positiveBoundedInt(query.Get("per_page"), 25, 100)

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
	allPlugins, err := h.getPluginsList(r, token)
	if err != nil {
		h.renderPluginListError(w, r, err)
		return
	}

	// Apply search filter
	filteredPlugins := h.filterPlugins(allPlugins, searchQuery)

	// Apply sorting
	h.sortPlugins(filteredPlugins, sortColumn, sortDir)
	total := len(filteredPlugins)
	page, _, start, end := paginationBounds(total, page, perPage)
	paginatedPlugins := filteredPlugins[start:end]

	// Convert to partial type
	pluginsList := make([]csipartials.PluginSummary, len(paginatedPlugins))
	for i, p := range paginatedPlugins {
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
		Page:       page,
		PerPage:    perPage,
		Total:      total,
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
	layoutProps := h.handler.BaseLayoutProps(r,
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
	err = Render(w, r,
		csi.PluginsList(layoutProps, props),   // Full page with layout
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
	sort.SliceStable(plugins, func(i, j int) bool {
		comparison := 0
		switch column {
		case "id":
			comparison = strings.Compare(plugins[i].ID, plugins[j].ID)
		case "provider":
			comparison = strings.Compare(plugins[i].Provider, plugins[j].Provider)
		case "controllers":
			comparison = plugins[i].ControllersHealthy - plugins[j].ControllersHealthy
		case "nodes":
			comparison = plugins[i].NodesHealthy - plugins[j].NodesHealthy
		default:
			comparison = strings.Compare(plugins[i].ID, plugins[j].ID)
		}
		if comparison == 0 {
			comparison = strings.Compare(plugins[i].ID, plugins[j].ID)
		}
		if dir == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

// getPluginsList fetches CSI plugins from the Nomad API.
// Note: CSI plugins are cluster-wide resources, not namespaced.
func (h *CSIHandler) getPluginsList(r *http.Request, token string) ([]PluginSummary, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, errNomadClientUnavailable
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
		return nil, err
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

	return result, nil
}

func (h *CSIHandler) renderPluginListError(w http.ResponseWriter, r *http.Request, err error) {
	if apiResponseHasStatus(err, http.StatusForbidden) {
		h.renderPluginForbidden(w, r, "You do not have permission to view CSI plugins")
		return
	}
	h.handler.logger.Error("failed to fetch CSI plugins", "error", err)
	h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{
		Message: "Wander could not load CSI plugins from the cluster.",
	})
}

// The handler and templates share one view model so API values are mapped once.
type PluginDetail = csipartials.PluginDetail
type PluginAllocation = csipartials.PluginAllocation

type pluginAllocationsPage struct {
	Plugin      PluginDetail
	Allocations []PluginAllocation
	Type        string
	Health      string
	Sort        string
	Direction   string
	Page        int
	PerPage     int
	Total       int
	TotalPages  int
}

// PluginDetail renders the CSI plugin overview and its recent allocations.
func (h *CSIHandler) PluginDetail(w http.ResponseWriter, r *http.Request, pluginID string) {
	plugin, ok := h.loadPluginDetail(w, r, pluginID)
	if !ok {
		return
	}

	layoutProps := h.handler.BaseLayoutProps(r,
		plugin.ID+" - CSI Plugin - Wander",
		"CSI plugin detail for "+plugin.ID,
		"/ui/csi/plugins/"+url.PathEscape(plugin.ID),
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Plugins", URL: "/ui/csi/plugins"},
			{Label: plugin.ID, URL: ""},
		},
	)

	h.renderPluginPage(w, r, pluginID, "detail",
		csi.PluginDetail(layoutProps, *plugin),
		csipartials.PluginDetailContent(*plugin),
	)
}

// PluginAllocations renders the filterable and paginated CSI allocation view.
func (h *CSIHandler) PluginAllocations(w http.ResponseWriter, r *http.Request, pluginID string) {
	plugin, ok := h.loadPluginDetail(w, r, pluginID)
	if !ok {
		return
	}

	page := buildPluginAllocationsPage(*plugin, r.URL.Query())
	view := toPluginAllocationsView(page)
	layoutProps := h.handler.BaseLayoutProps(r,
		plugin.ID+" allocations - CSI Plugin - Wander",
		"CSI plugin allocations for "+plugin.ID,
		"/ui/csi/plugins/"+url.PathEscape(plugin.ID)+"/allocations",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Storage", URL: ""},
			{Label: "Plugins", URL: "/ui/csi/plugins"},
			{Label: plugin.ID, URL: "/ui/csi/plugins/" + url.PathEscape(plugin.ID)},
			{Label: "Allocations", URL: ""},
		},
	)

	h.renderPluginPage(w, r, pluginID, "allocations",
		csi.PluginAllocations(layoutProps, view),
		csipartials.PluginAllocationsContent(view),
	)
}

func (h *CSIHandler) loadPluginDetail(w http.ResponseWriter, r *http.Request, pluginID string) (*PluginDetail, bool) {
	plugin, err := h.getPluginDetail(r, pluginID, GetTokenFromRequest(r))
	if err != nil {
		h.renderPluginError(w, r, pluginID, err)
		return nil, false
	}
	return plugin, true
}

func (h *CSIHandler) renderPluginPage(w http.ResponseWriter, r *http.Request, pluginID, page string, full, partial templ.Component) {
	if err := Render(w, r, full, partial); err != nil {
		h.handler.logger.Error("failed to render CSI plugin page", "pluginID", pluginID, "page", page, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *CSIHandler) getPluginDetail(r *http.Request, pluginID, token string) (*PluginDetail, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, errNomadClientUnavailable
	}

	query := resourceQueryOptions(token)
	plugin, _, err := client.CSIPlugins().Info(pluginID, query)
	if err != nil {
		return nil, err
	}
	if plugin == nil {
		return nil, errors.New("Nomad returned an empty CSI plugin response")
	}

	allocations := make(map[string]*api.AllocationListStub, len(plugin.Allocations))
	for _, allocation := range plugin.Allocations {
		if allocation != nil {
			allocations[allocation.ID] = allocation
		}
	}

	return &PluginDetail{
		ID:                  plugin.ID,
		Provider:            plugin.Provider,
		Version:             plugin.Version,
		ControllerRequired:  plugin.ControllerRequired,
		ControllersHealthy:  plugin.ControllersHealthy,
		ControllersExpected: plugin.ControllersExpected,
		NodesHealthy:        plugin.NodesHealthy,
		NodesExpected:       plugin.NodesExpected,
		Controllers:         joinPluginAllocations("controller", plugin.Controllers, allocations),
		Nodes:               joinPluginAllocations("node", plugin.Nodes, allocations),
	}, nil
}

func joinPluginAllocations(kind string, records map[string]*api.CSIInfo, allocations map[string]*api.AllocationListStub) []PluginAllocation {
	result := make([]PluginAllocation, 0, len(records))
	for nodeID, info := range records {
		if info == nil {
			continue
		}
		item := PluginAllocation{
			Type:              kind,
			AllocationID:      info.AllocID,
			NodeID:            nodeID,
			Healthy:           info.Healthy,
			HealthDescription: info.HealthDescription,
			Updated:           info.UpdateTime,
		}
		if allocation := allocations[info.AllocID]; allocation != nil {
			item.AllocationKnown = true
			item.NodeID = firstNonEmpty(allocation.NodeID, item.NodeID)
			item.NodeName = allocation.NodeName
			item.JobID = allocation.JobID
			item.Namespace = allocation.Namespace
			item.TaskGroup = allocation.TaskGroup
			item.JobVersion = allocation.JobVersion
			item.ClientStatus = allocation.ClientStatus
			item.Created = unixNanoTime(allocation.CreateTime)
			item.Modified = unixNanoTime(allocation.ModifyTime)
			item.CPU, item.MemoryMB, item.ResourcesKnown = allocatedCapacity(allocation.AllocatedResources)
		}
		result = append(result, item)
	}
	sortPluginAllocations(result, "modified", "desc")
	return result
}

func allocatedCapacity(resources *api.AllocatedResources) (cpu, memoryMB int64, known bool) {
	if resources == nil {
		return 0, 0, false
	}
	for _, task := range resources.Tasks {
		if task == nil {
			continue
		}
		cpu += task.Cpu.CpuShares
		memoryMB += task.Memory.MemoryMB
	}
	return cpu, memoryMB, true
}

func buildPluginAllocationsPage(plugin PluginDetail, query url.Values) pluginAllocationsPage {
	typeFilter := query.Get("type")
	if typeFilter != "controller" && typeFilter != "node" {
		typeFilter = ""
	}
	healthFilter := query.Get("healthy")
	if healthFilter != "true" && healthFilter != "false" {
		healthFilter = ""
	}
	sortColumn := query.Get("sort")
	if sortColumn != "health" && sortColumn != "id" {
		sortColumn = "modified"
	}
	direction := query.Get("dir")
	if direction != "asc" {
		direction = "desc"
	}
	page := positiveBoundedInt(query.Get("page"), 1, math.MaxInt)
	perPage := positiveBoundedInt(query.Get("per_page"), 25, 100)

	combined := make([]PluginAllocation, 0, len(plugin.Controllers)+len(plugin.Nodes))
	if typeFilter != "node" {
		combined = append(combined, plugin.Controllers...)
	}
	if typeFilter != "controller" {
		combined = append(combined, plugin.Nodes...)
	}
	if healthFilter != "" {
		wantHealthy := healthFilter == "true"
		filtered := combined[:0]
		for _, allocation := range combined {
			if allocation.Healthy == wantHealthy {
				filtered = append(filtered, allocation)
			}
		}
		combined = filtered
	}
	sortPluginAllocations(combined, sortColumn, direction)

	total := len(combined)
	page, totalPages, start, end := paginationBounds(total, page, perPage)

	return pluginAllocationsPage{
		Plugin: plugin, Allocations: combined[start:end], Type: typeFilter,
		Health: healthFilter, Sort: sortColumn, Direction: direction,
		Page: page, PerPage: perPage, Total: total, TotalPages: totalPages,
	}
}

func sortPluginAllocations(allocations []PluginAllocation, column, direction string) {
	sort.SliceStable(allocations, func(i, j int) bool {
		comparison := 0
		switch column {
		case "health":
			if allocations[i].Healthy != allocations[j].Healthy {
				if allocations[i].Healthy {
					comparison = 1
				} else {
					comparison = -1
				}
			}
		case "id":
			comparison = strings.Compare(allocations[i].AllocationID, allocations[j].AllocationID)
		default:
			comparison = allocations[i].Updated.Compare(allocations[j].Updated)
		}
		if comparison == 0 {
			comparison = strings.Compare(allocations[i].AllocationID, allocations[j].AllocationID)
		}
		if direction == "desc" {
			return comparison > 0
		}
		return comparison < 0
	})
}

func positiveBoundedInt(raw string, fallback, maximum int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return fallback
	}
	return min(value, maximum)
}

func paginationBounds(total, page, perPage int) (boundedPage, totalPages, start, end int) {
	totalPages = (total + perPage - 1) / perPage
	boundedPage = page
	if totalPages == 0 {
		boundedPage = 1
	} else if boundedPage > totalPages {
		boundedPage = totalPages
	}
	start = min((boundedPage-1)*perPage, total)
	end = min(start+perPage, total)
	return boundedPage, totalPages, start, end
}

func unixNanoTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(0, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (h *CSIHandler) renderPluginError(w http.ResponseWriter, r *http.Request, pluginID string, err error) {
	switch {
	case apiResponseHasStatus(err, http.StatusNotFound):
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     "The requested CSI plugin was not found.",
			RequestPath: r.URL.Path,
		})
	case apiResponseHasStatus(err, http.StatusForbidden):
		h.renderPluginForbidden(w, r, "You do not have permission to view CSI plugin details")
	default:
		h.handler.logger.Error("failed to fetch CSI plugin", "pluginID", pluginID, "error", err)
		h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{
			Message: "Wander could not load the CSI plugin from the cluster.",
		})
	}
}

func (h *CSIHandler) renderPluginForbidden(w http.ResponseWriter, r *http.Request, message string) {
	h.handler.RenderForbidden(w, r, ForbiddenError{
		Message:    message,
		Capability: "csi-list-volume",
		Namespace:  "default",
	})
}

func toPluginAllocationsView(page pluginAllocationsPage) csipartials.PluginAllocationsProps {
	return csipartials.PluginAllocationsProps{
		Plugin: page.Plugin, Allocations: page.Allocations,
		Type: page.Type, Health: page.Health, Sort: page.Sort, Direction: page.Direction,
		Page: page.Page, PerPage: page.PerPage, Total: page.Total, TotalPages: page.TotalPages,
	}
}

package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui-htmx/templates/components"
	"github.com/open-wander/wander/ui-htmx/templates/pages/deployments"
	"github.com/open-wander/wander/ui-htmx/templates/partials/deploymentspartials"
)

// DeploymentsHandler handles requests for the deployments panel.
type DeploymentsHandler struct {
	handler *Handler
}

// NewDeploymentsHandler creates a new deployments handler.
func NewDeploymentsHandler(h *Handler) *DeploymentsHandler {
	return &DeploymentsHandler{
		handler: h,
	}
}

// ServeHTTP handles GET requests to the deployments endpoint.
func (h *DeploymentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path

	// Handle /ui/deployments exactly (list page)
	if path == "/ui/deployments" {
		h.List(w, r)
		return
	}

	// Handle /ui/deployments/{id} (detail page)
	if strings.HasPrefix(path, "/ui/deployments/") {
		h.Detail(w, r)
		return
	}

	http.NotFound(w, r)
}

// DeploymentSummary represents a summary view of a deployment for processing.
type DeploymentSummary struct {
	ID                string
	JobID             string
	JobVersion        uint64
	Namespace         string
	Status            string
	StatusDescription string
	IsMultiregion     bool
	CreateIndex       uint64
	ModifyIndex       uint64
	CreateTime        time.Time
	TaskGroups        map[string]*api.DeploymentState
}

// List handles the deployments list request and returns the partial content
func (h *DeploymentsHandler) List(w http.ResponseWriter, r *http.Request) {
	// Log the request
	h.handler.logger.Debug("deployments list request", "path", r.URL.Path, "htmx", IsHTMXFromRequest(r))

	// Parse query parameters
	query := r.URL.Query()
	namespace := query.Get("namespace")
	if namespace == "" {
		namespace = "default"
	}
	statusFilter := query.Get("status")
	jobFilter := query.Get("job")
	searchQuery := query.Get("search")
	sortColumn := query.Get("sort")
	sortDir := query.Get("dir")
	pageStr := query.Get("page")
	perPageStr := query.Get("per_page")

	// Default sorting
	if sortColumn == "" {
		sortColumn = "created"
	}
	if sortDir == "" {
		sortDir = "desc"
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

	// Get all deployments with server-side filtering
	allDeployments := h.getDeploymentsList(namespace, token, statusFilter, jobFilter)

	// Apply client-side search filter (status/job already filtered server-side)
	filteredDeployments := h.filterDeployments(allDeployments, "", "", searchQuery)

	// Apply sorting
	h.sortDeployments(filteredDeployments, sortColumn, sortDir)

	// Calculate pagination
	total := len(filteredDeployments)
	totalPages := (total + perPage - 1) / perPage // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	// Apply pagination
	startIdx := (page - 1) * perPage
	endIdx := startIdx + perPage
	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > total {
		endIdx = total
	}
	if startIdx > total {
		startIdx = total
	}

	paginatedDeployments := filteredDeployments[startIdx:endIdx]

	// Convert to template format
	deploymentsList := make([]deploymentspartials.DeploymentSummary, len(paginatedDeployments))
	for i, deploy := range paginatedDeployments {
		// Get short ID (first 8 characters)
		shortID := deploy.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// Calculate time ago from CreateTime
		timeAgo := formatTimeAgo(deploy.CreateTime)

		// Calculate deployment metrics from task groups
		var totalDesired, totalPlaced, totalHealthy, totalUnhealthy int
		for _, tg := range deploy.TaskGroups {
			totalDesired += tg.DesiredTotal
			totalPlaced += tg.PlacedAllocs
			totalHealthy += tg.HealthyAllocs
			totalUnhealthy += tg.UnhealthyAllocs
		}

		deploymentsList[i] = deploymentspartials.DeploymentSummary{
			ID:                deploy.ID,
			ShortID:           shortID,
			JobID:             deploy.JobID,
			JobName:           deploy.JobID, // JobName is same as JobID in the API
			JobVersion:        deploy.JobVersion,
			Status:            deploy.Status,
			StatusDescription: deploy.StatusDescription,
			IsMultiregion:     deploy.IsMultiregion,
			TimeAgo:           timeAgo,
			DesiredTotal:      totalDesired,
			PlacedAllocs:      totalPlaced,
			HealthyAllocs:     totalHealthy,
			UnhealthyAllocs:   totalUnhealthy,
		}
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		"Deployments - Wander",
		"View Nomad deployments",
		"/ui/deployments",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Deployments", URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		deployments.List(layoutProps, deploymentsList),       // Full page with layout
		deploymentspartials.ListContent(deploymentsList),     // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render deployments list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getDeploymentsList fetches deployments from the Nomad API with server-side filtering.
// Falls back to empty list if API is unavailable.
func (h *DeploymentsHandler) getDeploymentsList(namespace, token, statusFilter, jobFilter string) []DeploymentSummary {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available, returning empty deployments list")
		return []DeploymentSummary{}
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
	}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Build Nomad filter expression for server-side filtering
	var filters []string
	if statusFilter != "" {
		filters = append(filters, fmt.Sprintf("Status == %q", statusFilter))
	}
	if jobFilter != "" {
		filters = append(filters, fmt.Sprintf("JobID == %q", jobFilter))
	}
	if len(filters) > 0 {
		queryOpts.Filter = strings.Join(filters, " and ")
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching deployments from API",
		"namespace", namespace,
		"filter", queryOpts.Filter,
		"hasToken", token != "",
	)

	// Fetch deployments from the API
	deploys, _, err := client.Deployments().List(queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch deployments from Nomad API", "error", err)
		return []DeploymentSummary{}
	}

	h.handler.logger.Debug("fetched deployments from API", "count", len(deploys))

	// Convert API response to our summary type
	result := make([]DeploymentSummary, 0, len(deploys))
	for _, deploy := range deploys {
		// Estimate create time from CreateIndex (not perfect but reasonable)
		// In production, you might want to fetch individual deployments for exact timestamps
		createTime := time.Now().Add(-time.Duration(deploy.ModifyIndex-deploy.CreateIndex) * time.Second)

		result = append(result, DeploymentSummary{
			ID:                deploy.ID,
			JobID:             deploy.JobID,
			JobVersion:        deploy.JobVersion,
			Namespace:         deploy.Namespace,
			Status:            deploy.Status,
			StatusDescription: deploy.StatusDescription,
			IsMultiregion:     deploy.IsMultiregion,
			CreateIndex:       deploy.CreateIndex,
			ModifyIndex:       deploy.ModifyIndex,
			CreateTime:        createTime,
			TaskGroups:        deploy.TaskGroups,
		})
	}

	return result
}

// filterDeployments applies client-side filtering to the deployments list.
// Note: status and job filters are already applied server-side.
func (h *DeploymentsHandler) filterDeployments(deploys []DeploymentSummary, status, job, search string) []DeploymentSummary {
	if search == "" && status == "" && job == "" {
		return deploys
	}

	searchLower := strings.ToLower(strings.TrimSpace(search))
	result := make([]DeploymentSummary, 0, len(deploys))

	for _, deploy := range deploys {
		// Apply status filter if provided (usually already handled server-side)
		if status != "" && deploy.Status != status {
			continue
		}

		// Apply job filter if provided (usually already handled server-side)
		if job != "" && deploy.JobID != job {
			continue
		}

		// Apply search filter across multiple fields
		if searchLower != "" {
			idLower := strings.ToLower(deploy.ID)
			jobLower := strings.ToLower(deploy.JobID)
			statusLower := strings.ToLower(deploy.Status)
			descLower := strings.ToLower(deploy.StatusDescription)

			if !strings.Contains(idLower, searchLower) &&
				!strings.Contains(jobLower, searchLower) &&
				!strings.Contains(statusLower, searchLower) &&
				!strings.Contains(descLower, searchLower) {
				continue
			}
		}

		result = append(result, deploy)
	}

	return result
}

// sortDeployments sorts the deployments list by the specified column
func (h *DeploymentsHandler) sortDeployments(deploys []DeploymentSummary, column, dir string) {
	sort.Slice(deploys, func(i, j int) bool {
		var less bool

		switch column {
		case "id":
			less = deploys[i].ID < deploys[j].ID
		case "job":
			less = deploys[i].JobID < deploys[j].JobID
		case "version":
			less = deploys[i].JobVersion < deploys[j].JobVersion
		case "status":
			less = deploys[i].Status < deploys[j].Status
		case "created":
			less = deploys[i].CreateTime.Before(deploys[j].CreateTime)
		default:
			// Default to sorting by created time
			less = deploys[i].CreateTime.Before(deploys[j].CreateTime)
		}

		if dir == "desc" {
			return !less
		}
		return less
	})
}

// Detail handles the deployment detail page
func (h *DeploymentsHandler) Detail(w http.ResponseWriter, r *http.Request) {
	// Extract deployment ID from path
	deploymentID := strings.TrimPrefix(r.URL.Path, "/ui/deployments/")
	deploymentID = strings.TrimSuffix(deploymentID, "/")

	// Log the request
	h.handler.logger.Debug("deployment detail request", "deploymentID", deploymentID, "htmx", IsHTMXFromRequest(r))

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get deployment detail from Nomad API
	deploymentDetail, found := h.getDeploymentDetail(deploymentID, token)
	if !found {
		http.NotFound(w, r)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(
		deploymentDetail.ShortID+" - Deployment Detail - Wander",
		"Deployment detail for "+deploymentDetail.ShortID,
		"/ui/deployments/"+deploymentID,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Deployments", URL: "/ui/deployments"},
			{Label: deploymentDetail.ShortID, URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err := Render(w, r,
		deployments.Detail(layoutProps, deploymentDetail),                // Full page with layout
		deploymentspartials.DetailContent(deploymentDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render deployment detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getDeploymentDetail fetches a specific deployment from the Nomad API.
func (h *DeploymentsHandler) getDeploymentDetail(deploymentID, token string) (deploymentspartials.DeploymentDetail, bool) {
	client := h.handler.NomadClient()
	if client == nil {
		h.handler.logger.Warn("no Nomad client available")
		return deploymentspartials.DeploymentDetail{}, false
	}

	// Build query options
	queryOpts := &api.QueryOptions{}

	// Set ACL token if available
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Log the request for debugging
	h.handler.logger.Debug("fetching deployment from API",
		"deploymentID", deploymentID,
		"hasToken", token != "",
	)

	// Fetch deployment from the API
	deploy, _, err := client.Deployments().Info(deploymentID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch deployment from Nomad API", "error", err)
		return deploymentspartials.DeploymentDetail{}, false
	}

	if deploy == nil {
		h.handler.logger.Debug("deployment not found", "deploymentID", deploymentID)
		return deploymentspartials.DeploymentDetail{}, false
	}

	// Get short ID (first 8 characters)
	shortID := deploy.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	// Calculate time from CreateIndex (not perfect but reasonable)
	createTime := time.Now().Add(-time.Duration(deploy.ModifyIndex-deploy.CreateIndex) * time.Second)
	createTimeAgo := formatTimeAgo(createTime)

	// Calculate modify time from ModifyIndex (not perfect but reasonable)
	modifyTime := time.Now()
	modifyTimeAgo := formatTimeAgo(modifyTime)

	// Convert task groups to detail format
	taskGroups := make([]deploymentspartials.TaskGroupStatus, 0, len(deploy.TaskGroups))
	for name, tg := range deploy.TaskGroups {
		taskGroups = append(taskGroups, deploymentspartials.TaskGroupStatus{
			Name:            name,
			AutoRevert:      tg.AutoRevert,
			Promoted:        tg.Promoted,
			DesiredCanaries: tg.DesiredCanaries,
			DesiredTotal:    tg.DesiredTotal,
			PlacedCanaries:  len(tg.PlacedCanaries),
			PlacedAllocs:    tg.PlacedAllocs,
			HealthyAllocs:   tg.HealthyAllocs,
			UnhealthyAllocs: tg.UnhealthyAllocs,
		})
	}

	// Sort task groups by name for consistent display
	sort.Slice(taskGroups, func(i, j int) bool {
		return taskGroups[i].Name < taskGroups[j].Name
	})

	return deploymentspartials.DeploymentDetail{
		ID:                deploy.ID,
		ShortID:           shortID,
		JobID:             deploy.JobID,
		JobVersion:        deploy.JobVersion,
		Namespace:         deploy.Namespace,
		Status:            deploy.Status,
		StatusDescription: deploy.StatusDescription,
		IsMultiregion:     deploy.IsMultiregion,
		CreateTime:        createTime,
		CreateTimeAgo:     createTimeAgo,
		ModifyTime:        modifyTime,
		ModifyTimeAgo:     modifyTimeAgo,
		TaskGroups:        taskGroups,
	}, true
}

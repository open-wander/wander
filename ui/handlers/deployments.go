package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/deployments"
	"github.com/open-wander/wander/ui/templates/partials/deploymentspartials"
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
		deploymentID, err := url.PathUnescape(strings.Trim(strings.TrimPrefix(path, "/ui/deployments/"), "/"))
		if err != nil || deploymentID == "" || strings.Contains(deploymentID, "/") {
			http.NotFound(w, r)
			return
		}
		h.Detail(w, r, deploymentID)
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
	namespace := GetNamespaceFromRequest(r)
	statusFilter := query.Get("status")
	jobFilter := query.Get("job")
	view := parseListViewOptions(query, "created", "desc")

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get all deployments with server-side filtering
	allDeployments, err := h.getDeploymentsList(r, namespace, token, statusFilter, jobFilter)
	if err != nil {
		h.renderDeploymentsError(w, r, namespace, "list deployments", err)
		return
	}

	// Apply client-side search filter (status/job already filtered server-side)
	filteredDeployments := h.filterDeployments(allDeployments, "", "", view.search)

	// Apply sorting
	h.sortDeployments(filteredDeployments, view.sort, view.dir)

	// Calculate pagination
	total := len(filteredDeployments)
	_, _, startIdx, endIdx := paginationBounds(total, view.page, view.perPage)

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
			if tg == nil {
				continue
			}
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
	layoutProps := h.handler.BaseLayoutProps(r,
		"Deployments - Wander",
		"View Wander deployments",
		"/ui/deployments",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Deployments", URL: ""},
		},
	)

	// Render full page or partial based on HTMX request
	err = Render(w, r,
		deployments.List(layoutProps, deploymentsList),   // Full page with layout
		deploymentspartials.ListContent(deploymentsList), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render deployments list", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getDeploymentsList fetches deployments from the Nomad API with server-side filtering.
func (h *DeploymentsHandler) getDeploymentsList(r *http.Request, namespace, token, statusFilter, jobFilter string) ([]DeploymentSummary, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available for deployments list")
		return nil, errNomadClientUnavailable
	}

	// Build query options with namespace
	queryOpts := &api.QueryOptions{
		Namespace: namespace,
		Region:    GetRegionFromRequest(r),
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
		return nil, err
	}

	h.handler.logger.Debug("fetched deployments from API", "count", len(deploys))

	// Convert API response to our summary type
	result := make([]DeploymentSummary, 0, len(deploys))
	for _, deploy := range deploys {
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
			CreateTime:        time.Time{},
			TaskGroups:        deploy.TaskGroups,
		})
	}

	return result, nil
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
		if dir == "desc" {
			return sortDeploymentLess(deploys[j], deploys[i], column)
		}
		return sortDeploymentLess(deploys[i], deploys[j], column)
	})
}

func sortDeploymentLess(left, right DeploymentSummary, column string) bool {
	switch column {
	case "id":
		return left.ID < right.ID
	case "job":
		return left.JobID < right.JobID
	case "version":
		return left.JobVersion < right.JobVersion
	case "status":
		return left.Status < right.Status
	default:
		return left.CreateIndex < right.CreateIndex
	}
}

// Detail handles the deployment detail page
func (h *DeploymentsHandler) Detail(w http.ResponseWriter, r *http.Request, deploymentID string) {
	// Log the request
	h.handler.logger.Debug("deployment detail request", "deploymentID", deploymentID, "htmx", IsHTMXFromRequest(r))

	// Get ACL token from request context
	token := GetTokenFromRequest(r)

	// Get deployment detail from Nomad API
	deploymentDetail, err := h.getDeploymentDetail(r, deploymentID, token)
	if err != nil {
		h.renderDeploymentsError(w, r, GetNamespaceFromRequest(r), "load deployment", err)
		return
	}

	// Build layout props
	layoutProps := h.handler.BaseLayoutProps(r,
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
	err = Render(w, r,
		deployments.Detail(layoutProps, deploymentDetail),   // Full page with layout
		deploymentspartials.DetailContent(deploymentDetail), // Partial content for HTMX requests
	)

	if err != nil {
		h.handler.logger.Error("failed to render deployment detail", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// getDeploymentDetail fetches a specific deployment from the Nomad API.
var errDeploymentNotFound = errors.New("deployment not found")

func (h *DeploymentsHandler) getDeploymentDetail(r *http.Request, deploymentID, token string) (deploymentspartials.DeploymentDetail, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Warn("no Nomad client available")
		return deploymentspartials.DeploymentDetail{}, errNomadClientUnavailable
	}

	// Build query options
	queryOpts := &api.QueryOptions{Namespace: GetNamespaceFromRequest(r), Region: GetRegionFromRequest(r)}

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
		return deploymentspartials.DeploymentDetail{}, err
	}

	if deploy == nil {
		h.handler.logger.Debug("deployment not found", "deploymentID", deploymentID)
		return deploymentspartials.DeploymentDetail{}, errDeploymentNotFound
	}

	// Get short ID (first 8 characters)
	shortID := deploy.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	createTime := time.Time{}
	createTimeAgo := formatTimeAgo(createTime)

	modifyTime := time.Time{}
	modifyTimeAgo := formatTimeAgo(modifyTime)

	// Convert task groups to detail format
	taskGroups := make([]deploymentspartials.TaskGroupStatus, 0, len(deploy.TaskGroups))
	for name, tg := range deploy.TaskGroups {
		if tg == nil {
			continue
		}
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
	}, nil
}

func (h *DeploymentsHandler) renderDeploymentsError(w http.ResponseWriter, r *http.Request, namespace, action string, err error) {
	notFound := errors.Is(err, errDeploymentNotFound) || apiResponseHasStatus(err, http.StatusNotFound)
	h.handler.renderJobScopedReadError(w, r, namespace, action, "deployment", notFound, err)
}

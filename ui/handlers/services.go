package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/a-h/templ"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/services"
	"github.com/open-wander/wander/ui/templates/partials/servicespartials"
)

// ServicesHandler renders read-only Nomad service registrations.
type ServicesHandler struct {
	handler *Handler
}

func (h *ServicesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	switch {
	case r.URL.Path == "/ui/services":
		h.List(w, r)
	case strings.HasPrefix(r.URL.Path, "/ui/services/"):
		h.Detail(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *ServicesHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := GetNamespaceFromRequest(r)
	if !h.canReadServices(w, r, namespace) {
		return
	}

	client := h.clientForRequest(w, r)
	if client == nil {
		return
	}

	response, _, err := client.Services().List(serviceQueryOptions(r, namespace))
	if err != nil {
		h.renderFetchError(w, r, err, "services", "namespace", namespace)
		return
	}

	serviceList := make([]servicespartials.ServiceSummary, 0)
	for _, group := range response {
		if group == nil {
			continue
		}
		for _, service := range group.Services {
			if service == nil {
				continue
			}
			serviceList = append(serviceList, servicespartials.ServiceSummary{
				Name:      service.ServiceName,
				Namespace: group.Namespace,
				Tags:      service.Tags,
			})
		}
	}
	sort.Slice(serviceList, func(i, j int) bool {
		if serviceList[i].Namespace == serviceList[j].Namespace {
			return serviceList[i].Name < serviceList[j].Name
		}
		return serviceList[i].Namespace < serviceList[j].Namespace
	})

	layoutProps := h.handler.BaseLayoutProps(r,
		"Services - Wander",
		"View service registrations",
		"/ui/services",
		[]components.Breadcrumb{{Label: "Home", URL: "/ui/"}, {Label: "Services"}},
	)
	h.renderPage(w, r,
		services.List(layoutProps, serviceList),
		servicespartials.ListContent(serviceList),
	)
}

func (h *ServicesHandler) Detail(w http.ResponseWriter, r *http.Request) {
	rawName := strings.TrimPrefix(r.URL.Path, "/ui/services/")
	if rawName == "" || strings.Contains(rawName, "/") {
		http.NotFound(w, r)
		return
	}
	serviceName, err := url.PathUnescape(rawName)
	if err != nil || serviceName == "" {
		http.NotFound(w, r)
		return
	}

	namespace := GetNamespaceFromRequest(r)
	if !h.canReadServices(w, r, namespace) {
		return
	}
	client := h.clientForRequest(w, r)
	if client == nil {
		return
	}

	registrations, _, err := client.Services().Get(serviceName, serviceQueryOptions(r, namespace))
	if err != nil {
		h.renderFetchError(w, r, err, "service registrations", "service", serviceName)
		return
	}
	if len(registrations) == 0 {
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     fmt.Sprintf("Service %q was not found.", serviceName),
			RequestPath: r.URL.Path,
			Suggestions: []string{`Go to <a href="/ui/services" class="underline">Services</a>`},
		})
		return
	}

	detail := servicespartials.ServiceDetail{Name: serviceName, Registrations: registrations}
	layoutProps := h.handler.BaseLayoutProps(r,
		serviceName+" - Service - Wander",
		"Service registrations for "+serviceName,
		"/ui/services/"+rawName,
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Services", URL: "/ui/services"},
			{Label: serviceName},
		},
	)
	h.renderPage(w, r,
		services.Detail(layoutProps, detail),
		servicespartials.DetailContent(detail),
	)
}

func (h *ServicesHandler) clientForRequest(w http.ResponseWriter, r *http.Request) *api.Client {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.RenderClusterUnavailable(w, r, ClusterUnavailableError{Message: "The Wander API client is unavailable."})
	}
	return client
}

func (h *ServicesHandler) renderFetchError(w http.ResponseWriter, r *http.Request, err error, resource, contextKey, contextValue string) {
	if h.handler.CheckClusterError(w, r, err) {
		return
	}
	h.handler.logger.Error("failed to fetch "+resource, "error", err, contextKey, contextValue)
	http.Error(w, "Failed to fetch "+resource+" from Wander API", http.StatusBadGateway)
}

func (h *ServicesHandler) renderPage(w http.ResponseWriter, r *http.Request, full, partial templ.Component) {
	if err := Render(w, r, full, partial); err != nil {
		h.handler.logger.Error("failed to render services page", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *ServicesHandler) canReadServices(w http.ResponseWriter, r *http.Request, namespace string) bool {
	permissions := GetPermissionsFromRequest(r)
	if permissions == nil || permissions.CanReadJob(namespace) {
		return true
	}
	h.handler.RenderForbidden(w, r, ForbiddenError{
		Message:    "You do not have permission to view service registrations",
		Capability: "read-job",
		Namespace:  namespace,
		Suggestions: []string{
			"Contact your administrator to request the 'read-job' capability",
		},
	})
	return false
}

func serviceQueryOptions(r *http.Request, namespace string) *api.QueryOptions {
	return &api.QueryOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}
}

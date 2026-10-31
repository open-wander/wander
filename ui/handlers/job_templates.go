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
	"github.com/open-wander/wander/ui/templates/pages/jobs"
	"github.com/open-wander/wander/ui/templates/partials/jobspartials"
)

const jobTemplatePrefix = "nomad/job-templates/"

type JobTemplatesHandler struct {
	handler *Handler
}

func NewJobTemplatesHandler(handler *Handler) *JobTemplatesHandler {
	return &JobTemplatesHandler{handler: handler}
}

func (h *JobTemplatesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/ui/jobs/run/templates" && r.Method == http.MethodGet:
		h.list(w, r, false)
	case path == "/ui/jobs/run/templates/new" && r.Method == http.MethodGet:
		h.showCreate(w, r)
	case path == "/ui/jobs/run/templates/new" && r.Method == http.MethodPost:
		h.create(w, r)
	case path == "/ui/jobs/run/templates/manage" && r.Method == http.MethodGet:
		h.list(w, r, true)
	case strings.HasPrefix(path, "/ui/jobs/run/templates/manage/") && r.Method == http.MethodPost:
		h.update(w, r)
	case strings.HasPrefix(path, "/ui/jobs/run/templates/manage/") && r.Method == http.MethodDelete:
		h.delete(w, r)
	default:
		if path == "/ui/jobs/run/templates" || path == "/ui/jobs/run/templates/new" ||
			path == "/ui/jobs/run/templates/manage" || strings.HasPrefix(path, "/ui/jobs/run/templates/manage/") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     "The job template page you requested could not be found.",
			RequestPath: r.URL.Path,
			Suggestions: []string{`Go to <a href="/ui/jobs/run/templates" class="underline">Job Templates</a>`},
		})
	}
}

func (h *JobTemplatesHandler) list(w http.ResponseWriter, r *http.Request, manage bool) {
	namespace := GetNamespaceFromRequest(r)
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanListVariables(namespace, jobTemplatePrefix+"*") {
		h.renderTemplateForbidden(w, r, "list", namespace, jobTemplatePrefix+"*")
		return
	}

	templates, err := h.fetchTemplates(r, namespace)
	if err != nil {
		if h.handler.CheckClusterError(w, r, err) {
			return
		}
		http.Error(w, "Failed to fetch job templates: "+err.Error(), http.StatusInternalServerError)
		return
	}

	currentPath := "/ui/jobs/run/templates"
	title := "Job Templates - Wander"
	label := "Templates"
	if manage {
		currentPath += "/manage"
		title = "Manage Job Templates - Wander"
		label = "Manage"
	}
	layoutProps := h.handler.BaseLayoutProps(r, title, "Choose and manage stored job templates", currentPath, []components.Breadcrumb{
		{Label: "Home", URL: "/ui/"},
		{Label: "Jobs", URL: "/ui/jobs"},
		{Label: "Run Job", URL: namespaceURL("/ui/jobs/run", namespace)},
		{Label: label, URL: ""},
	})
	props := jobspartials.JobTemplatesProps{Templates: templates, Namespace: namespace, Manage: manage}
	if err := Render(w, r, jobs.JobTemplates(layoutProps, props), jobspartials.JobTemplatesContent(props)); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *JobTemplatesHandler) showCreate(w http.ResponseWriter, r *http.Request) {
	namespace := GetNamespaceFromRequest(r)
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanWriteVariables(namespace, jobTemplatePrefix+"*") {
		h.renderTemplateForbidden(w, r, "write", namespace, jobTemplatePrefix+"*")
		return
	}
	layoutProps := h.handler.BaseLayoutProps(r, "Create Job Template - Wander", "Store a reusable HCL job template", "/ui/jobs/run/templates/new", []components.Breadcrumb{
		{Label: "Home", URL: "/ui/"},
		{Label: "Jobs", URL: "/ui/jobs"},
		{Label: "Templates", URL: namespaceURL("/ui/jobs/run/templates", namespace)},
		{Label: "Create", URL: ""},
	})
	props := jobspartials.JobTemplateCreateProps{Namespace: namespace}
	if err := Render(w, r, jobs.JobTemplateCreate(layoutProps, props), jobspartials.JobTemplateCreateContent(props)); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *JobTemplatesHandler) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	name, definition, namespace, ok := h.templateForm(w, r)
	if !ok {
		return
	}
	path := jobTemplatePrefix + name
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanWriteVariables(namespace, path) {
		h.renderTemplateForbidden(w, r, "write", namespace, path)
		return
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander client not available", http.StatusInternalServerError)
		return
	}
	created, _, err := client.Variables().CheckedCreate(&api.Variable{
		Namespace: namespace,
		Path:      path,
		Items:     api.VariableItems{"template": definition},
	}, &api.WriteOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)})
	if err != nil {
		http.Error(w, "Failed to create job template: "+err.Error(), http.StatusBadRequest)
		return
	}
	h.redirect(w, r, namespaceURL("/ui/jobs/run?template="+url.QueryEscape(templateName(created.Path)), created.Namespace), http.StatusCreated)
}

func (h *JobTemplatesHandler) update(w http.ResponseWriter, r *http.Request) {
	name, err := templateNameFromManagePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	definition := strings.TrimSpace(r.FormValue("template"))
	if definition == "" {
		http.Error(w, "Template definition is required", http.StatusBadRequest)
		return
	}
	namespace := GetNamespaceFromRequest(r)
	path := jobTemplatePrefix + name
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanWriteVariables(namespace, path) {
		h.renderTemplateForbidden(w, r, "write", namespace, path)
		return
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander client not available", http.StatusInternalServerError)
		return
	}
	query := &api.QueryOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}
	existing, _, err := client.Variables().Read(path, query)
	if err != nil {
		http.Error(w, "Failed to read job template: "+err.Error(), http.StatusNotFound)
		return
	}
	items := existing.Items
	if items == nil {
		items = make(api.VariableItems)
	}
	items["template"] = definition
	_, _, err = client.Variables().CheckedUpdate(&api.Variable{
		Namespace: namespace, Path: path, Items: items, ModifyIndex: existing.ModifyIndex,
	}, &api.WriteOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)})
	if err != nil {
		http.Error(w, "Failed to update job template: "+err.Error(), http.StatusBadRequest)
		return
	}
	h.redirect(w, r, namespaceURL("/ui/jobs/run/templates/manage", namespace), http.StatusOK)
}

func (h *JobTemplatesHandler) delete(w http.ResponseWriter, r *http.Request) {
	name, err := templateNameFromManagePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	namespace := GetNamespaceFromRequest(r)
	path := jobTemplatePrefix + name
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanDestroyVariables(namespace, path) {
		h.renderTemplateForbidden(w, r, "destroy", namespace, path)
		return
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander client not available", http.StatusInternalServerError)
		return
	}
	if _, err := client.Variables().Delete(path, &api.WriteOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}); err != nil {
		http.Error(w, "Failed to delete job template: "+err.Error(), http.StatusBadRequest)
		return
	}
	h.redirect(w, r, namespaceURL("/ui/jobs/run/templates/manage", namespace), http.StatusOK)
}

func (h *JobTemplatesHandler) fetchTemplates(r *http.Request, namespace string) ([]jobspartials.JobTemplate, error) {
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return nil, errors.New("Wander client not available")
	}
	query := &api.QueryOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)}
	metadata, _, err := client.Variables().PrefixList(jobTemplatePrefix, query)
	if err != nil {
		return nil, err
	}
	perms := GetPermissionsFromRequest(r)
	templates := make([]jobspartials.JobTemplate, 0, len(metadata))
	for _, item := range metadata {
		entry := jobspartials.JobTemplate{
			Name:        templateName(item.Path),
			Path:        item.Path,
			Modified:    time.Unix(0, item.ModifyTime),
			ModifyIndex: item.ModifyIndex,
		}
		if perms != nil && !perms.CanReadVariables(namespace, item.Path) {
			entry.Readable = false
			templates = append(templates, entry)
			continue
		}
		variable, _, readErr := client.Variables().Read(item.Path, query)
		if readErr != nil {
			return nil, readErr
		}
		entry.Readable = true
		entry.Definition, entry.Valid = variable.Items["template"]
		templates = append(templates, entry)
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	return templates, nil
}

func (h *JobTemplatesHandler) readTemplate(r *http.Request, namespace, name string) (string, error) {
	if err := validateTemplateName(name); err != nil {
		return "", err
	}
	path := jobTemplatePrefix + name
	perms := GetPermissionsFromRequest(r)
	if perms != nil && !perms.CanReadVariables(namespace, path) {
		return "", fmt.Errorf("permission denied: read access is required for job template %q", name)
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return "", errors.New("Wander client not available")
	}
	variable, _, err := client.Variables().Read(path, &api.QueryOptions{Namespace: namespace, AuthToken: GetTokenFromRequest(r)})
	if err != nil {
		return "", fmt.Errorf("job template %q could not be read: %w", name, err)
	}
	definition, ok := variable.Items["template"]
	if !ok || strings.TrimSpace(definition) == "" {
		return "", fmt.Errorf("job template %q is malformed: the template item is missing", name)
	}
	return definition, nil
}

func (h *JobTemplatesHandler) templateForm(w http.ResponseWriter, r *http.Request) (string, string, string, bool) {
	name := strings.TrimSpace(r.FormValue("name"))
	if err := validateTemplateName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return "", "", "", false
	}
	definition := strings.TrimSpace(r.FormValue("template"))
	if definition == "" {
		http.Error(w, "Template definition is required", http.StatusBadRequest)
		return "", "", "", false
	}
	namespace := strings.TrimSpace(r.FormValue("namespace"))
	if namespace == "" {
		namespace = GetNamespaceFromRequest(r)
	}
	return name, definition, namespace, true
}

func validateTemplateName(name string) error {
	if name == "" {
		return errors.New("Template name is required")
	}
	if strings.ContainsAny(name, "/\\ ") || name == "." || name == ".." {
		return errors.New("Template name must be a single path segment without spaces")
	}
	return nil
}

func templateName(path string) string {
	return strings.TrimPrefix(path, jobTemplatePrefix)
}

func templateNameFromManagePath(path string) (string, error) {
	encoded := strings.TrimPrefix(strings.TrimSuffix(path, "/"), "/ui/jobs/run/templates/manage/")
	name, err := url.PathUnescape(encoded)
	if err != nil {
		return "", errors.New("Invalid template name")
	}
	return name, validateTemplateName(name)
}

func namespaceURL(path, namespace string) string {
	parsed, err := url.Parse(path)
	if err != nil {
		return path
	}
	query := parsed.Query()
	query.Set("namespace", namespace)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (h *JobTemplatesHandler) redirect(w http.ResponseWriter, r *http.Request, destination string, status int) {
	w.Header().Set("HX-Redirect", destination)
	w.Header().Set("Location", destination)
	if IsHTMXFromRequest(r) {
		w.WriteHeader(status)
		return
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
}

func (h *JobTemplatesHandler) renderTemplateForbidden(w http.ResponseWriter, r *http.Request, capability, namespace, path string) {
	h.handler.RenderForbidden(w, r, ForbiddenError{
		Message:    "You do not have permission to " + capability + " job templates",
		Capability: "variables: " + capability,
		Namespace:  namespace,
		Suggestions: []string{
			"Request variable access for " + path,
			"Ensure you are using a valid ACL token with the required variable capability",
		},
	})
}

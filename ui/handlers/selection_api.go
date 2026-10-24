package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/open-wander/wander/api"
)

type selectionSpec struct {
	queryName     string
	cookieName    string
	label         string
	allowWildcard bool
}

var selectionSpecs = map[string]selectionSpec{
	"/api/namespace/select": {
		queryName:     "ns",
		cookieName:    WanderNamespaceCookie,
		label:         "namespace",
		allowWildcard: true,
	},
	"/api/region/select": {
		queryName:  "region",
		cookieName: WanderRegionCookie,
		label:      "region",
	},
}

// SelectionAPIHandler persists browser-scoped namespace and region choices.
type SelectionAPIHandler struct {
	handler *Handler
}

func NewSelectionAPIHandler(h *Handler) *SelectionAPIHandler {
	return &SelectionAPIHandler{handler: h}
}

func (h *SelectionAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	spec, ok := selectionSpecs[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	value := r.URL.Query().Get(spec.queryName)
	if !validSelectionValue(value, spec.allowWildcard) {
		http.Error(w, "Valid "+spec.label+" parameter required", http.StatusBadRequest)
		return
	}
	if status, err := h.validateSelection(r, spec, value); err != nil {
		h.handler.logger.Warn("selection rejected", spec.label, value, "error", err)
		http.Error(w, "Unknown or inaccessible "+spec.label, status)
		return
	}

	http.SetCookie(w, selectionCookie(r, spec.cookieName, value))
	h.handler.logger.Info(spec.label+" selected", spec.label, value)
	setToast(w, "Switched to "+spec.label+": "+value, "success")
	setSelectionNavigation(w, r, spec.label, value)
	w.WriteHeader(http.StatusOK)
}

func (h *SelectionAPIHandler) validateSelection(r *http.Request, spec selectionSpec, value string) (int, error) {
	if spec.allowWildcard && value == "*" {
		return 0, nil
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		return http.StatusServiceUnavailable, errors.New("Wander API client is unavailable")
	}

	var err error
	if spec.cookieName == WanderNamespaceCookie {
		_, _, err = client.Namespaces().Info(value, nil)
	} else {
		var regions []string
		regions, err = client.Regions().List()
		if err == nil && !containsSelection(regions, value) {
			return http.StatusBadRequest, errors.New("selection does not exist")
		}
	}
	if err == nil {
		return 0, nil
	}
	if IsClusterUnavailableError(err) {
		return http.StatusServiceUnavailable, err
	}
	var responseErr api.UnexpectedResponseError
	if errors.As(err, &responseErr) {
		switch responseErr.StatusCode() {
		case http.StatusForbidden:
			return http.StatusForbidden, err
		case http.StatusNotFound:
			return http.StatusBadRequest, err
		}
	}
	return http.StatusBadGateway, err
}

func containsSelection(values []string, selected string) bool {
	for _, value := range values {
		if value == selected {
			return true
		}
	}
	return false
}

func selectionCookie(r *http.Request, name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour) / time.Second),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	}
}

func setSelectionNavigation(w http.ResponseWriter, r *http.Request, key, value string) {
	currentURL := GetCurrentURL(r)
	parsed, err := url.Parse(currentURL)
	if err != nil || !strings.HasPrefix(parsed.Path, "/ui/") {
		w.Header().Set("HX-Refresh", "true")
		return
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	w.Header().Set("HX-Redirect", parsed.RequestURI())
}

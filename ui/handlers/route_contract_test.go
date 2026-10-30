package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var topLevelPageContracts = []string{
	"/ui/jobs",
	"/ui/jobs/run",
	"/ui/jobs/run/templates",
	"/ui/jobs/run/templates/new",
	"/ui/jobs/run/templates/manage",
	"/ui/allocations",
	"/ui/clients",
	"/ui/servers",
	"/ui/services",
	"/ui/topology",
	"/ui/variables",
	"/ui/variables/new",
	"/ui/namespaces",
	"/ui/access-control",
	"/ui/access-control/policies",
	"/ui/access-control/policies/new",
	"/ui/access-control/tokens",
	"/ui/access-control/tokens/new",
	"/ui/access-control/roles",
	"/ui/access-control/roles/new",
	"/ui/policies",
	"/ui/tokens",
	"/ui/tokens/new",
	"/ui/roles",
	"/ui/roles/new",
	"/ui/csi/plugins",
	"/ui/csi/volumes",
	"/ui/settings",
	"/ui/settings/tokens",
	"/ui/evaluations",
	"/ui/deployments",
}

func TestTopLevelPageRouteContracts(t *testing.T) {
	router, closeBackend := newRouteContractRouter(t)
	defer closeBackend()

	for _, path := range topLevelPageContracts {
		path := path
		t.Run(path, func(t *testing.T) {
			wantStatus := topLevelPageStatus(path)
			for _, htmx := range []bool{false, true} {
				mode := "full page"
				if htmx {
					mode = "HTMX"
				}
				t.Run(mode, func(t *testing.T) {
					request := httptest.NewRequest(http.MethodGet, path, nil)
					if htmx {
						request.Header.Set("HX-Request", "true")
					}
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					assertRouteStatus(t, response, http.MethodGet, path, wantStatus)
					hasDocument := strings.Contains(strings.ToLower(response.Body.String()), "<!doctype html")
					if hasDocument == htmx {
						t.Fatalf("GET %s HTMX=%t document shell = %t, want %t", path, htmx, hasDocument, !htmx)
					}
				})
			}
		})
	}
}

func TestTopLevelRoutesRejectUnsupportedMethods(t *testing.T) {
	router, closeBackend := newRouteContractRouter(t)
	defer closeBackend()

	assertRoutesHaveStatus(t, router, http.MethodPatch, http.StatusMethodNotAllowed, topLevelPageContracts)
}

func TestActionRoutesRejectUnsupportedMethods(t *testing.T) {
	router := NewRouter(NewHandler(nil))
	assertRoutesHaveStatus(t, router, http.MethodGet, http.StatusMethodNotAllowed, []string{
		"/api/jobs/example/stop",
		"/api/namespace/select",
		"/api/region/select",
		"/api/clients/example/eligibility",
		"/api/deployments/example/fail",
		"/api/v1/auth/clear-cache",
		"/ui/settings/oidc",
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/token-info", nil))
	assertRouteStatus(t, response, http.MethodPost, "/api/v1/auth/token-info", http.StatusMethodNotAllowed)
}

func TestDynamicUIRoutesRejectUnsupportedMethods(t *testing.T) {
	router := NewRouter(NewHandler(nil))
	assertRoutesHaveStatus(t, router, http.MethodPatch, http.StatusMethodNotAllowed, []string{
		"/ui/",
		"/ui/static/css/output.css",
		"/ui/search",
		"/ui/jobs/example",
		"/ui/jobs/example/plan",
		"/ui/jobs/example/run",
		"/ui/jobs/example/taskgroups/web/scale",
		"/ui/jobs/example/versions/1/revert",
		"/ui/allocations/example",
		"/ui/allocations/example/restart",
		"/ui/allocations/example/stop",
		"/ui/clients/example",
		"/ui/clients/example/monitor",
		"/ui/clients/example/monitor/stream",
		"/ui/clients/example/drain",
		"/ui/clients/example/drain/cancel",
		"/ui/servers/example",
		"/ui/servers/example/monitor",
		"/ui/servers/example/monitor/stream",
		"/ui/services/example",
		"/ui/topology/data",
		"/ui/variables/example",
		"/ui/access-control/policies/example",
		"/ui/access-control/tokens/example",
		"/ui/access-control/roles/example",
		"/ui/policies/example",
		"/ui/tokens/example",
		"/ui/roles/example",
		"/ui/csi/volumes/example",
		"/ui/csi/plugins/example",
		"/ui/csi/plugins/example/allocations",
		"/ui/settings/token",
		"/ui/evaluations/example",
		"/ui/deployments/example",
	})
}

func TestUIHomeRedirectContract(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter(NewHandler(nil)).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/ui/", nil))
	if response.Code != http.StatusFound {
		t.Fatalf("GET /ui/ status = %d, want 302", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/ui/jobs" {
		t.Fatalf("GET /ui/ location = %q, want /ui/jobs", location)
	}
}

func TestSearchRouteIsAlwaysAPartial(t *testing.T) {
	router := NewRouter(NewHandler(nil))
	for _, htmx := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodGet, "/ui/search?q=", nil)
		if htmx {
			request.Header.Set("HX-Request", "true")
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("search HTMX=%t status = %d, want 200", htmx, response.Code)
		}
		if strings.Contains(strings.ToLower(response.Body.String()), "<!doctype html") {
			t.Fatalf("search HTMX=%t unexpectedly returned a document shell", htmx)
		}
	}
}

func TestUnknownRoutesReturnNotFound(t *testing.T) {
	router := NewRouter(NewHandler(nil))
	assertRoutesHaveStatus(t, router, http.MethodGet, http.StatusNotFound, []string{
		"/ui/not-a-route",
		"/ui/clients/example/monitor/not-a-route",
		"/ui/servers/example/monitor/not-a-route",
		"/ui/services/name/extra",
		"/ui/topology/extra",
		"/ui/settings/not-a-route",
		"/ui/settings/not-a-route/token",
		"/ui/access-control/not-a-route",
		"/api/jobs/example/not-an-action",
		"/api/namespace/not-an-action",
		"/api/region/not-an-action",
		"/api/clients/example/not-an-action",
		"/api/deployments/example/not-an-action",
	})
}

func TestMissingDynamicResourcesHaveTruthfulStatus(t *testing.T) {
	router, closeBackend := newRouteContractRouter(t)
	defer closeBackend()

	for _, contract := range []struct {
		path       string
		wantStatus int
	}{
		{path: "/ui/jobs/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/allocations/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/clients/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/servers/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/services/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/variables/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/access-control/policies/missing", wantStatus: http.StatusForbidden},
		{path: "/ui/access-control/tokens/missing", wantStatus: http.StatusForbidden},
		{path: "/ui/access-control/roles/missing", wantStatus: http.StatusForbidden},
		{path: "/ui/policies/missing", wantStatus: http.StatusForbidden},
		{path: "/ui/tokens/missing", wantStatus: http.StatusForbidden},
		{path: "/ui/roles/missing", wantStatus: http.StatusForbidden},
		{path: "/ui/csi/volumes/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/csi/plugins/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/evaluations/missing", wantStatus: http.StatusNotFound},
		{path: "/ui/deployments/missing", wantStatus: http.StatusNotFound},
	} {
		t.Run(contract.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, contract.path, nil))
			assertRouteStatus(t, response, http.MethodGet, contract.path, contract.wantStatus)
		})
	}
}

func topLevelPageStatus(path string) int {
	if strings.HasPrefix(path, aclBasePath) || path == legacyPoliciesPath || strings.HasPrefix(path, legacyTokensPath) || strings.HasPrefix(path, legacyRolesPath) {
		return http.StatusForbidden
	}
	return http.StatusOK
}

func newRouteContractRouter(t *testing.T) (*Router, func()) {
	t.Helper()
	backend := newEmptyRouteContractBackend(t)
	t.Setenv("WANDER_ADDR", backend.URL)
	getTokenCache().Clear()
	return NewRouter(newJobsTestHandler(t, backend.URL)), backend.Close
}

func assertRouteStatus(t *testing.T, response *httptest.ResponseRecorder, method, path string, wantStatus int) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body: %s", method, path, response.Code, wantStatus, response.Body.String())
	}
}

func assertRoutesHaveStatus(t *testing.T, router *Router, method string, wantStatus int, paths []string) {
	t.Helper()
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(method, path, nil))
			assertRouteStatus(t, response, method, path, wantStatus)
		})
	}
}

func newEmptyRouteContractBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/jobs", "/v1/allocations", "/v1/nodes", "/v1/services",
			"/v1/namespaces", "/v1/regions", "/v1/vars", "/v1/acl/policies",
			"/v1/plugins", "/v1/volumes", "/v1/evaluations", "/v1/deployments":
			_, _ = w.Write([]byte(`[]`))
		case "/v1/agent/members":
			_, _ = w.Write([]byte(`{"ServerName":"","ServerRegion":"global","ServerDC":"dc1","Members":[]}`))
		case "/v1/status/leader":
			_, _ = w.Write([]byte(`""`))
		case "/v1/acl/token/self":
			http.Error(w, "ACL support disabled", http.StatusBadRequest)
		default:
			http.NotFound(w, r)
		}
	}))
}

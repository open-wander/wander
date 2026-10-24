package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
	"github.com/shoenig/test/must"
)

func TestACLTokenCreateAndSecretVisibility(t *testing.T) {
	var created api.ACLToken
	backend := newACLManagementBackend(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/acl/token":
			must.NoError(t, json.NewDecoder(r.Body).Decode(&created))
			response := created
			response.AccessorID = "accessor-1"
			response.SecretID = "create-only-secret"
			writeJSON(t, w, response)
			return true
		case r.Method == http.MethodGet && r.URL.Path == "/v1/acl/token/accessor-1":
			writeJSON(t, w, api.ACLToken{AccessorID: "accessor-1", SecretID: "must-not-leak", Name: "automation", Type: "client"})
			return true
		}
		return false
	})
	defer backend.Close()

	handler := NewACLManagementHandler(newJobsTestHandler(t, backend.URL))
	createRequest := managementRequest(http.MethodPost, "/ui/tokens/new", url.Values{
		"name":           {"automation"},
		"type":           {"client"},
		"global":         {"true"},
		"expiration_ttl": {"24h"},
		"policies":       {"read-jobs", "read-jobs"},
		"roles":          {"role-1"},
	})
	createRequest.Header.Set("HX-Request", "true")
	createResponse := httptest.NewRecorder()
	handler.ServeTokensHTTP(createResponse, createRequest)

	must.Eq(t, http.StatusOK, createResponse.Code)
	must.StrContains(t, createResponse.Body.String(), "create-only-secret")
	must.Eq(t, "/ui/tokens/accessor-1", createResponse.Header().Get("HX-Push-Url"))
	must.Eq(t, "automation", created.Name)
	must.Eq(t, "client", created.Type)
	must.True(t, created.Global)
	must.Eq(t, []string{"read-jobs"}, created.Policies)
	must.Len(t, 1, created.Roles)
	must.Eq(t, "role-1", created.Roles[0].ID)
	must.Eq(t, "24h0m0s", created.ExpirationTTL.String())

	detailRequest := managementRequest(http.MethodGet, "/ui/tokens/accessor-1", nil)
	detailRequest.Header.Set("HX-Request", "true")
	detailResponse := httptest.NewRecorder()
	handler.ServeTokensHTTP(detailResponse, detailRequest)

	must.Eq(t, http.StatusOK, detailResponse.Code)
	must.StrNotContains(t, detailResponse.Body.String(), "must-not-leak")
}

func TestACLRoleCreateUsesSelectedPolicies(t *testing.T) {
	var created api.ACLRole
	backend := newACLManagementBackend(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/acl/role" {
			return false
		}
		must.NoError(t, json.NewDecoder(r.Body).Decode(&created))
		created.ID = "role-1"
		writeJSON(t, w, created)
		return true
	})
	defer backend.Close()

	handler := NewACLManagementHandler(newJobsTestHandler(t, backend.URL))
	request := managementRequest(http.MethodPost, "/ui/roles/new", url.Values{
		"name":        {"operators"},
		"description": {"Operations team"},
		"policies":    {"read-jobs", "submit-jobs"},
	})
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	handler.ServeRolesHTTP(response, request)

	must.Eq(t, http.StatusOK, response.Code)
	must.Eq(t, "/ui/roles/role-1", response.Header().Get("HX-Push-Url"))
	must.Eq(t, "operators", created.Name)
	must.Len(t, 2, created.Policies)
	must.Eq(t, "read-jobs", created.Policies[0].Name)
	must.Eq(t, "submit-jobs", created.Policies[1].Name)
}

func TestACLManagementRequiresManagementToken(t *testing.T) {
	handler := NewACLManagementHandler(&Handler{logger: hclog.NewNullLogger()})
	for _, path := range []string{"/ui/tokens", "/ui/tokens/new", "/ui/roles", "/ui/roles/new"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request = request.WithContext(WithPermissions(request.Context(), NewUserPermissions(&ACLTokenInfo{
				Token: &api.ACLToken{Type: "client"}, IsValid: true, ACLEnabled: true,
			})))
			response := httptest.NewRecorder()
			if strings.HasPrefix(path, "/ui/tokens") {
				handler.ServeTokensHTTP(response, request)
			} else {
				handler.ServeRolesHTTP(response, request)
			}
			must.Eq(t, http.StatusForbidden, response.Code)
			must.StrContains(t, response.Body.String(), "Access Denied")
		})
	}
}

func TestACLManagementReportsMissingResourcesAndAPIFailures(t *testing.T) {
	backend := newACLManagementBackend(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/v1/acl/token/missing", "/v1/acl/role/missing":
			http.NotFound(w, r)
			return true
		case "/v1/acl/tokens":
			http.Error(w, "backend failed", http.StatusInternalServerError)
			return true
		}
		return false
	})
	defer backend.Close()

	handler := NewACLManagementHandler(newJobsTestHandler(t, backend.URL))
	for _, test := range []struct {
		path       string
		serve      func(http.ResponseWriter, *http.Request)
		wantStatus int
	}{
		{path: "/ui/tokens/missing", serve: handler.ServeTokensHTTP, wantStatus: http.StatusNotFound},
		{path: "/ui/roles/missing", serve: handler.ServeRolesHTTP, wantStatus: http.StatusNotFound},
		{path: "/ui/tokens", serve: handler.ServeTokensHTTP, wantStatus: http.StatusInternalServerError},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.serve(response, managementRequest(http.MethodGet, test.path, nil))
			must.Eq(t, test.wantStatus, response.Code)
		})
	}
}

func TestACLFormValidation(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/ui/tokens/new", strings.NewReader(url.Values{
		"type":           {"client"},
		"expiration_ttl": {"yesterday"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	token, tokenError := parseTokenForm(request, "")
	must.NotNil(t, token)
	must.StrContains(t, tokenError, "positive duration")

	request = httptest.NewRequest(http.MethodPost, "/ui/roles/new", strings.NewReader(url.Values{"name": {"operators"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	role, roleError := parseRoleForm(request, "")
	must.Eq(t, "operators", role.Name)
	must.StrContains(t, roleError, "at least one policy")
}

func TestPolicyCreate(t *testing.T) {
	var created api.ACLPolicy
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/acl/policy/read-jobs" {
			http.NotFound(w, r)
			return
		}
		must.NoError(t, json.NewDecoder(r.Body).Decode(&created))
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := NewPoliciesHandler(newJobsTestHandler(t, backend.URL))
	request := managementRequest(http.MethodPost, "/ui/policies/new", url.Values{
		"name":        {"read-jobs"},
		"description": {"Read job data"},
		"rules":       {`namespace "default" { policy = "read" }`},
	})
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	must.Eq(t, http.StatusOK, response.Code)
	must.Eq(t, "/ui/policies/read-jobs", response.Header().Get("HX-Redirect"))
	must.Eq(t, "read-jobs", created.Name)
	must.Eq(t, "Read job data", created.Description)
}

func newACLManagementBackend(t *testing.T, custom func(http.ResponseWriter, *http.Request) bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if custom(w, r) {
			return
		}
		switch r.URL.Path {
		case "/v1/acl/policies":
			writeJSON(t, w, []api.ACLPolicyListStub{{Name: "read-jobs", Description: "Read jobs"}, {Name: "submit-jobs", Description: "Submit jobs"}})
		case "/v1/acl/roles":
			writeJSON(t, w, []api.ACLRoleListStub{{ID: "role-1", Name: "operators", Description: "Operations team"}})
		case "/v1/namespaces":
			_, _ = w.Write([]byte(`[]`))
		case "/v1/regions":
			_, _ = w.Write([]byte(`["global"]`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func managementRequest(method, path string, form url.Values) *http.Request {
	var body *strings.Reader
	if form == nil {
		body = strings.NewReader("")
	} else {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	ctx := WithTokenContext(request.Context(), "management-secret")
	ctx = WithPermissions(ctx, NewUserPermissions(&ACLTokenInfo{
		Token: &api.ACLToken{AccessorID: "current", Type: "management"}, IsValid: true, ACLEnabled: true,
	}))
	return request.WithContext(ctx)
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	must.NoError(t, json.NewEncoder(w).Encode(value))
}

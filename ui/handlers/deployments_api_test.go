package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeploymentMutationsPreserveResolvedScope(t *testing.T) {
	for _, test := range []struct {
		action       string
		mutationPath string
	}{
		{action: "promote", mutationPath: "/v1/deployment/promote/deployment-1"},
		{action: "fail", mutationPath: "/v1/deployment/fail/deployment-1"},
	} {
		t.Run(test.action, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Nomad-Token") != "browser-token" {
					t.Errorf("token = %q", r.Header.Get("X-Nomad-Token"))
				}
				if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("region") != "west" {
					t.Errorf("scope = %q", r.URL.RawQuery)
				}
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/deployment/deployment-1":
					_, _ = w.Write([]byte(`{"ID":"deployment-1","Namespace":"team-a","Status":"running"}`))
				case r.Method == http.MethodPut && r.URL.Path == test.mutationPath:
					_, _ = w.Write([]byte(`{"EvalID":"eval-1"}`))
				default:
					t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
					http.NotFound(w, r)
				}
			}))
			defer backend.Close()

			base := newJobsTestHandler(t, backend.URL)
			request := requestWithToken(http.MethodPost, "/api/deployments/deployment-1/"+test.action+"?namespace=team-a&region=west")
			request.Header.Set("HX-Request", "true")
			request = request.WithContext(WithPermissions(request.Context(), NewUserPermissions(&ACLTokenInfo{
				IsValid: true, ACLEnabled: false,
			})))
			response := httptest.NewRecorder()
			NewDeploymentsAPIHandler(base).ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			if response.Header().Get("HX-Refresh") != "true" {
				t.Fatalf("HX-Refresh = %q", response.Header().Get("HX-Refresh"))
			}
		})
	}
}

func TestDeploymentMutationErrorsAreTruthfulAndSafe(t *testing.T) {
	for _, test := range []struct {
		name       string
		apiStatus  int
		wantStatus int
		wantBody   string
	}{
		{name: "denied", apiStatus: http.StatusForbidden, wantStatus: http.StatusForbidden, wantBody: "Permission denied"},
		{name: "missing", apiStatus: http.StatusNotFound, wantStatus: http.StatusNotFound, wantBody: "Deployment not found"},
		{name: "unavailable", apiStatus: http.StatusInternalServerError, wantStatus: http.StatusBadGateway, wantBody: "cluster rejected"},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "secret backend detail", test.apiStatus)
			}))
			defer backend.Close()

			response := httptest.NewRecorder()
			request := requestWithToken(http.MethodPost, "/api/deployments/deployment-1/promote?namespace=team-a&region=west")
			NewDeploymentsAPIHandler(newJobsTestHandler(t, backend.URL)).ServeHTTP(response, request)

			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("unexpected response: status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "secret backend detail") {
				t.Fatalf("response exposed backend detail: %s", response.Body.String())
			}
		})
	}
}

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestServicesListAndDetail(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/v1/services":
			_, _ = w.Write([]byte(`[{"Namespace":"default","Services":[{"ServiceName":"wander-http","Tags":["ui","http"]}]}]`))
		case "/v1/service/wander-http":
			_, _ = w.Write([]byte(`[{"ID":"service-1","ServiceName":"wander-http","Namespace":"default","NodeID":"node-123456","Datacenter":"dc1","JobID":"example","AllocID":"alloc-123456","Tags":["ui","http"],"Address":"127.0.0.1","Port":8080}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	tests := []struct {
		name     string
		target   string
		htmx     bool
		wantBody []string
	}{
		{name: "full list", target: "/ui/services", wantBody: []string{"<!doctype html", "wander-http", "ui, http"}},
		{name: "HTMX list", target: "/ui/services", htmx: true, wantBody: []string{"wander-http", "Nomad-native service registrations"}},
		{name: "detail", target: "/ui/services/wander-http", wantBody: []string{"127.0.0.1:8080", "example", "alloc-12", "node-123"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			if test.htmx {
				request.Header.Set("HX-Request", "true")
			}
			response := httptest.NewRecorder()
			(&ServicesHandler{handler: handler}).ServeHTTP(response, request)
			for _, expected := range test.wantBody {
				assertHTTPResponse(t, response, http.StatusOK, expected)
			}
		})
	}
}

func TestServicesStates(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/services":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case "/v1/service/missing":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		case "/v1/service/unavailable":
			http.Error(w, "No cluster leader", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	tests := []struct {
		name        string
		target      string
		permissions *UserPermissions
		wantStatus  int
		wantBody    string
	}{
		{name: "empty", target: "/ui/services", wantStatus: http.StatusOK, wantBody: "No services found"},
		{name: "not found", target: "/ui/services/missing", wantStatus: http.StatusNotFound, wantBody: "was not found"},
		{name: "unavailable", target: "/ui/services/unavailable", wantStatus: http.StatusServiceUnavailable, wantBody: "Cluster Unavailable"},
		{
			name:   "denied",
			target: "/ui/services",
			permissions: NewUserPermissions(&ACLTokenInfo{
				Token: &api.ACLToken{Type: "client"}, IsValid: true, ACLEnabled: true,
			}),
			wantStatus: http.StatusForbidden,
			wantBody:   "permission to view service registrations",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.target, nil)
			if test.permissions != nil {
				request = request.WithContext(WithPermissions(request.Context(), test.permissions))
			}
			response := httptest.NewRecorder()
			(&ServicesHandler{handler: handler}).ServeHTTP(response, request)
			assertHTTPResponse(t, response, test.wantStatus, test.wantBody)
		})
	}
}

func TestServicesRejectUnsupportedRoutesAndMethods(t *testing.T) {
	handler := &ServicesHandler{handler: NewHandler(nil)}
	for _, test := range []struct {
		method string
		path   string
		status int
	}{
		{method: http.MethodPost, path: "/ui/services", status: http.StatusMethodNotAllowed},
		{method: http.MethodGet, path: "/ui/services/name/extra", status: http.StatusNotFound},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
		if response.Code != test.status {
			t.Fatalf("%s %s status = %d, want %d", test.method, test.path, response.Code, test.status)
		}
	}
}

func TestJobServicesAndClientsUseLiveScopedData(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("region") != "region-a" {
			t.Errorf("request scope = %q/%q, want team-a/region-a", r.URL.Query().Get("namespace"), r.URL.Query().Get("region"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/v1/job/parent%2Fdispatch-1/services":
			_, _ = w.Write([]byte(`[{"ServiceName":"child-service","JobID":"parent/dispatch-1"}]`))
		case "/v1/job/parent%2Fdispatch-1/allocations":
			_, _ = w.Write([]byte(`[
				{"ID":"alloc-1","NodeID":"node-a","NodeName":"client-a","ClientStatus":"complete","ModifyIndex":1},
				{"ID":"alloc-2","NodeID":"node-a","NodeName":"client-a","ClientStatus":"running","ModifyIndex":2},
				{"ID":"alloc-3","NodeID":"node-b","NodeName":"client-b","ClientStatus":"failed","ModifyIndex":3}
			]`))
		default:
			t.Errorf("unexpected API request: %s", r.URL.RequestURI())
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := NewJobsHandler(newJobsTestHandler(t, backend.URL))
	request := httptest.NewRequest(http.MethodGet, "/ui/jobs/parent/dispatch-1/clients?namespace=team-a&region=region-a", nil)
	services, err := handler.getJobServices(request, "parent/dispatch-1")
	if err != nil || len(services) != 1 || services[0].ServiceName != "child-service" {
		t.Fatalf("unexpected services: %#v, err: %v", services, err)
	}
	clients, err := handler.getJobClients(request, "parent/dispatch-1")
	if err != nil {
		t.Fatalf("get job clients: %v", err)
	}
	if len(clients) != 2 || clients[0].ID != "node-a" || clients[0].Allocations != 2 || clients[0].ActiveAllocations != 1 || clients[0].LatestStatus != "running" {
		t.Fatalf("unexpected job clients: %#v", clients)
	}
}

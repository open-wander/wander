package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/partials/clientspartials"
)

func TestClientListKeepsRowsWhenAllocationSummaryFails(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/nodes" {
			_ = json.NewEncoder(w).Encode([]*api.NodeListStub{{ID: "node-1", Name: "worker", Status: "ready"}})
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer backend.Close()

	request := httptest.NewRequest(http.MethodGet, "/ui/clients", nil)
	clientsList, err := NewClientsHandler(newJobsTestHandler(t, backend.URL)).getClientsList(request, "")
	if err != nil {
		t.Fatalf("get clients list: %v", err)
	}
	if len(clientsList) != 1 || clientsList[0].AllocationsAvailable || clientsList[0].ResourcesAvailable {
		t.Fatalf("partial allocation failure = %#v", clientsList)
	}
}

func TestClientListDistinguishesEmptyDeniedAndUnavailable(t *testing.T) {
	assertClientListState(t, http.StatusOK, http.StatusOK, "No clients found")
	assertClientListState(t, http.StatusForbidden, http.StatusForbidden, "Access Denied")
	assertClientListState(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, "Cluster Unavailable")
}

func TestClientListPaginationRendersOnlyTablePartial(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]"))
	}))
	defer backend.Close()

	request := httptest.NewRequest(http.MethodGet, "/ui/clients?page=2", nil)
	request.Header.Set("HX-Request", "true")
	request.Header.Set("HX-Target", "clients-table-container")
	response := httptest.NewRecorder()
	NewClientsHandler(newJobsTestHandler(t, backend.URL)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if !strings.Contains(body, "No clients found") || strings.Contains(body, "<h1") {
		t.Fatalf("pagination response is not table-only: %s", body)
	}
}

func assertClientListState(t *testing.T, backendCode, wantCode int, wantText string) {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/nodes" {
			w.WriteHeader(backendCode)
			if backendCode == http.StatusOK {
				_, _ = w.Write([]byte("[]"))
			}
			return
		}
		_, _ = w.Write([]byte("[]"))
	}))
	defer backend.Close()

	handler := NewClientsHandler(newJobsTestHandler(t, backend.URL))
	request := httptest.NewRequest(http.MethodGet, "/ui/clients", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantCode {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, wantCode, response.Body.String())
	}
	if !containsBodyText(response.Body.String(), wantText) {
		t.Fatalf("body does not contain %q: %s", wantText, response.Body.String())
	}
}

func containsBodyText(body, text string) bool {
	return strings.Contains(body, text)
}

func TestClientListComposesFilters(t *testing.T) {
	handler := &ClientsHandler{}
	clientsList := clientListFixtures()

	filtered := handler.applyFilters(clientsList, clientspartials.ClientFilters{
		Status: "ready", Datacenter: "dc1", NodePool: "batch", NodeClass: "gpu",
		Version: "1.6.5", Volume: "models", Search: "worker",
	})
	if len(filtered) != 1 || filtered[0].ID != "node-b" {
		t.Fatalf("filtered clients = %#v, want node-b", filtered)
	}

	options := handler.extractFilterOptions(clientsList)
	if len(options.NodePools) != 2 || len(options.Versions) != 2 || len(options.Volumes) != 2 {
		t.Fatalf("incomplete filter options: %#v", options)
	}
}

func TestClientListSortAndPaginationAreDeterministic(t *testing.T) {
	handler := &ClientsHandler{}
	clientsList := clientListFixtures()
	clientsList = append(clientsList, clientspartials.ClientSummary{ID: "node-c", Name: "alpha", Allocations: 1})

	handler.sortClients(clientsList, "name", "asc")
	if clientsList[0].ID != "node-a" || clientsList[1].ID != "node-c" {
		t.Fatalf("name sort = %#v", clientsList)
	}
	page := paginateClients(clientsList, 2, 2)
	if len(page) != 1 || page[0].ID != "node-b" {
		t.Fatalf("second page = %#v", page)
	}

	handler.sortClients(clientsList, "allocations", "desc")
	if clientsList[0].ID != "node-b" {
		t.Fatalf("allocation sort = %#v", clientsList)
	}
}

func clientListFixtures() []clientspartials.ClientSummary {
	return []clientspartials.ClientSummary{
		{ID: "node-a", Name: "alpha", Status: "ready", Datacenter: "dc2", NodePool: "default", NodeClass: "general", Version: "1.6.4", Volumes: []string{"cache"}, Allocations: 1},
		{ID: "node-b", Name: "worker", Status: "ready", Datacenter: "dc1", NodePool: "batch", NodeClass: "gpu", Version: "1.6.5", Volumes: []string{"models"}, Allocations: 4},
	}
}

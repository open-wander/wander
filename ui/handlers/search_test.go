package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/api/contexts"
)

func TestSearchHandler_EmptyQuery(t *testing.T) {
	// Create a handler with no Nomad client (will use mock data)
	h := NewHandler(hclog.NewNullLogger())
	searchHandler := NewSearchHandler(h)

	// Create request with empty query
	req := httptest.NewRequest(http.MethodGet, "/ui/search?q=", nil)
	w := httptest.NewRecorder()

	// Make request
	searchHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSearchHandler_ShortQuery(t *testing.T) {
	// Create a handler with no Nomad client (will use mock data)
	h := NewHandler(hclog.NewNullLogger())
	searchHandler := NewSearchHandler(h)

	// Create request with query too short (only 1 char)
	req := httptest.NewRequest(http.MethodGet, "/ui/search?q=a", nil)
	w := httptest.NewRecorder()

	// Make request
	searchHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSearchHandler_MethodNotAllowed(t *testing.T) {
	// Create a handler
	h := NewHandler(hclog.NewNullLogger())
	searchHandler := NewSearchHandler(h)

	// Create POST request (should only accept GET)
	req := httptest.NewRequest(http.MethodPost, "/ui/search?q=test", nil)
	w := httptest.NewRecorder()

	// Make request
	searchHandler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestSearchHandlerUsesLiveScopedFuzzySearch(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/search/fuzzy" {
			t.Fatalf("search request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("region") != "west" {
			t.Fatalf("search scope = %s", r.URL.RawQuery)
		}
		if token := r.Header.Get("X-Nomad-Token"); token != "browser-token" {
			t.Fatalf("search token = %q", token)
		}
		var request api.FuzzySearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode search request: %v", err)
		}
		if request.Text != "smoke" || request.Context != contexts.All {
			t.Fatalf("search request = %#v", request)
		}
		writeJSON(t, w, &api.FuzzySearchResponse{
			Matches: map[contexts.Context][]api.FuzzyMatch{
				contexts.Nodes:    {{ID: "node-1"}},
				contexts.Allocs:   {{ID: "alloc/id", Scope: []string{"team-a", "smoke"}}},
				contexts.Jobs:     {{ID: "z-job", Scope: []string{"team-a"}}, {ID: "a/job", Scope: []string{"team-a"}}},
				contexts.Commands: {{ID: "unsupported"}},
			},
			Truncations: map[contexts.Context]bool{contexts.Jobs: true},
		})
	}))
	defer backend.Close()

	request := httptest.NewRequest(http.MethodGet, "/ui/search?q=smoke&namespace=team-a&region=west", nil)
	request = request.WithContext(WithTokenContext(request.Context(), "browser-token"))
	response := httptest.NewRecorder()
	NewSearchHandler(newJobsTestHandler(t, backend.URL)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("search status = %d, body: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	assertTextOrder(t, body, "Jobs", "Allocations", "Nodes")
	assertTextOrder(t, body, "a/job", "z-job")
	for _, expected := range []string{
		`/ui/jobs/a%2Fjob?namespace=team-a&amp;region=west`,
		`/ui/allocations/alloc%2Fid?namespace=team-a&amp;region=west`,
		`/ui/clients/node-1?region=west`,
		`showing first 2`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("search response missing %q: %s", expected, body)
		}
	}
	if strings.Contains(body, "unsupported") {
		t.Fatalf("search response exposed unsupported result: %s", body)
	}
}

func TestSearchHandlerRendersDistinctNonResultStates(t *testing.T) {
	short := httptest.NewRecorder()
	NewSearchHandler(NewHandler(hclog.NewNullLogger())).ServeHTTP(short, httptest.NewRequest(http.MethodGet, "/ui/search?q=a", nil))
	if short.Code != http.StatusOK || !strings.Contains(short.Body.String(), "data-search-too-short") {
		t.Fatalf("short-query response: status=%d body=%s", short.Code, short.Body.String())
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "backend-secret-error", http.StatusInternalServerError)
	}))
	defer backend.Close()
	unavailable := httptest.NewRecorder()
	NewSearchHandler(newJobsTestHandler(t, backend.URL)).ServeHTTP(
		unavailable,
		httptest.NewRequest(http.MethodGet, "/ui/search?global-search=smoke", nil),
	)
	if unavailable.Code != http.StatusOK || !strings.Contains(unavailable.Body.String(), "data-search-error") {
		t.Fatalf("unavailable response: status=%d body=%s", unavailable.Code, unavailable.Body.String())
	}
	if strings.Contains(unavailable.Body.String(), "backend-secret-error") {
		t.Fatalf("unavailable response leaked backend error: %s", unavailable.Body.String())
	}
}

func TestBuildSearchResultURL(t *testing.T) {
	for _, test := range []struct {
		name  string
		ctx   contexts.Context
		match api.FuzzyMatch
		want  string
	}{
		{name: "job", ctx: contexts.Jobs, match: api.FuzzyMatch{ID: "job/child", Scope: []string{"team-a"}}, want: "/ui/jobs/job%2Fchild?namespace=team-a&region=west"},
		{name: "service", ctx: contexts.Services, match: api.FuzzyMatch{ID: "web api", Scope: []string{"team-a"}}, want: "/ui/services/web%20api?namespace=team-a&region=west"},
		{name: "node pool", ctx: contexts.NodePools, match: api.FuzzyMatch{ID: "gpu/pool"}, want: "/ui/clients?node_pool=gpu%2Fpool&region=west"},
		{name: "malformed group", ctx: contexts.Groups, match: api.FuzzyMatch{ID: "web", Scope: []string{"team-a"}}, want: ""},
		{name: "unsupported", ctx: contexts.Commands, match: api.FuzzyMatch{ID: "run"}, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := buildResultURL(test.ctx, test.match, "default", "west"); got != test.want {
				t.Fatalf("result URL = %q, want %q", got, test.want)
			}
		})
	}
}

func assertTextOrder(t *testing.T, text string, values ...string) {
	t.Helper()
	previous := -1
	for _, value := range values {
		position := strings.Index(text, value)
		if position <= previous {
			t.Fatalf("%q is not ordered after previous value in response", value)
		}
		previous = position
	}
}

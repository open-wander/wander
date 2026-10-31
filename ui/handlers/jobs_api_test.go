package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestDispatchJobFormUsesLiveContractAndRawPayload(t *testing.T) {
	var dispatched api.JobDispatchRequest
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/job/example":
			_, _ = w.Write([]byte(`{"ID":"example","ParameterizedJob":{"Payload":"required","MetaRequired":["environment"],"MetaOptional":["version"]}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/v1/job/example/dispatch":
			if err := json.NewDecoder(r.Body).Decode(&dispatched); err != nil {
				t.Errorf("decode dispatch request: %v", err)
			}
			_, _ = w.Write([]byte(`{"DispatchedJobID":"example/dispatch-123","EvalID":"eval-123"}`))
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	form := url.Values{
		"meta.environment": {"staging"},
		"meta.version":     {"v1"},
		"payload":          {"plain-text-payload"},
	}
	response := performJobAction(handler, "/api/jobs/example/dispatch?namespace=team-a&region=region-a", "application/x-www-form-urlencoded", form.Encode())

	assertHTTPResponse(t, response, http.StatusOK, "")
	if string(dispatched.Payload) != "plain-text-payload" {
		t.Fatalf("payload = %q, want raw browser payload", dispatched.Payload)
	}
	if dispatched.Meta["environment"] != "staging" || dispatched.Meta["version"] != "v1" {
		t.Fatalf("unexpected metadata: %#v", dispatched.Meta)
	}
	redirect := response.Header().Get("HX-Redirect")
	for _, value := range []string{"/ui/jobs/example%2Fdispatch-123", "namespace=team-a", "region=region-a"} {
		if !strings.Contains(redirect, value) {
			t.Fatalf("HX-Redirect %q does not contain %q", redirect, value)
		}
	}
}

func TestDispatchJobRejectsMissingInputsBeforeMutation(t *testing.T) {
	mutated := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			mutated = true
		}
		_, _ = w.Write([]byte(`{"ID":"example","ParameterizedJob":{"Payload":"required","MetaRequired":["environment"]}}`))
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	response := performJobAction(handler, "/api/jobs/example/dispatch", "application/x-www-form-urlencoded", "payload=")

	assertHTTPResponse(t, response, http.StatusBadRequest, "")
	if mutated {
		t.Fatal("invalid dispatch request reached the mutation endpoint")
	}
}

func TestDispatchJobRequiresDispatchCapability(t *testing.T) {
	handler := NewHandler(nil)
	request := httptest.NewRequest(http.MethodPost, "/api/jobs/example/dispatch", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	permissions := NewUserPermissions(nil)
	request = request.WithContext(WithPermissions(request.Context(), permissions))
	response := httptest.NewRecorder()

	NewJobsAPIHandler(handler).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "dispatch-job") {
		t.Fatalf("unexpected denial response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPurgeJobUsesPurgeFlagAndScopedRedirect(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/job/example/dispatch-123" {
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("purge") != "true" || r.URL.Query().Get("namespace") != "team-a" {
			t.Errorf("unexpected purge query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"EvalID":"eval-purge"}`))
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	response := performJobAction(handler, "/api/jobs/example/dispatch-123/purge?namespace=team-a&region=region-a", "", "")

	assertHTTPResponse(t, response, http.StatusOK, "")
	redirect := response.Header().Get("HX-Redirect")
	if !strings.Contains(redirect, "/ui/jobs?") || !strings.Contains(redirect, "namespace=team-a") || !strings.Contains(redirect, "region=region-a") {
		t.Fatalf("unexpected scoped redirect: %q", redirect)
	}
}

func TestForcePeriodicJobUsesScopedLiveMutation(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/v1/job/periodic/periodic/force" {
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("namespace") != "team-a" {
			t.Errorf("unexpected periodic force query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"EvalID":"eval-periodic"}`))
	}))
	defer backend.Close()

	handler := newJobsTestHandler(t, backend.URL)
	response := performJobAction(handler, "/api/jobs/periodic/periodic/force?namespace=team-a&region=region-a", "", "")

	assertHTTPResponse(t, response, http.StatusOK, "")
	if trigger := response.Header().Get("HX-Trigger"); !strings.Contains(trigger, "Periodic job forced successfully") {
		t.Fatalf("unexpected periodic force trigger: %q", trigger)
	}
}

func performJobAction(handler *Handler, target, contentType, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	NewJobsAPIHandler(handler).ServeHTTP(response, request)
	return response
}

func TestSplitJobPathsPreserveChildJobIDs(t *testing.T) {
	jobID, action, ok := splitJobActionPath("/api/jobs/example/dispatch-123/purge")
	if !ok || jobID != "example/dispatch-123" || action != "purge" {
		t.Fatalf("unexpected action path split: job=%q action=%q ok=%t", jobID, action, ok)
	}
	jobID, tab := splitJobDetailPath("example/dispatch-123/definition")
	if jobID != "example/dispatch-123" || tab != "definition" {
		t.Fatalf("unexpected detail path split: job=%q tab=%q", jobID, tab)
	}
}

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestJobTemplateLifecycleUsesNamespaceScopedVariables(t *testing.T) {
	const namespace = "team-a"
	var (
		mu      sync.Mutex
		stored  = map[string]*api.Variable{}
		methods []string
	)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("namespace"); got != namespace {
			t.Errorf("namespace = %q, want %q", got, namespace)
		}
		mu.Lock()
		defer mu.Unlock()
		methods = append(methods, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/vars":
			if got := r.URL.Query().Get("prefix"); got != jobTemplatePrefix {
				t.Errorf("prefix = %q, want %q", got, jobTemplatePrefix)
			}
			metadata := make([]*api.VariableMetadata, 0, len(stored))
			for _, variable := range stored {
				metadata = append(metadata, &api.VariableMetadata{Path: variable.Path, Namespace: variable.Namespace, ModifyIndex: variable.ModifyIndex})
			}
			writeJSON(t, w, metadata)
		case strings.HasPrefix(r.URL.Path, "/v1/var/"):
			path := strings.TrimPrefix(r.URL.Path, "/v1/var/")
			switch r.Method {
			case http.MethodGet:
				variable := stored[path]
				if variable == nil {
					http.NotFound(w, r)
					return
				}
				writeJSON(t, w, variable)
			case http.MethodPut:
				var variable api.Variable
				if err := json.NewDecoder(r.Body).Decode(&variable); err != nil {
					t.Fatalf("decode variable: %v", err)
				}
				if variable.Path != path || variable.Namespace != namespace {
					t.Errorf("unexpected variable: %#v", variable)
				}
				variable.ModifyIndex++
				stored[path] = &variable
				writeJSON(t, w, &variable)
			case http.MethodDelete:
				delete(stored, path)
				w.WriteHeader(http.StatusNoContent)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	handler := NewJobTemplatesHandler(newJobsTestHandler(t, backend.URL))
	createForm := url.Values{"name": {"service"}, "namespace": {namespace}, "template": {`job "service" {}`}}
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, formRequest(http.MethodPost, "/ui/jobs/run/templates/new?namespace="+namespace, createForm))
	if createResponse.Code != http.StatusSeeOther {
		t.Fatalf("create status = %d, want %d: %s", createResponse.Code, http.StatusSeeOther, createResponse.Body.String())
	}
	if location := createResponse.Header().Get("Location"); location != "/ui/jobs/run?namespace=team-a&template=service" {
		t.Fatalf("create location = %q", location)
	}

	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/ui/jobs/run/templates?namespace="+namespace, nil))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "Use Template") {
		t.Fatalf("list response = %d: %s", listResponse.Code, listResponse.Body.String())
	}

	definition, err := handler.readTemplate(httptest.NewRequest(http.MethodGet, "/ui/jobs/run?namespace="+namespace, nil), namespace, "service")
	if err != nil || definition != `job "service" {}` {
		t.Fatalf("read template = %q, %v", definition, err)
	}

	updateForm := url.Values{"template": {`job "service" { type = "service" }`}}
	updateResponse := httptest.NewRecorder()
	handler.ServeHTTP(updateResponse, formRequest(http.MethodPost, "/ui/jobs/run/templates/manage/service?namespace="+namespace, updateForm))
	if updateResponse.Code != http.StatusSeeOther {
		t.Fatalf("update status = %d: %s", updateResponse.Code, updateResponse.Body.String())
	}

	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/ui/jobs/run/templates/manage/service?namespace="+namespace, nil))
	if deleteResponse.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(stored) != 0 {
		t.Fatalf("stored templates after delete = %#v", stored)
	}
	for _, expected := range []string{
		"PUT /v1/var/nomad/job-templates/service",
		"GET /v1/vars",
		"GET /v1/var/nomad/job-templates/service",
		"DELETE /v1/var/nomad/job-templates/service",
	} {
		if !containsString(methods, expected) {
			t.Errorf("API calls %v do not contain %q", methods, expected)
		}
	}
}

func TestJobTemplateValidationAndMalformedVariable(t *testing.T) {
	for _, name := range []string{"", "two words", "nested/name", ".", ".."} {
		if err := validateTemplateName(name); err == nil {
			t.Errorf("validateTemplateName(%q) succeeded", name)
		}
	}
	if err := validateTemplateName("service-api_v2"); err != nil {
		t.Fatalf("valid template name rejected: %v", err)
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, &api.Variable{Path: jobTemplatePrefix + "broken", Namespace: "default", Items: api.VariableItems{"description": "missing source"}})
	}))
	defer backend.Close()
	handler := NewJobTemplatesHandler(newJobsTestHandler(t, backend.URL))
	_, err := handler.readTemplate(httptest.NewRequest(http.MethodGet, "/ui/jobs/run", nil), "default", "broken")
	if err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed template error = %v", err)
	}
}

func formRequest(method, target string, form url.Values) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

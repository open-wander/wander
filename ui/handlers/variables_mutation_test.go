package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestVariableCreateUsesCheckedWriteAndPreservesScope(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertVariableMutationScope(t, r, http.MethodPut, "0")
		var variable api.Variable
		if err := json.NewDecoder(r.Body).Decode(&variable); err != nil {
			t.Fatalf("decode variable: %v", err)
		}
		if variable.Path != "team/service" || variable.Items["answer"] != "42" {
			t.Fatalf("unexpected create payload: %#v", variable)
		}
		variable.ModifyIndex = 7
		writeJSON(t, w, &variable)
	}))
	defer backend.Close()

	response := serveVariableMutation(t, backend.URL, http.MethodPost, "/ui/variables?region=west", url.Values{
		"path": {"team/service"}, "namespace": {"team-a"}, "key_0": {"answer"}, "value_0": {"42"},
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body: %s", response.Code, response.Body.String())
	}
	if redirect := response.Header().Get("HX-Redirect"); redirect != "/ui/variables/team%2Fservice?namespace=team-a&region=west" {
		t.Fatalf("create redirect = %q", redirect)
	}
}

func TestVariableMutationsReportCASConflictsWithoutLeakingValues(t *testing.T) {
	for _, test := range []struct {
		name       string
		method     string
		target     string
		form       url.Values
		wantMethod string
		wantCAS    string
		wantText   string
	}{
		{
			name: "create", method: http.MethodPost, target: "/ui/variables?region=west",
			form:       url.Values{"path": {"team/service"}, "namespace": {"team-a"}, "key_0": {"answer"}, "value_0": {"42"}},
			wantMethod: http.MethodPut, wantCAS: "0", wantText: "already exists",
		},
		{
			name: "update", method: http.MethodPost, target: "/ui/variables/team%2Fservice?namespace=team-a&region=west",
			form:       url.Values{"modify_index": {"41"}, "key_0": {"answer"}, "value_0": {"updated"}},
			wantMethod: http.MethodPut, wantCAS: "41", wantText: "changed since this page was loaded",
		},
		{
			name: "delete", method: http.MethodDelete, target: "/ui/variables/team%2Fservice?namespace=team-a&region=west&modify_index=41",
			wantMethod: http.MethodDelete, wantCAS: "41", wantText: "changed since this page was loaded",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assertVariableMutationScope(t, r, test.wantMethod, test.wantCAS)
				w.WriteHeader(http.StatusConflict)
				writeJSON(t, w, &api.Variable{Path: "team/service", Namespace: "team-a", ModifyIndex: 42, Items: api.VariableItems{"secret": "must-not-leak"}})
			}))
			defer backend.Close()

			response := serveVariableMutation(t, backend.URL, test.method, test.target, test.form)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), test.wantText) {
				t.Fatalf("conflict response: status=%d body=%s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "must-not-leak") {
				t.Fatalf("conflict response disclosed current variable value: %s", response.Body.String())
			}
		})
	}
}

func TestVariableUpdateAcceptsJSONAndReturnsNextModifyIndex(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertVariableMutationScope(t, r, http.MethodPut, "41")
		var variable api.Variable
		if err := json.NewDecoder(r.Body).Decode(&variable); err != nil {
			t.Fatalf("decode variable: %v", err)
		}
		if variable.Items["answer"] != "updated" || variable.Items["mode"] != "json" {
			t.Fatalf("unexpected update payload: %#v", variable)
		}
		variable.ModifyIndex = 42
		writeJSON(t, w, &variable)
	}))
	defer backend.Close()

	response := serveVariableMutation(t, backend.URL, http.MethodPost, "/ui/variables/team%2Fservice?namespace=team-a&region=west", url.Values{
		"modify_index": {"41"},
		"items_json":   {`{"answer":"updated","mode":"json"}`},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d, body: %s", response.Code, response.Body.String())
	}
	if modifyIndex := response.Header().Get("X-Wander-Modify-Index"); modifyIndex != "42" {
		t.Fatalf("modify index header = %q", modifyIndex)
	}
}

func TestVariableCreateRejectsInvalidInputBeforeMutation(t *testing.T) {
	requests := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "unexpected mutation", http.StatusInternalServerError)
	}))
	defer backend.Close()

	for _, test := range []struct {
		name string
		form url.Values
		want string
	}{
		{name: "duplicate", form: url.Values{"path": {"team/service"}, "namespace": {"team-a"}, "key_0": {"answer"}, "value_0": {"one"}, "key_1": {"answer"}, "value_1": {"two"}}, want: "duplicate variable item key"},
		{name: "empty", form: url.Values{"path": {"team/service"}, "namespace": {"team-a"}}, want: "requires at least one item"},
		{name: "reserved", form: url.Values{"path": {"nomad/private"}, "namespace": {"team-a"}, "key_0": {"answer"}, "value_0": {"42"}}, want: "reserved"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serveVariableMutation(t, backend.URL, http.MethodPost, "/ui/variables?region=west", test.form)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), test.want) {
				t.Fatalf("validation response: status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
	if requests != 0 {
		t.Fatalf("invalid inputs issued %d API requests", requests)
	}
}

func serveVariableMutation(t *testing.T, backendURL, method, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequest(method, target, body)
	request = request.WithContext(WithTokenContext(request.Context(), "browser-token"))
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	response := httptest.NewRecorder()
	NewVariablesHandler(newJobsTestHandler(t, backendURL)).ServeHTTP(response, request)
	return response
}

func assertVariableMutationScope(t *testing.T, r *http.Request, method, cas string) {
	t.Helper()
	if r.Method != method || r.URL.Path != "/v1/var/team/service" || r.URL.Query().Get("cas") != cas {
		t.Fatalf("mutation request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
	}
	if r.URL.Query().Get("namespace") != "team-a" || r.URL.Query().Get("region") != "west" {
		t.Fatalf("mutation scope = %s", r.URL.RawQuery)
	}
	if r.Header.Get("X-Nomad-Token") != "browser-token" {
		t.Fatalf("mutation token = %q", r.Header.Get("X-Nomad-Token"))
	}
}

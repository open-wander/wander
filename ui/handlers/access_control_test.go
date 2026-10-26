package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-wander/wander/api"
	"github.com/shoenig/test/must"
)

func TestAccessControlOverviewShowsLiveResourceCounts(t *testing.T) {
	backend := newAccessControlBackend(t, []api.ACLTokenListStub{{AccessorID: "one"}, {AccessorID: "two"}})
	defer backend.Close()

	handler := NewAccessControlHandler(newJobsTestHandler(t, backend.URL))
	response := serveAccessControl(t, handler, aclBasePath)

	must.StrContains(t, response.Body.String(), "Access Control")
	must.StrContains(t, response.Body.String(), aclPoliciesPath+"/new")
	must.StrContains(t, response.Body.String(), aclTokensPath+"/new")
	must.StrContains(t, response.Body.String(), aclRolesPath+"/new")
	must.StrContains(t, response.Body.String(), ">2</span>")
}

func TestLegacyAccessControlAliasUsesCanonicalLinks(t *testing.T) {
	backend := newAccessControlBackend(t, []api.ACLTokenListStub{})
	defer backend.Close()

	handler := NewAccessControlHandler(newJobsTestHandler(t, backend.URL))
	alias := rewritePathPrefix(legacyTokensPath, aclTokensPath, handler)
	response := serveAccessControl(t, alias, legacyTokensPath)

	must.StrContains(t, response.Body.String(), aclTokensPath+"/new")
	must.StrNotContains(t, response.Body.String(), `href="`+legacyTokensPath+`/new`)
}

func TestAccessControlOverviewRejectsUnsupportedMethods(t *testing.T) {
	response := httptest.NewRecorder()
	NewAccessControlHandler(NewHandler(nil)).ServeHTTP(response, managementRequest(http.MethodPost, aclBasePath, nil))
	must.Eq(t, http.StatusMethodNotAllowed, response.Code)
	must.Eq(t, http.MethodGet, response.Header().Get("Allow"))
}

func newAccessControlBackend(t *testing.T, tokens []api.ACLTokenListStub) *httptest.Server {
	t.Helper()
	return newACLManagementBackend(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v1/acl/tokens" {
			return false
		}
		writeJSON(t, w, tokens)
		return true
	})
}

func serveAccessControl(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, managementRequest(http.MethodGet, path, nil))
	must.Eq(t, http.StatusOK, response.Code)
	return response
}

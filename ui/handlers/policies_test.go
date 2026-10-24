package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
	"github.com/shoenig/test/must"
)

func TestPoliciesHandlerRoutesMutations(t *testing.T) {
	handler := NewPoliciesHandler(&Handler{logger: hclog.NewNullLogger()})
	permissions := NewUserPermissions(&ACLTokenInfo{
		Token:      &api.ACLToken{Type: "management"},
		IsValid:    true,
		ACLEnabled: true,
	})

	tests := []struct {
		method string
		body   url.Values
	}{
		{method: http.MethodPost, body: url.Values{"description": {"test"}, "rules": {"namespace \"default\" { policy = \"read\" }"}}},
		{method: http.MethodDelete},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/ui/policies/test-policy", strings.NewReader(test.body.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request = request.WithContext(WithPermissions(request.Context(), permissions))
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			must.Eq(t, http.StatusInternalServerError, response.Code)
		})
	}
}

func TestPolicyRelationshipsIncludesDirectAndRoleBindings(t *testing.T) {
	roles := []*api.ACLRoleListStub{
		{ID: "role-b", Name: "builders", Policies: []*api.ACLRolePolicyLink{{Name: "read-jobs"}}},
		{ID: "role-o", Name: "other", Policies: []*api.ACLRolePolicyLink{{Name: "other-policy"}}},
	}
	tokens := []*api.ACLTokenListStub{
		{AccessorID: "direct", Name: "direct token", Policies: []string{"read-jobs"}},
		{AccessorID: "role", Name: "role token", Roles: []*api.ACLTokenRoleLink{{ID: "role-b"}}},
		{AccessorID: "unrelated", Name: "unrelated token", Policies: []string{"other-policy"}},
	}

	relatedTokens, relatedRoles := policyRelationships("read-jobs", tokens, roles)
	must.Len(t, 2, relatedTokens)
	must.Eq(t, "direct token", relatedTokens[0].Name)
	must.Eq(t, "direct", relatedTokens[0].Via)
	must.Eq(t, "role token", relatedTokens[1].Name)
	must.Eq(t, "role builders", relatedTokens[1].Via)
	must.Len(t, 1, relatedRoles)
	must.Eq(t, "builders", relatedRoles[0].Name)
}

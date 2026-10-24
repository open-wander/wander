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

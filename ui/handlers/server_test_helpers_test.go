package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newServersTestHandler(t *testing.T, membersResponse string) *ServersHandler {
	t.Helper()
	responses := map[string]string{
		"/v1/agent/members": membersResponse,
		"/v1/status/leader": `"127.0.0.1:4647"`,
		"/v1/agent/self":    `{"member":{"Name":"different-server"}}`,
		"/v1/namespaces":    `[]`,
		"/v1/regions":       `[]`,
	}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(backend.Close)
	return NewServersHandler(newJobsTestHandler(t, backend.URL))
}

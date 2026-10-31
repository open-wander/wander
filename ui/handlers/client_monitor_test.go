package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestNormalizeAgentMonitorLevel(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "", want: "info", ok: true},
		{input: " DEBUG ", want: "debug", ok: true},
		{input: "trace", want: "trace", ok: true},
		{input: "fatal", want: "fatal", ok: false},
	} {
		got, ok := normalizeAgentMonitorLevel(test.input)
		if got != test.want || ok != test.ok {
			t.Fatalf("normalizeAgentMonitorLevel(%q) = (%q, %t), want (%q, %t)", test.input, got, ok, test.want, test.ok)
		}
	}
}

func TestParseClientMonitorPath(t *testing.T) {
	for _, test := range []struct {
		path     string
		clientID string
		stream   bool
		ok       bool
	}{
		{path: "/ui/clients/node-1/monitor", clientID: "node-1", ok: true},
		{path: "/ui/clients/node-1/monitor/stream", clientID: "node-1", stream: true, ok: true},
		{path: "/ui/clients/node-1/monitor/extra"},
		{path: "/ui/clients//monitor"},
	} {
		clientID, stream, ok := parseClientMonitorPath(test.path)
		if clientID != test.clientID || stream != test.stream || ok != test.ok {
			t.Fatalf("parseClientMonitorPath(%q) = (%q, %t, %t), want (%q, %t, %t)", test.path, clientID, stream, ok, test.clientID, test.stream, test.ok)
		}
	}
}

func TestIsClientMonitorRoute(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{path: "/ui/clients/node-1/monitor", want: true},
		{path: "/ui/clients/node-1/monitor/stream", want: true},
		{path: "/ui/clients/node-1/monitor/not-a-route", want: true},
		{path: "/ui/clients/monitoring"},
		{path: "/ui/clients/node-1"},
	} {
		if got := isClientMonitorRoute(test.path); got != test.want {
			t.Fatalf("isClientMonitorRoute(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestClientMonitorPageAndPermissionStates(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/node/node-1":
			_, _ = w.Write([]byte(`{"ID":"node-1","Name":"client-one"}`))
		case "/v1/namespaces", "/v1/regions":
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()
	handler := NewClientsHandler(newJobsTestHandler(t, backend.URL))

	for _, htmx := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodGet, "/ui/clients/node-1/monitor?level=debug", nil)
		if htmx {
			request.Header.Set("HX-Request", "true")
			request = request.WithContext(WithHTMXContext(request.Context(), true))
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("monitor page HTMX=%t status = %d; body: %s", htmx, response.Code, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, "Agent Monitor") || !strings.Contains(body, `agentMonitor(&#34;clients&#34;, &#34;node-1&#34;, &#34;debug&#34;, &#34;&#34;)`) {
			t.Fatalf("monitor page HTMX=%t missing live monitor configuration: %s", htmx, body)
		}
		if strings.Contains(strings.ToLower(body), "<!doctype html") == htmx {
			t.Fatalf("monitor page HTMX=%t returned wrong document shape", htmx)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/ui/clients/node-1/monitor", nil)
	request = request.WithContext(WithPermissions(request.Context(), NewUserPermissions(&ACLTokenInfo{
		Token:      &api.ACLToken{Type: "client"},
		IsValid:    true,
		ACLEnabled: true,
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertHTTPResponse(t, response, http.StatusForbidden, "agent:read")
}

func TestClientMonitorStreamForwardsScopeAndFrames(t *testing.T) {
	requests := make(chan *http.Request, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/monitor" {
			http.NotFound(w, r)
			return
		}
		requests <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"Offset\":0,\"Data\":\"aGVsbG8K\",\"FileEvent\":\"log\"}\n"))
	}))
	defer backend.Close()

	handler := NewClientsHandler(newJobsTestHandler(t, backend.URL))
	request := httptest.NewRequest(http.MethodGet, "/ui/clients/node-1/monitor/stream?level=warn&region=region-a", nil)
	ctx := context.WithValue(request.Context(), wanderTokenKey, "secret-token")
	ctx = context.WithValue(ctx, wanderRegionKey, "region-a")
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("monitor stream response = %d %q; body: %s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `event: log`) || !strings.Contains(response.Body.String(), `"message":"hello\n"`) {
		t.Fatalf("monitor stream did not forward decoded log frame: %s", response.Body.String())
	}

	upstream := <-requests
	assertMonitorQueryValue(t, upstream, "node_id", "node-1")
	assertMonitorQueryValue(t, upstream, "log_level", "warn")
	assertMonitorQueryValue(t, upstream, "region", "region-a")
	if got := upstream.Header.Get(XNomadToken); got != "secret-token" {
		t.Fatalf("upstream ACL token = %q, want secret-token", got)
	}
}

func assertMonitorQueryValue(t *testing.T, request *http.Request, key, want string) {
	t.Helper()
	if got := request.URL.Query().Get(key); got != want {
		t.Fatalf("upstream %s = %q, want %q", key, got, want)
	}
}

func assertRequestHeaderValue(t *testing.T, request *http.Request, key, want string) {
	t.Helper()
	if got := request.Header.Get(key); got != want {
		t.Errorf("header %q = %q, want %q", key, got, want)
	}
}

func assertScopedBrowserRequest(t *testing.T, request *http.Request) {
	t.Helper()
	assertRequestHeaderValue(t, request, "X-Nomad-Token", "browser-token")
	assertMonitorQueryValue(t, request, "namespace", "team-a")
	assertMonitorQueryValue(t, request, "region", "west")
}

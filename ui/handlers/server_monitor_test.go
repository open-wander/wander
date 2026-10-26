package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestParseServerMonitorPath(t *testing.T) {
	for _, test := range []struct {
		path       string
		serverName string
		stream     bool
		ok         bool
	}{
		{path: "/ui/servers/server-1/monitor", serverName: "server-1", ok: true},
		{path: "/ui/servers/server-1/monitor/stream", serverName: "server-1", stream: true, ok: true},
		{path: "/ui/servers/server-1/monitor/extra"},
		{path: "/ui/servers//monitor"},
	} {
		serverName, stream, ok := parseServerMonitorPath(test.path)
		if serverName != test.serverName || stream != test.stream || ok != test.ok {
			t.Fatalf("parseServerMonitorPath(%q) = (%q, %t, %t), want (%q, %t, %t)", test.path, serverName, stream, ok, test.serverName, test.stream, test.ok)
		}
	}
}

func TestServerMonitorPageAndPermissionStates(t *testing.T) {
	handler := newServersTestHandler(t, `{"ServerRegion":"region-a","ServerDC":"dc1","Members":[{"Name":"server-1","Addr":"127.0.0.1","Port":4648,"Status":"alive","Tags":{"region":"region-a","dc":"dc1"}}]}`)

	for _, htmx := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodGet, "/ui/servers/server-1/monitor?level=warn", nil)
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
		if !strings.Contains(body, "Agent Monitor") || !strings.Contains(body, `agentMonitor(&#34;servers&#34;, &#34;server-1&#34;, &#34;warn&#34;, &#34;region-a&#34;)`) {
			t.Fatalf("monitor page HTMX=%t missing live monitor configuration: %s", htmx, body)
		}
		if strings.Contains(strings.ToLower(body), "<!doctype html") == htmx {
			t.Fatalf("monitor page HTMX=%t returned wrong document shape", htmx)
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/ui/servers/server-1/monitor", nil)
	request = request.WithContext(WithPermissions(request.Context(), NewUserPermissions(&ACLTokenInfo{
		Token:      &api.ACLToken{Type: "client"},
		IsValid:    true,
		ACLEnabled: true,
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assertHTTPResponse(t, response, http.StatusForbidden, "agent:read")
}

func TestServerMonitorStreamForwardsScopeAndFrames(t *testing.T) {
	requests := make(chan *http.Request, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/agent/monitor" {
			http.NotFound(w, r)
			return
		}
		requests <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"Offset\":0,\"Data\":\"c2VydmVyIGxvZwo=\",\"FileEvent\":\"log\"}\n"))
	}))
	defer backend.Close()

	handler := NewServersHandler(newJobsTestHandler(t, backend.URL))
	request := httptest.NewRequest(http.MethodGet, "/ui/servers/server-1/monitor/stream?level=error&region=region-a", nil)
	ctx := context.WithValue(request.Context(), wanderTokenKey, "secret-token")
	ctx = context.WithValue(ctx, wanderRegionKey, "region-a")
	request = request.WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("monitor stream response = %d %q; body: %s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `event: log`) || !strings.Contains(response.Body.String(), `"message":"server log\n"`) {
		t.Fatalf("monitor stream did not forward decoded log frame: %s", response.Body.String())
	}

	upstream := <-requests
	assertMonitorQueryValue(t, upstream, "server_id", "server-1")
	assertMonitorQueryValue(t, upstream, "log_level", "error")
	assertMonitorQueryValue(t, upstream, "region", "region-a")
	if got := upstream.Header.Get(XNomadToken); got != "secret-token" {
		t.Fatalf("upstream ACL token = %q, want secret-token", got)
	}
}

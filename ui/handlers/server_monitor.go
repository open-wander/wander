package handlers

import (
	"net/http"
	"net/url"

	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/servers"
	"github.com/open-wander/wander/ui/templates/partials/serverspartials"
)

func parseServerMonitorPath(path string) (serverName string, stream bool, ok bool) {
	return parseAgentMonitorPath(path, "/ui/servers/")
}

func isServerMonitorRoute(path string) bool {
	return isAgentMonitorRoute(path, "/ui/servers/")
}

func (h *ServersHandler) serverMonitorPage(w http.ResponseWriter, r *http.Request, serverName, level string) {
	if !h.handler.canReadAgentMonitor(w, r) {
		return
	}
	server, found := h.getServerDetail(r, serverName)
	if !found {
		h.handler.RenderNotFound(w, r, NotFoundError{Message: "The requested server was not found.", RequestPath: r.URL.Path})
		return
	}
	props := serverspartials.ServerMonitorProps{ServerName: server.Name, Region: server.Region, Level: level}
	layoutProps := h.handler.BaseLayoutProps(r,
		server.Name+" - Server Monitor - Wander",
		"Live agent logs for "+server.Name,
		"/ui/servers/"+url.PathEscape(server.Name)+"/monitor",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Servers", URL: "/ui/servers"},
			{Label: server.Name, URL: "/ui/servers/" + url.PathEscape(server.Name)},
			{Label: "Monitor", URL: ""},
		},
	)
	if err := Render(w, r, servers.Monitor(layoutProps, props), serverspartials.ServerMonitorContent(props)); err != nil {
		h.handler.logger.Error("failed to render server monitor", "serverName", serverName, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (h *ServersHandler) streamServerMonitor(w http.ResponseWriter, r *http.Request, serverName, level string) {
	if !h.handler.canReadAgentMonitor(w, r) {
		return
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander API client is unavailable", http.StatusServiceUnavailable)
		return
	}
	h.handler.streamAgentMonitor(w, r, client, "server_id", serverName, level, "server")
}

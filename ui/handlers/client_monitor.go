package handlers

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/open-wander/wander/api"
	"github.com/open-wander/wander/ui/templates/components"
	"github.com/open-wander/wander/ui/templates/pages/clients"
	"github.com/open-wander/wander/ui/templates/partials/clientspartials"
)

const defaultAgentMonitorLevel = "info"

var agentMonitorLevels = map[string]struct{}{
	"trace": {},
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

// Monitor serves the client agent-log page and its same-origin SSE stream.
func (h *ClientsHandler) Monitor(w http.ResponseWriter, r *http.Request) {
	clientID, stream, ok := parseClientMonitorPath(r.URL.Path)
	if !ok {
		h.handler.RenderNotFound(w, r, NotFoundError{
			Message:     "The client monitor page you requested could not be found.",
			RequestPath: r.URL.Path,
		})
		return
	}

	level, ok := normalizeAgentMonitorLevel(r.URL.Query().Get("level"))
	if !ok {
		http.Error(w, "Invalid level: expected trace, debug, info, warn, or error", http.StatusBadRequest)
		return
	}

	if stream {
		h.streamClientMonitor(w, r, clientID, level)
		return
	}
	h.clientMonitorPage(w, r, clientID, level)
}

func parseClientMonitorPath(path string) (clientID string, stream bool, ok bool) {
	parts := clientMonitorPathParts(path)
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] != "monitor" {
		return
	}
	if len(parts) == 3 && parts[2] != "stream" {
		return
	}
	clientID, err := url.PathUnescape(parts[0])
	if err != nil || clientID == "" || strings.Contains(clientID, "/") {
		return "", false, false
	}
	return clientID, len(parts) == 3, true
}

func isClientMonitorRoute(path string) bool {
	parts := clientMonitorPathParts(path)
	return strings.HasPrefix(path, "/ui/clients/") && len(parts) >= 2 && parts[1] == "monitor"
}

func clientMonitorPathParts(path string) []string {
	remainder := strings.TrimPrefix(path, "/ui/clients/")
	return strings.Split(strings.Trim(remainder, "/"), "/")
}

func normalizeAgentMonitorLevel(level string) (string, bool) {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		return defaultAgentMonitorLevel, true
	}
	_, ok := agentMonitorLevels[level]
	return level, ok
}

func (h *ClientsHandler) clientMonitorPage(w http.ResponseWriter, r *http.Request, clientID, level string) {
	client := h.authorizedMonitorClient(w, r)
	if client == nil {
		return
	}
	nodeQuery := (&api.QueryOptions{
		Region:    GetRegionFromRequest(r),
		AuthToken: GetTokenFromRequest(r),
	}).WithContext(r.Context())
	node, _, err := client.Nodes().Info(clientID, nodeQuery)
	if err != nil {
		switch {
		case apiResponseHasStatus(err, http.StatusForbidden):
			h.handler.RenderForbidden(w, r, ForbiddenError{Message: "You do not have permission to view this client", Capability: "node:read"})
		case apiResponseHasStatus(err, http.StatusNotFound):
			h.handler.RenderNotFound(w, r, NotFoundError{Message: "The requested client was not found.", RequestPath: r.URL.Path})
		default:
			h.renderMonitorError(w, http.StatusBadGateway, "Failed to fetch client from Wander API", "failed to fetch client for monitor", clientID, err)
		}
		return
	}

	name := node.Name
	if name == "" {
		name = clientID
	}
	props := clientspartials.ClientMonitorProps{ClientID: clientID, ClientName: name, Level: level}
	layoutProps := h.handler.BaseLayoutProps(r,
		name+" - Client Monitor - Wander",
		"Live agent logs for "+name,
		"/ui/clients/"+url.PathEscape(clientID)+"/monitor",
		[]components.Breadcrumb{
			{Label: "Home", URL: "/ui/"},
			{Label: "Clients", URL: "/ui/clients"},
			{Label: name, URL: "/ui/clients/" + url.PathEscape(clientID)},
			{Label: "Monitor", URL: ""},
		},
	)

	if err := Render(w, r, clients.Monitor(layoutProps, props), clientspartials.ClientMonitorContent(props)); err != nil {
		h.renderMonitorError(w, http.StatusInternalServerError, "Internal server error", "failed to render client monitor", clientID, err)
	}
}

func (h *ClientsHandler) authorizedMonitorClient(w http.ResponseWriter, r *http.Request) *api.Client {
	if !h.canReadClientMonitor(w, r) {
		return nil
	}
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		http.Error(w, "Wander API client is unavailable", http.StatusServiceUnavailable)
	}
	return client
}

func (h *ClientsHandler) renderMonitorError(w http.ResponseWriter, status int, response, message, clientID string, err error) {
	h.handler.logger.Error(message, "clientID", clientID, "error", err)
	http.Error(w, response, status)
}

func (h *ClientsHandler) canReadClientMonitor(w http.ResponseWriter, r *http.Request) bool {
	perms := GetPermissionsFromRequest(r)
	if perms == nil || perms.CanReadAgent() {
		return true
	}
	h.handler.RenderForbidden(w, r, ForbiddenError{
		Message:    "You do not have permission to monitor agent logs",
		Capability: "agent:read",
	})
	return false
}

func (h *ClientsHandler) streamClientMonitor(w http.ResponseWriter, r *http.Request, clientID, level string) {
	client := h.authorizedMonitorClient(w, r)
	if client == nil {
		return
	}

	stop := make(chan struct{})
	var stopOnce sync.Once
	stopMonitor := func() { stopOnce.Do(func() { close(stop) }) }
	cancelContextStop := context.AfterFunc(r.Context(), stopMonitor)
	defer cancelContextStop()
	defer stopMonitor()
	query := (&api.QueryOptions{
		Region:    GetRegionFromRequest(r),
		AuthToken: GetTokenFromRequest(r),
		Params: map[string]string{
			"node_id":   clientID,
			"log_level": level,
		},
	}).WithContext(r.Context())
	frames, errs := client.Agent().Monitor(stop, query)
	if frames == nil {
		err := <-errs
		if apiResponseHasStatus(err, http.StatusForbidden) {
			h.handler.RenderForbidden(w, r, ForbiddenError{Message: "You do not have permission to monitor agent logs", Capability: "agent:read"})
			return
		}
		h.renderMonitorError(w, http.StatusBadGateway, "Failed to open client monitor", "failed to open client monitor", clientID, err)
		return
	}

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer sse.Close()

	for frames != nil || errs != nil {
		select {
		case <-r.Context().Done():
			return
		case <-sse.Done():
			return
		case frame, open := <-frames:
			if !open {
				frames = nil
				continue
			}
			if frame == nil || frame.IsHeartbeat() {
				continue
			}
			if err := sse.WriteJSON("log", map[string]string{"message": string(frame.Data)}); err != nil {
				return
			}
		case streamErr, open := <-errs:
			if !open {
				errs = nil
				continue
			}
			if streamErr == nil || streamErr == io.EOF || r.Context().Err() != nil {
				return
			}
			_ = sse.WriteJSON("error", map[string]string{"message": streamErr.Error()})
			return
		}
	}
}

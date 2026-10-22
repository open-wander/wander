package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/open-wander/wander/api"
)

// WebSocket message types for exec terminal communication
const (
	MsgTypeInput  = "input"  // Terminal input from client -> Nomad exec
	MsgTypeOutput = "output" // Nomad exec output -> Terminal
	MsgTypeResize = "resize" // Terminal size change -> Nomad exec
	MsgTypeError  = "error"  // Error message
	MsgTypeExit   = "exit"   // Session ended
	MsgTypeClose  = "close"  // Client requests close
)

// ExecMessage represents a WebSocket message for exec communication.
// All messages are JSON encoded with a type field indicating the message purpose.
type ExecMessage struct {
	Type    string `json:"type"`              // Message type: input, output, resize, error, exit, close
	Data    string `json:"data,omitempty"`    // For input/output: the terminal data
	Message string `json:"message,omitempty"` // For error messages
	Code    int    `json:"code,omitempty"`    // For exit: the exit code
	Cols    int    `json:"cols,omitempty"`    // For resize: terminal columns
	Rows    int    `json:"rows,omitempty"`    // For resize: terminal rows
}

// ExecHandler handles WebSocket connections for exec sessions.
type ExecHandler struct {
	handler  *Handler
	upgrader websocket.Upgrader
}

// NewExecHandler creates a new exec WebSocket handler.
func NewExecHandler(h *Handler) *ExecHandler {
	return &ExecHandler{
		handler: h,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
	}
}

// ServeHTTP handles WebSocket connections for exec.
// Route: GET /ui/allocations/{id}/exec/ws?task=<task>&command=<cmd>&tty=<bool>
func (h *ExecHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle WebSocket upgrade requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract allocation ID from path: /ui/allocations/{id}/exec/ws
	path := strings.TrimPrefix(r.URL.Path, "/ui/allocations/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[1] != "exec" || parts[2] != "ws" {
		http.Error(w, "Invalid exec WebSocket path", http.StatusBadRequest)
		return
	}
	allocID := parts[0]

	// Get query parameters
	task := r.URL.Query().Get("task")
	commandStr := r.URL.Query().Get("command")
	if commandStr == "" {
		commandStr = "/bin/sh"
	}

	// Parse command - for now, treat as single string
	// In production, may want to parse as JSON array
	command := []string{commandStr}

	// Parse TTY flag (default: true for interactive terminal)
	tty := true
	if ttyStr := r.URL.Query().Get("tty"); ttyStr == "false" {
		tty = false
	}

	// Validate required parameters
	if task == "" {
		h.handler.logger.Warn("exec missing task parameter", "allocID", allocID)
		http.Error(w, "Missing required parameter: task", http.StatusBadRequest)
		return
	}

	// Check permissions - need alloc-exec capability
	perms := GetPermissionsFromRequest(r)
	token := GetTokenFromRequest(r)

	// Get allocation to determine namespace
	client := h.handler.NomadClientForRequest(r)
	if client == nil {
		h.handler.logger.Error("no Nomad client available for exec")
		http.Error(w, "Wander client not available", http.StatusServiceUnavailable)
		return
	}

	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	alloc, _, err := client.Allocations().Info(allocID, queryOpts)
	if err != nil {
		h.handler.logger.Error("failed to fetch allocation for exec", "allocID", allocID, "error", err)
		http.Error(w, "Failed to fetch allocation", http.StatusInternalServerError)
		return
	}

	namespace := alloc.Namespace
	if namespace == "" {
		namespace = GetNamespaceFromRequest(r)
	}

	// Check alloc-exec permission
	if perms != nil && !perms.CanExec(namespace) {
		h.handler.logger.Warn("exec permission denied", "allocID", allocID, "namespace", namespace)
		http.Error(w, "Permission denied: alloc-exec capability required", http.StatusForbidden)
		return
	}

	h.handler.logger.Info("exec WebSocket connection",
		"allocID", allocID,
		"task", task,
		"command", command,
		"tty", tty,
		"namespace", namespace,
		"remoteAddr", r.RemoteAddr,
	)

	// Upgrade to WebSocket
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.handler.logger.Error("failed to upgrade WebSocket", "error", err)
		return
	}

	// Handle the exec session with real Nomad API
	h.handleSession(r, ws, alloc, task, command, tty, token)
}

// handleSession manages the bidirectional communication for an exec session.
// Connects to Nomad's exec API and proxies data between client WebSocket and Nomad.
func (h *ExecHandler) handleSession(r *http.Request, clientWS *websocket.Conn, alloc *api.Allocation, task string, command []string, tty bool, token string) {
	defer clientWS.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create pipes for stdin/stdout/stderr
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	stderrReader, stderrWriter := io.Pipe()

	// Terminal size channel for TTY resize events
	termSizeCh := make(chan api.TerminalSize, 1)

	// Read from the browser until the exec command finishes or the browser
	// disconnects. This goroutine is released when the WebSocket closes below;
	// waiting for it before sending the exit message would deadlock because the
	// browser waits for that message before disconnecting.
	go func() {
		defer stdinWriter.Close()
		h.readFromClientWebSocket(clientWS, stdinWriter, termSizeCh, cancel)
	}()

	// Track only output readers so all command output reaches the browser before
	// the exit message is sent.
	var outputWG sync.WaitGroup
	h.startExecOutputReader(&outputWG, clientWS, stdoutReader, cancel)

	// Goroutine to read from Nomad stderr and write to client WebSocket (if not TTY)
	if !tty {
		h.startExecOutputReader(&outputWG, clientWS, stderrReader, cancel)
	}

	// Build query options with token
	queryOpts := &api.QueryOptions{}
	if token != "" {
		queryOpts.AuthToken = token
	}

	// Call Nomad exec API - this blocks until command completes
	client := h.handler.NomadClientForRequest(r)
	exitCode, err := client.Allocations().Exec(
		ctx,
		alloc,
		task,
		tty,
		command,
		stdinReader,
		stdoutWriter,
		stderrWriter,
		termSizeCh,
		queryOpts,
	)

	// Close output pipes to signal readers to stop
	stdoutWriter.Close()
	stderrWriter.Close()

	// Wait for all output to be delivered before reporting command completion.
	outputWG.Wait()

	// Send exit message to client
	exitMsg := &ExecMessage{
		Type: MsgTypeExit,
		Code: exitCode,
	}

	if err != nil {
		h.handler.logger.Error("exec command failed", "allocID", alloc.ID, "task", task, "error", err)
		exitMsg.Message = err.Error()
		// If there's an error, also send an error message
		errMsg := &ExecMessage{
			Type:    MsgTypeError,
			Message: err.Error(),
		}
		h.sendMessageToClient(clientWS, errMsg)
	}

	h.sendMessageToClient(clientWS, exitMsg)

	h.handler.logger.Info("exec session completed",
		"allocID", alloc.ID,
		"task", task,
		"exitCode", exitCode,
	)
}

func (h *ExecHandler) startExecOutputReader(wg *sync.WaitGroup, ws *websocket.Conn, reader io.Reader, cancel context.CancelFunc) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.writeToClientWebSocket(ws, reader, MsgTypeOutput, cancel)
	}()
}

// readFromClientWebSocket reads messages from the client WebSocket and processes them.
// Handles input data and TTY resize events.
func (h *ExecHandler) readFromClientWebSocket(ws *websocket.Conn, stdinWriter io.WriteCloser, termSizeCh chan<- api.TerminalSize, cancel context.CancelFunc) {
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
				h.handler.logger.Error("WebSocket read error", "error", err)
			}
			cancel()
			return
		}

		// Parse the message
		var msg ExecMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			h.handler.logger.Warn("invalid message format", "error", err)
			continue
		}

		switch msg.Type {
		case MsgTypeInput:
			// Write input to stdin pipe (goes to Nomad)
			if msg.Data != "" {
				_, err := stdinWriter.Write([]byte(msg.Data))
				if err != nil {
					h.handler.logger.Error("failed to write stdin", "error", err)
					cancel()
					return
				}
			}

		case MsgTypeResize:
			// Send terminal resize event
			if msg.Cols > 0 && msg.Rows > 0 {
				select {
				case termSizeCh <- api.TerminalSize{Height: msg.Rows, Width: msg.Cols}:
				default:
					// Channel full, skip this resize event
				}
			}

		case MsgTypeClose:
			h.handler.logger.Info("client requested close")
			cancel()
			return

		default:
			h.handler.logger.Warn("unknown message type", "type", msg.Type)
		}
	}
}

// writeToClientWebSocket reads from a pipe and writes to the client WebSocket.
// Used for stdout and stderr streams.
func (h *ExecHandler) writeToClientWebSocket(ws *websocket.Conn, reader io.Reader, msgType string, cancel context.CancelFunc) {
	buf := make([]byte, 4096)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Send data to client
			msg := &ExecMessage{
				Type: msgType,
				Data: string(buf[:n]),
			}

			if err := h.sendMessageToClient(ws, msg); err != nil {
				h.handler.logger.Error("failed to send to client", "error", err)
				cancel()
				return
			}
		}

		if err != nil {
			if err != io.EOF {
				h.handler.logger.Error("failed to read output", "error", err)
			}
			return
		}
	}
}

// sendMessageToClient sends a message to the WebSocket client.
func (h *ExecHandler) sendMessageToClient(ws *websocket.Conn, msg *ExecMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return ws.WriteMessage(websocket.TextMessage, data)
}

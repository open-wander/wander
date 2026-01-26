package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

// ExecSession represents an active exec session.
// In production, this would manage the connection to Nomad's exec API.
type ExecSession struct {
	allocID string
	task    string
	command string
	cols    int
	rows    int
	ws      *websocket.Conn
	done    chan struct{}
	mu      sync.Mutex
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
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// In production, add proper origin checking
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// ServeHTTP handles WebSocket connections for exec.
// Route: GET /ui/allocations/{id}/exec/ws?task=<task>&command=<cmd>
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
	command := r.URL.Query().Get("command")
	if command == "" {
		command = "/bin/sh"
	}

	h.handler.logger.Info("exec WebSocket connection",
		"allocID", allocID,
		"task", task,
		"command", command,
		"remoteAddr", r.RemoteAddr,
	)

	// Upgrade to WebSocket
	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.handler.logger.Error("failed to upgrade WebSocket", "error", err)
		return
	}

	// Create exec session
	session := &ExecSession{
		allocID: allocID,
		task:    task,
		command: command,
		cols:    80,  // default
		rows:    24,  // default
		ws:      ws,
		done:    make(chan struct{}),
	}

	// Handle the exec session
	h.handleSession(session)
}

// handleSession manages the bidirectional communication for an exec session.
// In production, this would connect to Nomad's exec API and proxy data.
func (h *ExecHandler) handleSession(session *ExecSession) {
	defer func() {
		close(session.done)
		session.ws.Close()
		h.handler.logger.Info("exec session closed",
			"allocID", session.allocID,
			"task", session.task,
		)
	}()

	// Start a goroutine for reading from the WebSocket (client -> server)
	go h.readFromClient(session)

	// For mock implementation: start a demo session
	h.runMockSession(session)
}

// readFromClient reads messages from the WebSocket client.
func (h *ExecHandler) readFromClient(session *ExecSession) {
	for {
		select {
		case <-session.done:
			return
		default:
			_, message, err := session.ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.handler.logger.Error("WebSocket read error", "error", err)
				}
				return
			}

			// Parse the message
			var msg ExecMessage
			if err := json.Unmarshal(message, &msg); err != nil {
				h.handler.logger.Warn("invalid message format", "error", err)
				continue
			}

			h.handleClientMessage(session, &msg)
		}
	}
}

// handleClientMessage processes messages from the client.
func (h *ExecHandler) handleClientMessage(session *ExecSession, msg *ExecMessage) {
	switch msg.Type {
	case MsgTypeInput:
		// In production: forward input to Nomad exec
		h.handler.logger.Debug("exec input received",
			"allocID", session.allocID,
			"data", msg.Data,
		)
		// For mock: echo back the input (simulate a shell)
		h.handleMockInput(session, msg.Data)

	case MsgTypeResize:
		// Update terminal dimensions
		session.mu.Lock()
		session.cols = msg.Cols
		session.rows = msg.Rows
		session.mu.Unlock()

		h.handler.logger.Debug("exec resize received",
			"allocID", session.allocID,
			"cols", msg.Cols,
			"rows", msg.Rows,
		)
		// In production: send resize to Nomad exec

	case MsgTypeClose:
		h.handler.logger.Info("client requested close",
			"allocID", session.allocID,
		)
		// Signal session to close
		select {
		case <-session.done:
		default:
			// Send exit message before closing
			h.sendMessage(session, &ExecMessage{
				Type: MsgTypeExit,
				Code: 0,
			})
		}

	default:
		h.handler.logger.Warn("unknown message type",
			"type", msg.Type,
		)
	}
}

// sendMessage sends a message to the WebSocket client.
func (h *ExecHandler) sendMessage(session *ExecSession, msg *ExecMessage) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return session.ws.WriteMessage(websocket.TextMessage, data)
}

// sendOutput sends terminal output to the client.
func (h *ExecHandler) sendOutput(session *ExecSession, output string) error {
	return h.sendMessage(session, &ExecMessage{
		Type: MsgTypeOutput,
		Data: output,
	})
}

// runMockSession runs a mock exec session for demonstration purposes.
// This simulates a basic shell with some predefined responses.
func (h *ExecHandler) runMockSession(session *ExecSession) {
	// Send initial prompt
	time.Sleep(100 * time.Millisecond) // Small delay for realism
	h.sendOutput(session, "\r\n")
	h.sendOutput(session, "\033[32m"+session.task+"\033[0m@\033[34malloc-"+session.allocID[:8]+"\033[0m:~$ ")

	// Wait for session to end
	<-session.done
}

// handleMockInput simulates shell input processing.
// This is a mock implementation - in production, input would be sent to Nomad exec.
func (h *ExecHandler) handleMockInput(session *ExecSession, input string) {
	// Echo the input character by character (simulating terminal behavior)
	for _, char := range input {
		switch char {
		case '\r', '\n':
			// Enter pressed - process command
			h.sendOutput(session, "\r\n")
			h.processMockCommand(session, "")
		case '\x03':
			// Ctrl+C
			h.sendOutput(session, "^C\r\n")
			h.sendOutput(session, "\033[32m"+session.task+"\033[0m@\033[34malloc-"+session.allocID[:8]+"\033[0m:~$ ")
		case '\x04':
			// Ctrl+D - exit
			h.sendOutput(session, "\r\n")
			h.sendOutput(session, "\033[33mlogout\033[0m\r\n")
			h.sendMessage(session, &ExecMessage{
				Type: MsgTypeExit,
				Code: 0,
			})
		case '\x7f', '\b':
			// Backspace - send cursor back, space, cursor back
			h.sendOutput(session, "\b \b")
		default:
			// Echo the character
			h.sendOutput(session, string(char))
		}
	}
}

// processMockCommand simulates command execution.
func (h *ExecHandler) processMockCommand(session *ExecSession) {
	// For the mock, we just show a new prompt
	// In a real implementation, we'd parse the command and show appropriate output
	h.sendOutput(session, "\033[32m"+session.task+"\033[0m@\033[34malloc-"+session.allocID[:8]+"\033[0m:~$ ")
}

package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-hclog"
	"github.com/open-wander/wander/api"
)

func TestExecHandlerUsesDefaultSameOriginPolicy(t *testing.T) {
	handler := NewExecHandler(NewHandler(hclog.NewNullLogger()))
	if handler.upgrader.CheckOrigin != nil {
		t.Fatal("exec WebSocket must use Gorilla's default same-origin policy")
	}
}

func TestExecCancellationIsNotReportedAsCommandFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !execSessionCanceled(ctx, errors.New("transport stopped")) {
		t.Fatal("canceled exec context was not classified as an intentional disconnect")
	}
	if !execSessionCanceled(context.Background(), context.Canceled) {
		t.Fatal("context-canceled exec error was not classified as an intentional disconnect")
	}
	if execSessionCanceled(context.Background(), errors.New("command failed")) {
		t.Fatal("command failure was classified as an intentional disconnect")
	}
}

func TestExecClientFramesForwardResizeAndClose(t *testing.T) {
	handler := NewExecHandler(NewHandler(hclog.NewNullLogger()))
	resizeCh := make(chan api.TerminalSize, 1)
	canceled := make(chan struct{})
	done := make(chan struct{})
	var cancelOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connection, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade test WebSocket: %v", err)
			return
		}
		reader, writer := io.Pipe()
		defer reader.Close()
		handler.readFromClientWebSocket(connection, writer, resizeCh, func() {
			cancelOnce.Do(func() { close(canceled) })
		})
		_ = connection.Close()
		close(done)
	}))
	defer server.Close()

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial test WebSocket: %v", err)
	}
	defer connection.Close()

	if err := connection.WriteJSON(ExecMessage{Type: MsgTypeResize, Cols: 132, Rows: 43}); err != nil {
		t.Fatalf("write resize frame: %v", err)
	}
	select {
	case size := <-resizeCh:
		if size.Width != 132 || size.Height != 43 {
			t.Fatalf("terminal size = %#v, want width 132 height 43", size)
		}
	case <-time.After(time.Second):
		t.Fatal("resize frame was not forwarded")
	}

	if err := connection.WriteJSON(ExecMessage{Type: MsgTypeClose}); err != nil {
		t.Fatalf("write close frame: %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("close frame did not cancel the exec session")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("client reader did not stop after close frame")
	}
}

func TestExecWebSocketDenialUsesAllocationNamespaceAndScope(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/allocation/allocation-id" {
			t.Errorf("unexpected API path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("region") != "region-a" {
			t.Errorf("allocation region = %q", r.URL.Query().Get("region"))
		}
		if token := r.Header.Get("X-Nomad-Token"); token != "browser-token" {
			t.Errorf("allocation token = %q", token)
		}
		_, _ = w.Write([]byte(`{"ID":"allocation-id","Namespace":"team-a"}`))
	}))
	defer backend.Close()

	execHandler := NewExecHandler(newJobsTestHandler(t, backend.URL))
	permissions := NewUserPermissions(&ACLTokenInfo{
		Token:      &api.ACLToken{Type: "client"},
		IsValid:    true,
		ACLEnabled: true,
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithPermissions(r.Context(), permissions)
		ctx = WithTokenContext(ctx, "browser-token")
		execHandler.ServeHTTP(w, r.WithContext(ctx))
	}))
	defer server.Close()

	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/ui/allocations/allocation-id/exec/ws?task=web&namespace=default&region=region-a"
	connection, response, err := websocket.DefaultDialer.DialContext(context.Background(), endpoint, nil)
	if connection != nil {
		_ = connection.Close()
	}
	if err == nil {
		t.Fatal("exec WebSocket unexpectedly upgraded without alloc-exec")
	}
	if response == nil {
		t.Fatal("exec denial did not return an HTTP response")
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		t.Fatalf("read denial body: %v", readErr)
	}
	if response.StatusCode != http.StatusForbidden || !strings.Contains(string(body), "alloc-exec") {
		t.Fatalf("denial = %d %q, want 403 alloc-exec", response.StatusCode, body)
	}
}

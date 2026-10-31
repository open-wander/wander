package handlers

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSSEHandler(t *testing.T) {
	tests := []struct {
		name        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "successful creation",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/stream", nil)

			sse, err := NewSSEHandler(w, r)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSSEHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("NewSSEHandler() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if sse == nil {
				t.Fatal("NewSSEHandler() returned nil handler")
			}
			defer sse.Close()

			// Verify SSE headers are set
			headers := w.Header()
			tests := []struct {
				header string
				want   string
			}{
				{"Content-Type", "text/event-stream"},
				{"Cache-Control", "no-cache"},
				{"Connection", "keep-alive"},
				{"X-Accel-Buffering", "no"},
			}

			for _, tc := range tests {
				if got := headers.Get(tc.header); got != tc.want {
					t.Errorf("Header %s = %v, want %v", tc.header, got, tc.want)
				}
			}
		})
	}
}

func TestSSEHandler_Write(t *testing.T) {
	tests := []struct {
		name      string
		event     string
		data      string
		wantEvent string
		wantData  string
	}{
		{
			name:      "simple event",
			event:     "message",
			data:      "hello",
			wantEvent: "event: message\n",
			wantData:  "data: hello\n\n",
		},
		{
			name:      "event without name",
			event:     "",
			data:      "test",
			wantEvent: "",
			wantData:  "data: test\n\n",
		},
		{
			name:      "multiline data",
			event:     "log",
			data:      "line1\nline2",
			wantEvent: "event: log\n",
			wantData:  "data: line1\nline2\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/stream", nil)

			sse, err := NewSSEHandler(w, r)
			if err != nil {
				t.Fatalf("NewSSEHandler() error = %v", err)
			}
			defer sse.Close()

			// Clear the retry message from initial setup
			w.Body.Reset()

			if err := sse.Write(tt.event, tt.data); err != nil {
				t.Errorf("Write() error = %v", err)
				return
			}

			got := w.Body.String()
			want := tt.wantEvent + tt.wantData

			if got != want {
				t.Errorf("Write() output = %q, want %q", got, want)
			}
		})
	}
}

func TestSSEHandler_WriteJSON(t *testing.T) {
	type testData struct {
		Message string `json:"message"`
		Level   string `json:"level"`
	}

	tests := []struct {
		name      string
		event     string
		data      interface{}
		wantEvent string
		wantJSON  string
	}{
		{
			name:      "simple object",
			event:     "log",
			data:      testData{Message: "hello", Level: "info"},
			wantEvent: "event: log\n",
			wantJSON:  `{"message":"hello","level":"info"}`,
		},
		{
			name:      "string value",
			event:     "message",
			data:      "test",
			wantEvent: "event: message\n",
			wantJSON:  `"test"`,
		},
		{
			name:      "number value",
			event:     "count",
			data:      42,
			wantEvent: "event: count\n",
			wantJSON:  "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/stream", nil)

			sse, err := NewSSEHandler(w, r)
			if err != nil {
				t.Fatalf("NewSSEHandler() error = %v", err)
			}
			defer sse.Close()

			// Clear the retry message from initial setup
			w.Body.Reset()

			if err := sse.WriteJSON(tt.event, tt.data); err != nil {
				t.Errorf("WriteJSON() error = %v", err)
				return
			}

			got := w.Body.String()
			want := tt.wantEvent + "data: " + tt.wantJSON + "\n\n"

			if got != want {
				t.Errorf("WriteJSON() output = %q, want %q", got, want)
			}
		})
	}
}

func TestSSEHandler_WriteComment(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{
			name:    "simple comment",
			comment: "heartbeat",
			want:    ": heartbeat\n\n",
		},
		{
			name:    "empty comment",
			comment: "",
			want:    ": \n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/stream", nil)

			sse, err := NewSSEHandler(w, r)
			if err != nil {
				t.Fatalf("NewSSEHandler() error = %v", err)
			}
			defer sse.Close()

			// Clear the retry message from initial setup
			w.Body.Reset()

			if err := sse.WriteComment(tt.comment); err != nil {
				t.Errorf("WriteComment() error = %v", err)
				return
			}

			got := w.Body.String()
			if got != tt.want {
				t.Errorf("WriteComment() output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSSEHandler_Close(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream", nil)

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		t.Fatalf("NewSSEHandler() error = %v", err)
	}

	// Close should succeed
	if err := sse.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Multiple closes should be safe
	if err := sse.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}

	// Writing after close should fail
	err = sse.Write("test", "data")
	if err == nil {
		t.Error("Write() after Close() should return error")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("Write() error = %v, want error containing 'closed'", err)
	}
}

func TestSSEHandler_ContextCancellation(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest("GET", "/stream", nil).WithContext(ctx)

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		t.Fatalf("NewSSEHandler() error = %v", err)
	}
	defer sse.Close()

	// Cancel the context
	cancel()

	// Wait a bit for cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	// Done channel should be closed
	select {
	case <-sse.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Done() channel not closed after context cancellation")
	}

	// Err should return context.Canceled
	if err := sse.Err(); err != context.Canceled {
		t.Errorf("Err() = %v, want %v", err, context.Canceled)
	}
}

func TestSSEHandler_WithOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *SSEOptions
		wantErr bool
	}{
		{
			name: "with heartbeat",
			opts: &SSEOptions{
				HeartbeatInterval: 100 * time.Millisecond,
				RetryInterval:     1 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "no heartbeat",
			opts: &SSEOptions{
				HeartbeatInterval: 0,
				RetryInterval:     1 * time.Second,
			},
			wantErr: false,
		},
		{
			name:    "nil options uses defaults",
			opts:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/stream", nil)

			sse, err := NewSSEHandlerWithOptions(w, r, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSSEHandlerWithOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			defer sse.Close()

			// Verify retry was written if specified
			if tt.opts != nil && tt.opts.RetryInterval > 0 {
				sse.mu.Lock()
				output := w.Body.String()
				sse.mu.Unlock()
				expectedRetry := "retry:"
				if !strings.Contains(output, expectedRetry) {
					t.Errorf("Expected retry field in output, got: %q", output)
				}
			}

			// If heartbeat is enabled, verify it fires
			if tt.opts != nil && tt.opts.HeartbeatInterval > 0 {
				sse.mu.Lock()
				w.Body.Reset()
				sse.mu.Unlock()
				time.Sleep(tt.opts.HeartbeatInterval + 50*time.Millisecond)
				sse.mu.Lock()
				output := w.Body.String()
				sse.mu.Unlock()
				if !strings.Contains(output, ": heartbeat") {
					t.Errorf("Expected heartbeat comment, got: %q", output)
				}
			}
		})
	}
}

func TestSSEWriter(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream", nil)

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		t.Fatalf("NewSSEHandler() error = %v", err)
	}
	defer sse.Close()

	// Clear the retry message from initial setup
	w.Body.Reset()

	writer := NewSSEWriter(sse, "output")

	testData := []byte("test output")
	n, err := writer.Write(testData)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}

	if n != len(testData) {
		t.Errorf("Write() wrote %d bytes, want %d", n, len(testData))
	}

	got := w.Body.String()
	want := "event: output\ndata: test output\n\n"

	if got != want {
		t.Errorf("SSEWriter output = %q, want %q", got, want)
	}
}

func TestDefaultSSEOptions(t *testing.T) {
	opts := DefaultSSEOptions()

	if opts == nil {
		t.Fatal("DefaultSSEOptions() returned nil")
	}

	if opts.HeartbeatInterval == 0 {
		t.Error("DefaultSSEOptions().HeartbeatInterval should be non-zero")
	}

	if opts.RetryInterval == 0 {
		t.Error("DefaultSSEOptions().RetryInterval should be non-zero")
	}
}

func TestSSEHandler_ConcurrentWrites(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream", nil)

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		t.Fatalf("NewSSEHandler() error = %v", err)
	}
	defer sse.Close()

	// Test concurrent writes don't cause data races
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 5; j++ {
				_ = sse.Write("test", "data")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Just verify no panics occurred and some data was written
	if w.Body.Len() == 0 {
		t.Error("Expected some data to be written")
	}
}

func TestSSEHandler_Flush(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/stream", nil)

	sse, err := NewSSEHandler(w, r)
	if err != nil {
		t.Fatalf("NewSSEHandler() error = %v", err)
	}
	defer sse.Close()

	// Flush should not panic
	sse.Flush()

	// Flush after close should not panic
	sse.Close()
	sse.Flush()
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/open-wander/wander/api"
)

func TestStreamLogsRejectsInvalidLogType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ui/allocations/alloc/logs/stream?task=web&type=invalid", nil)
	recorder := httptest.NewRecorder()

	(&AllocationLogsHandler{}).StreamLogs(recorder, req, "alloc")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestNormalizeLogType(t *testing.T) {
	tests := []struct {
		input string
		want  string
		valid bool
	}{
		{input: "", want: api.FSLogNameStdout, valid: true},
		{input: api.FSLogNameStdout, want: api.FSLogNameStdout, valid: true},
		{input: api.FSLogNameStderr, want: api.FSLogNameStderr, valid: true},
		{input: "both", want: "both", valid: true},
		{input: "invalid", valid: false},
	}

	for _, test := range tests {
		got, valid := normalizeLogType(test.input)
		if got != test.want || valid != test.valid {
			t.Fatalf("normalizeLogType(%q) = (%q, %t), want (%q, %t)", test.input, got, valid, test.want, test.valid)
		}
	}
}

func TestParseAllocationLogPosition(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		want    allocationLogPosition
		wantErr string
	}{
		{name: "live defaults", want: allocationLogPosition{follow: true, origin: "end"}},
		{name: "head snapshot", query: "?follow=false&origin=start&offset=0", want: allocationLogPosition{origin: "start"}},
		{name: "tail snapshot", query: "?follow=false&origin=end&offset=50000", want: allocationLogPosition{origin: "end", offset: 50000}},
		{name: "invalid follow", query: "?follow=sometimes", wantErr: "invalid follow"},
		{name: "non-canonical follow", query: "?follow=TRUE", wantErr: "invalid follow"},
		{name: "invalid origin", query: "?origin=middle", wantErr: "invalid origin"},
		{name: "invalid offset", query: "?offset=-1", wantErr: "invalid offset"},
		{name: "non-numeric offset", query: "?offset=many", wantErr: "invalid offset"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/ui/allocations/alloc/logs/stream"+test.query, nil)
			got, err := parseAllocationLogPosition(request)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parse error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse position: %v", err)
			}
			if got != test.want {
				t.Fatalf("position = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestAllocationLogEndEvent(t *testing.T) {
	tests := []struct {
		name, status, want string
		follow             bool
	}{
		{name: "finite read", status: api.AllocClientStatusRunning, want: "complete"},
		{name: "completed allocation", status: api.AllocClientStatusComplete, follow: true, want: "complete"},
		{name: "failed allocation", status: api.AllocClientStatusFailed, follow: true, want: "complete"},
		{name: "lost allocation", status: api.AllocClientStatusLost, follow: true, want: "complete"},
		{name: "running task restart", status: api.AllocClientStatusRunning, follow: true, want: "restart"},
		{name: "pending task stream", status: api.AllocClientStatusPending, follow: true, want: "restart"},
		{name: "unknown state", status: "unknown", follow: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := allocationLogEndEvent(test.follow, test.status); got != test.want {
				t.Fatalf("allocationLogEndEvent(%t, %q)=%q, want %q", test.follow, test.status, got, test.want)
			}
		})
	}
}

func TestStreamLogsEmitsLifecycleEvent(t *testing.T) {
	tests := []struct {
		name, query, status, wantEvent string
	}{
		{
			name: "finite snapshot completes", query: "follow=false&origin=start&offset=0",
			status: api.AllocClientStatusRunning, wantEvent: "complete",
		},
		{
			name: "terminal allocation completes", query: "follow=true&origin=end&offset=0",
			status: api.AllocClientStatusComplete, wantEvent: "complete",
		},
		{
			name: "running allocation restarts stream", query: "follow=true&origin=end&offset=0",
			status: api.AllocClientStatusRunning, wantEvent: "restart",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := newAllocationLogLifecycleBackend(t, test.status)
			defer backend.Close()
			handler := NewAllocationLogsHandler(newJobsTestHandler(t, backend.URL))
			request := requestWithToken(http.MethodGet,
				"/ui/allocations/alloc-1/logs/stream?namespace=team-a&region=west&task=web&type=stdout&"+test.query)
			response := httptest.NewRecorder()

			handler.StreamLogs(response, request, "alloc-1")

			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "event: log") ||
				!strings.Contains(response.Body.String(), "event: "+test.wantEvent) {
				t.Fatalf("log lifecycle response status=%d body=%q, want %q event", response.Code, response.Body.String(), test.wantEvent)
			}
		})
	}
}

func newAllocationLogLifecycleBackend(t *testing.T, finalStatus string) *httptest.Server {
	t.Helper()
	var allocationReads atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertScopedBrowserRequest(t, r)
		switch r.URL.Path {
		case "/v1/allocation/alloc-1":
			status := api.AllocClientStatusRunning
			if allocationReads.Add(1) > 1 {
				status = finalStatus
			}
			writeJSON(t, w, map[string]any{"ID": "alloc-1", "ClientStatus": status})
		case "/v1/client/fs/logs/alloc-1":
			if r.URL.Query().Get("task") != "web" || r.URL.Query().Get("type") != api.FSLogNameStdout {
				t.Fatalf("unexpected log selection: %s", r.URL.RawQuery)
			}
			_, _ = fmt.Fprintln(w, `{"Data":"bGluZQo="}`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestAllocationQueryOptionsPreserveBrowserScope(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ui/allocations/alloc/logs?namespace=team-a&region=west", nil)
	options := allocationQueryOptions(request, "browser-token")
	if options.Namespace != "team-a" || options.Region != "west" || options.AuthToken != "browser-token" {
		t.Fatalf("log query options lost browser scope: %#v", options)
	}
}

func TestForwardAllocationLogStream(t *testing.T) {
	t.Run("forwards frames and finishes when the frame stream closes", func(t *testing.T) {
		frames := make(chan *api.StreamFrame, 1)
		errs := make(chan error)
		events := make(chan allocationLogEvent, 1)
		frames <- &api.StreamFrame{Data: []byte("hello")}
		close(frames)

		forwardAllocationLogStream(context.Background(), api.FSLogNameStdout, frames, errs, events)

		event := <-events
		if got := string(event.frame.Data); got != "hello" {
			t.Fatalf("expected forwarded frame %q, got %q", "hello", got)
		}
	})

	t.Run("forwards stream errors", func(t *testing.T) {
		frames := make(chan *api.StreamFrame)
		errs := make(chan error, 1)
		events := make(chan allocationLogEvent, 1)
		want := errors.New("stream failed")
		errs <- want

		forwardAllocationLogStream(context.Background(), api.FSLogNameStderr, frames, errs, events)

		event := <-events
		if !errors.Is(event.err, want) {
			t.Fatalf("expected forwarded error %v, got %v", want, event.err)
		}
		if event.logType != api.FSLogNameStderr {
			t.Fatalf("expected stderr event, got %q", event.logType)
		}
	})
}

func TestLogLineFromFrame(t *testing.T) {
	handler := &AllocationLogsHandler{}

	stderr := handler.logLineFromFrame(&api.StreamFrame{Data: []byte("plain stderr")}, api.FSLogNameStderr)
	if stderr.Level != "error" {
		t.Fatalf("expected stderr to be marked as an error, got %q", stderr.Level)
	}

	binaryData := []byte{'a', 0, 'b'}
	binary := handler.logLineFromFrame(&api.StreamFrame{Data: binaryData}, api.FSLogNameStdout)
	if want := base64.StdEncoding.EncodeToString(binaryData); binary.Message != want {
		t.Fatalf("expected base64 message %q, got %q", want, binary.Message)
	}
}

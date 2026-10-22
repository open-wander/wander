// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
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

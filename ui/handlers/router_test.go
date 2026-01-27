package handlers

import (
	"testing"

	"github.com/hashicorp/go-hclog"
)

// TestNewHandler verifies the handler constructor works correctly
func TestNewHandler(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.logger == nil {
		t.Error("Handler logger is nil")
	}
}

// TestNewHandlerWithNilLogger verifies nil logger handling
func TestNewHandlerWithNilLogger(t *testing.T) {
	h := NewHandler(nil)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}

	if h.logger == nil {
		t.Error("Handler should create null logger when passed nil")
	}
}

// TestNewRouter verifies the router constructor works correctly
func TestNewRouter(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	r := NewRouter(h)

	if r == nil {
		t.Fatal("NewRouter returned nil")
	}

	if r.mux == nil {
		t.Error("Router mux is nil")
	}

	if r.handler == nil {
		t.Error("Router handler is nil")
	}
}

// TestNewStaticHandler verifies the static handler constructor works correctly
func TestNewStaticHandler(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	s := NewStaticHandler(h)

	if s == nil {
		t.Fatal("NewStaticHandler returned nil")
	}

	if s.fileSystem == nil {
		t.Error("StaticHandler fileSystem is nil")
	}
}

// TestNewHomeHandler verifies the home handler constructor works correctly
func TestNewHomeHandler(t *testing.T) {
	logger := hclog.NewNullLogger()
	h := NewHandler(logger)
	home := NewHomeHandler(h)

	if home == nil {
		t.Fatal("NewHomeHandler returned nil")
	}

	if home.handler == nil {
		t.Error("HomeHandler handler is nil")
	}
}

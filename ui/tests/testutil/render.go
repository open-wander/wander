package testutil

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
)

// RenderToString renders a templ.Component to a string.
// Returns the rendered HTML string and any error encountered.
//
// Example:
//
//	html, err := RenderToString(components.Button(components.ButtonProps{Variant: "primary"}, "Click me"))
//	if err != nil {
//	    t.Fatal(err)
//	}
func RenderToString(component templ.Component) (string, error) {
	buf := new(bytes.Buffer)
	err := component.Render(context.Background(), buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderToBytes renders a templ.Component to a byte slice.
// Returns the rendered HTML as bytes and any error encountered.
//
// Example:
//
//	html, err := RenderToBytes(components.Badge(components.BadgeProps{Variant: "success"}, "Active"))
//	if err != nil {
//	    t.Fatal(err)
//	}
func RenderToBytes(component templ.Component) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := component.Render(context.Background(), buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MustRender renders a templ.Component to a string and fails the test if an error occurs.
// This is a convenience wrapper around RenderToString for test cases where errors are not expected.
//
// Example:
//
//	html := MustRender(t, components.Alert(components.AlertProps{Variant: "info"}, "Information"))
//	// Test assertions on html...
func MustRender(t *testing.T, component templ.Component) string {
	t.Helper()
	html, err := RenderToString(component)
	if err != nil {
		t.Fatalf("failed to render component: %v", err)
	}
	return html
}

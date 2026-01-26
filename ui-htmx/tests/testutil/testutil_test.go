package testutil

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// Mock component for testing
func mockButton(variant, label string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`<button type="button" class="btn btn-` + variant + `">` + label + `</button>`))
		return err
	})
}

func mockBadge(variant, text string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`<span class="badge bg-` + variant + `-100 text-` + variant + `-800">` + text + `</span>`))
		return err
	})
}

func mockInput(name, inputType string, disabled bool) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		disabledAttr := ""
		if disabled {
			disabledAttr = ` disabled`
		}
		_, err := w.Write([]byte(`<input type="` + inputType + `" name="` + name + `"` + disabledAttr + ` />`))
		return err
	})
}

func mockDiv(id, class string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		_, err := w.Write([]byte(`<div id="` + id + `" class="` + class + `">Content</div>`))
		return err
	})
}

// Test RenderToString
func TestRenderToString(t *testing.T) {
	tests := []struct {
		name      string
		component templ.Component
		want      string
	}{
		{
			name:      "simple button",
			component: mockButton("primary", "Click me"),
			want:      `<button type="button" class="btn btn-primary">Click me</button>`,
		},
		{
			name:      "badge component",
			component: mockBadge("success", "Active"),
			want:      `<span class="badge bg-success-100 text-success-800">Active</span>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderToString(tt.component)
			if err != nil {
				t.Fatalf("RenderToString() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("RenderToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Test RenderToBytes
func TestRenderToBytes(t *testing.T) {
	component := mockButton("danger", "Delete")
	want := []byte(`<button type="button" class="btn btn-danger">Delete</button>`)

	got, err := RenderToBytes(component)
	if err != nil {
		t.Fatalf("RenderToBytes() error = %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("RenderToBytes() = %q, want %q", got, want)
	}
}

// Test MustRender
func TestMustRender(t *testing.T) {
	component := mockButton("secondary", "Cancel")
	want := `<button type="button" class="btn btn-secondary">Cancel</button>`

	got := MustRender(t, component)
	if got != want {
		t.Errorf("MustRender() = %q, want %q", got, want)
	}
}

// Test ContainsHTML
func TestContainsHTML(t *testing.T) {
	html := `<button type="button" class="btn btn-primary">Click me</button>`

	tests := []struct {
		name     string
		html     string
		expected string
		wantFail bool
	}{
		{
			name:     "contains button tag",
			html:     html,
			expected: "<button",
			wantFail: false,
		},
		{
			name:     "contains class",
			html:     html,
			expected: "btn-primary",
			wantFail: false,
		},
		{
			name:     "contains text",
			html:     html,
			expected: "Click me",
			wantFail: false,
		},
		{
			name:     "does not contain",
			html:     html,
			expected: "btn-danger",
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a sub-test to catch failures
			mockT := &testing.T{}
			ContainsHTML(mockT, tt.html, tt.expected)

			failed := mockT.Failed()
			if failed != tt.wantFail {
				if tt.wantFail {
					t.Errorf("ContainsHTML() should have failed but didn't")
				} else {
					t.Errorf("ContainsHTML() should not have failed but did")
				}
			}
		})
	}
}

// Test NotContainsHTML
func TestNotContainsHTML(t *testing.T) {
	html := `<button type="button" class="btn btn-primary">Click me</button>`

	tests := []struct {
		name       string
		html       string
		unexpected string
		wantFail   bool
	}{
		{
			name:       "does not contain danger",
			html:       html,
			unexpected: "btn-danger",
			wantFail:   false,
		},
		{
			name:       "does not contain input",
			html:       html,
			unexpected: "<input",
			wantFail:   false,
		},
		{
			name:       "contains primary (should fail)",
			html:       html,
			unexpected: "btn-primary",
			wantFail:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &testing.T{}
			NotContainsHTML(mockT, tt.html, tt.unexpected)

			failed := mockT.Failed()
			if failed != tt.wantFail {
				if tt.wantFail {
					t.Errorf("NotContainsHTML() should have failed but didn't")
				} else {
					t.Errorf("NotContainsHTML() should not have failed but did")
				}
			}
		})
	}
}

// Test ContainsClass
func TestContainsClass(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		class    string
		wantFail bool
	}{
		{
			name:     "single class",
			html:     `<div class="container">Content</div>`,
			class:    "container",
			wantFail: false,
		},
		{
			name:     "multiple classes",
			html:     `<button class="btn btn-primary btn-lg">Click</button>`,
			class:    "btn-primary",
			wantFail: false,
		},
		{
			name:     "class with hyphen",
			html:     `<span class="bg-success-100">Active</span>`,
			class:    "bg-success-100",
			wantFail: false,
		},
		{
			name:     "partial match should fail",
			html:     `<span class="bg-success-100">Active</span>`,
			class:    "success",
			wantFail: true,
		},
		{
			name:     "class not present",
			html:     `<div class="container">Content</div>`,
			class:    "wrapper",
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &testing.T{}
			ContainsClass(mockT, tt.html, tt.class)

			failed := mockT.Failed()
			if failed != tt.wantFail {
				if tt.wantFail {
					t.Errorf("ContainsClass() should have failed but didn't")
				} else {
					t.Errorf("ContainsClass() should not have failed but did")
				}
			}
		})
	}
}

// Test ContainsElement
func TestContainsElement(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		tag      string
		wantFail bool
	}{
		{
			name:     "button element exists",
			html:     `<button type="button">Click</button>`,
			tag:      "button",
			wantFail: false,
		},
		{
			name:     "div element exists",
			html:     `<div id="main" class="container">Content</div>`,
			tag:      "div",
			wantFail: false,
		},
		{
			name:     "span element exists",
			html:     `<span>Text</span>`,
			tag:      "span",
			wantFail: false,
		},
		{
			name:     "input element exists (self-closing)",
			html:     `<input type="text" />`,
			tag:      "input",
			wantFail: false,
		},
		{
			name:     "element does not exist",
			html:     `<button>Click</button>`,
			tag:      "input",
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &testing.T{}
			ContainsElement(mockT, tt.html, tt.tag)

			failed := mockT.Failed()
			if failed != tt.wantFail {
				if tt.wantFail {
					t.Errorf("ContainsElement() should have failed but didn't")
				} else {
					t.Errorf("ContainsElement() should not have failed but did")
				}
			}
		})
	}
}

// Test HasAttribute
func TestHasAttribute(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		attr     string
		value    string
		wantFail bool
	}{
		{
			name:     "type attribute with value",
			html:     `<button type="submit">Submit</button>`,
			attr:     "type",
			value:    "submit",
			wantFail: false,
		},
		{
			name:     "disabled attribute (boolean)",
			html:     `<input type="text" disabled />`,
			attr:     "disabled",
			value:    "",
			wantFail: false,
		},
		{
			name:     "name attribute",
			html:     `<input type="text" name="username" />`,
			attr:     "name",
			value:    "username",
			wantFail: false,
		},
		{
			name:     "hx-get attribute",
			html:     `<button hx-get="/api/data">Load</button>`,
			attr:     "hx-get",
			value:    "/api/data",
			wantFail: false,
		},
		{
			name:     "attribute wrong value",
			html:     `<button type="button">Click</button>`,
			attr:     "type",
			value:    "submit",
			wantFail: true,
		},
		{
			name:     "attribute does not exist",
			html:     `<button type="button">Click</button>`,
			attr:     "disabled",
			value:    "",
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockT := &testing.T{}
			HasAttribute(mockT, tt.html, tt.attr, tt.value)

			failed := mockT.Failed()
			if failed != tt.wantFail {
				if tt.wantFail {
					t.Errorf("HasAttribute() should have failed but didn't")
				} else {
					t.Errorf("HasAttribute() should not have failed but did")
				}
			}
		})
	}
}

// Test integration: rendering and asserting on real-ish components
func TestIntegration_ButtonComponent(t *testing.T) {
	component := mockButton("primary", "Click me")

	html := MustRender(t, component)

	// Assert element exists
	ContainsElement(t, html, "button")

	// Assert classes
	ContainsClass(t, html, "btn")
	ContainsClass(t, html, "btn-primary")

	// Assert attributes
	HasAttribute(t, html, "type", "button")

	// Assert text content
	ContainsHTML(t, html, "Click me")

	// Assert absence
	NotContainsHTML(t, html, "btn-danger")
}

func TestIntegration_BadgeComponent(t *testing.T) {
	component := mockBadge("warning", "Pending")

	html := MustRender(t, component)

	// Assert structure
	ContainsElement(t, html, "span")
	ContainsClass(t, html, "badge")
	ContainsClass(t, html, "bg-warning-100")
	ContainsClass(t, html, "text-warning-800")
	ContainsHTML(t, html, "Pending")
}

func TestIntegration_InputComponent(t *testing.T) {
	component := mockInput("email", "email", true)

	html := MustRender(t, component)

	// Assert structure
	ContainsElement(t, html, "input")
	HasAttribute(t, html, "type", "email")
	HasAttribute(t, html, "name", "email")
	HasAttribute(t, html, "disabled", "")
}

// Test edge cases
func TestEdgeCases(t *testing.T) {
	t.Run("empty HTML", func(t *testing.T) {
		mockT := &testing.T{}
		ContainsHTML(mockT, "", "test")
		if !mockT.Failed() {
			t.Error("should fail on empty HTML")
		}
	})

	t.Run("empty expected substring passes", func(t *testing.T) {
		mockT := &testing.T{}
		ContainsHTML(mockT, "<div>test</div>", "")
		if mockT.Failed() {
			t.Error("empty substring should always match")
		}
	})

	t.Run("class with special regex characters", func(t *testing.T) {
		html := `<div class="w-[100px]">Content</div>`
		ContainsClass(t, html, "w-[100px]")
	})

	t.Run("attribute with special characters", func(t *testing.T) {
		html := `<div data-test="value[0]">Content</div>`
		HasAttribute(t, html, "data-test", "value[0]")
	})
}

// Benchmark tests
func BenchmarkRenderToString(b *testing.B) {
	component := mockButton("primary", "Click")
	for i := 0; i < b.N; i++ {
		_, _ = RenderToString(component)
	}
}

func BenchmarkMustRender(b *testing.B) {
	component := mockButton("primary", "Click")
	t := &testing.T{}
	for i := 0; i < b.N; i++ {
		_ = MustRender(t, component)
	}
}

func BenchmarkContainsHTML(b *testing.B) {
	html := strings.Repeat(`<button class="btn btn-primary">Click</button>`, 100)
	t := &testing.T{}
	for i := 0; i < b.N; i++ {
		ContainsHTML(t, html, "btn-primary")
	}
}

func BenchmarkContainsClass(b *testing.B) {
	html := strings.Repeat(`<button class="btn btn-primary btn-lg">Click</button>`, 100)
	t := &testing.T{}
	for i := 0; i < b.N; i++ {
		ContainsClass(t, html, "btn-primary")
	}
}

package components

import (
	"context"
	"strings"
	"testing"
)

func TestInputType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Valid text", "text", "text"},
		{"Valid number", "number", "number"},
		{"Valid password", "password", "password"},
		{"Valid search", "search", "search"},
		{"Valid email", "email", "email"},
		{"Valid url", "url", "url"},
		{"Invalid type", "invalid", "text"},
		{"Empty type", "", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inputType(tt.input)
			if result != tt.expected {
				t.Errorf("inputType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildInputClasses(t *testing.T) {
	tests := []struct {
		name     string
		props    InputProps
		hasIcon  bool
		expected []string // classes that must be present
	}{
		{
			name:     "Default input",
			props:    InputProps{},
			hasIcon:  false,
			expected: []string{"input"},
		},
		{
			name:     "Small input",
			props:    InputProps{Size: "sm"},
			hasIcon:  false,
			expected: []string{"input", "input-sm"},
		},
		{
			name:     "Large input",
			props:    InputProps{Size: "lg"},
			hasIcon:  false,
			expected: []string{"input", "input-lg"},
		},
		{
			name:     "Input with error",
			props:    InputProps{Error: "Invalid input"},
			hasIcon:  false,
			expected: []string{"input", "input-error"},
		},
		{
			name:     "Search input with icon (md)",
			props:    InputProps{Type: "search"},
			hasIcon:  true,
			expected: []string{"input", "pl-10"},
		},
		{
			name:     "Search input with icon (sm)",
			props:    InputProps{Type: "search", Size: "sm"},
			hasIcon:  true,
			expected: []string{"input", "input-sm", "pl-8"},
		},
		{
			name:     "Search input with icon (lg)",
			props:    InputProps{Type: "search", Size: "lg"},
			hasIcon:  true,
			expected: []string{"input", "input-lg", "pl-12"},
		},
		{
			name:     "Input with custom class",
			props:    InputProps{Class: "custom-class"},
			hasIcon:  false,
			expected: []string{"input", "custom-class"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildInputClasses(tt.props, tt.hasIcon)
			for _, class := range tt.expected {
				if !strings.Contains(result, class) {
					t.Errorf("buildInputClasses() = %q, missing class %q", result, class)
				}
			}
		})
	}
}

func TestSearchIconSize(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		expected string
	}{
		{"Small icon", "sm", "h-4 w-4 text-neutral-400"},
		{"Medium icon (default)", "md", "h-5 w-5 text-neutral-400"},
		{"Large icon", "lg", "h-6 w-6 text-neutral-400"},
		{"Empty size (default)", "", "h-5 w-5 text-neutral-400"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := searchIconSize(tt.size)
			if result != tt.expected {
				t.Errorf("searchIconSize(%q) = %q, want %q", tt.size, result, tt.expected)
			}
		})
	}
}

func TestInputRendering(t *testing.T) {
	tests := []struct {
		name     string
		props    InputProps
		contains []string
	}{
		{
			name: "Basic text input",
			props: InputProps{
				Type:        "text",
				Name:        "username",
				ID:          "username",
				Placeholder: "Enter username",
			},
			contains: []string{
				`type="text"`,
				`name="username"`,
				`id="username"`,
				`placeholder="Enter username"`,
			},
		},
		{
			name: "Search input with icon",
			props: InputProps{
				Type:        "search",
				Name:        "query",
				Placeholder: "Search...",
			},
			contains: []string{
				`type="search"`,
				`name="query"`,
				`placeholder="Search..."`,
				"<svg", // search icon
			},
		},
		{
			name: "Input with error",
			props: InputProps{
				Type:  "email",
				Error: "Invalid email",
			},
			contains: []string{
				`type="email"`,
				`class="form-error"`,
				"Invalid email",
			},
		},
		{
			name: "Input with HTMX attributes",
			props: InputProps{
				Type:      "text",
				HxGet:     "/api/search",
				HxTarget:  "#results",
				HxTrigger: "keyup changed delay:500ms",
			},
			contains: []string{
				`hx-get="/api/search"`,
				`hx-target="#results"`,
				`hx-trigger="keyup changed delay:500ms"`,
			},
		},
		{
			name: "Required disabled readonly input",
			props: InputProps{
				Type:     "text",
				Required: true,
				Disabled: true,
				ReadOnly: true,
			},
			contains: []string{
				"required",
				"disabled",
				"readonly",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render the component to HTML
			var buf strings.Builder
			err := Input(tt.props).Render(context.Background(), &buf)
			if err != nil {
				t.Fatalf("Failed to render component: %v", err)
			}

			html := buf.String()

			// Check that all expected strings are present
			for _, expected := range tt.contains {
				if !strings.Contains(html, expected) {
					t.Errorf("Rendered HTML missing expected string %q.\nGot:\n%s", expected, html)
				}
			}
		})
	}
}

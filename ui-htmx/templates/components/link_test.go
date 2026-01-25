package components

import (
	"strings"
	"testing"
)

func TestIsExternalURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "empty string",
			url:      "",
			expected: false,
		},
		{
			name:     "internal absolute path",
			url:      "/dashboard",
			expected: false,
		},
		{
			name:     "internal relative path",
			url:      "settings",
			expected: false,
		},
		{
			name:     "fragment link",
			url:      "#section",
			expected: false,
		},
		{
			name:     "https external",
			url:      "https://example.com",
			expected: true,
		},
		{
			name:     "http external",
			url:      "http://example.com/page",
			expected: true,
		},
		{
			name:     "mailto link",
			url:      "mailto:test@example.com",
			expected: true,
		},
		{
			name:     "ftp protocol",
			url:      "ftp://files.example.com",
			expected: true,
		},
		{
			name:     "relative path with extension",
			url:      "./docs/guide.html",
			expected: false,
		},
		{
			name:     "parent directory",
			url:      "../other",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExternalURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsExternalURL(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestLinkClasses(t *testing.T) {
	tests := []struct {
		name     string
		props    LinkProps
		contains []string // strings that should be in the result
	}{
		{
			name: "default variant",
			props: LinkProps{
				Href:    "/test",
				Variant: "default",
			},
			contains: []string{
				"inline-flex",
				"text-blue-600",
				"hover:text-blue-800",
			},
		},
		{
			name: "muted variant",
			props: LinkProps{
				Href:    "/test",
				Variant: "muted",
			},
			contains: []string{
				"inline-flex",
				"text-neutral-600",
				"hover:text-neutral-900",
			},
		},
		{
			name: "primary variant",
			props: LinkProps{
				Href:    "/test",
				Variant: "primary",
			},
			contains: []string{
				"inline-flex",
				"text-primary-600",
				"hover:text-primary-700",
				"font-medium",
			},
		},
		{
			name: "with custom classes",
			props: LinkProps{
				Href:  "/test",
				Class: "custom-class another-class",
			},
			contains: []string{
				"inline-flex",
				"custom-class",
				"another-class",
			},
		},
		{
			name: "empty variant defaults to default",
			props: LinkProps{
				Href:    "/test",
				Variant: "",
			},
			contains: []string{
				"text-blue-600",
				"hover:text-blue-800",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := linkClasses(tt.props)
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("linkClasses() result missing %q\nGot: %s", substr, result)
				}
			}
		})
	}
}

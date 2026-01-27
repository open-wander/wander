package testutil

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// ContainsHTML asserts that the HTML string contains the expected substring.
// Fails the test if the substring is not found.
//
// Example:
//
//	html := MustRender(t, components.Button(components.ButtonProps{Variant: "primary"}, "Click"))
//	ContainsHTML(t, html, "btn-primary")
func ContainsHTML(t *testing.T, html, expected string) {
	t.Helper()
	if !strings.Contains(html, expected) {
		t.Errorf("HTML does not contain expected substring.\nExpected substring: %q\nHTML: %s", expected, html)
	}
}

// NotContainsHTML asserts that the HTML string does not contain the unexpected substring.
// Fails the test if the substring is found.
//
// Example:
//
//	html := MustRender(t, components.Button(components.ButtonProps{Variant: "primary"}, "Click"))
//	NotContainsHTML(t, html, "btn-danger")
func NotContainsHTML(t *testing.T, html, unexpected string) {
	t.Helper()
	if strings.Contains(html, unexpected) {
		t.Errorf("HTML contains unexpected substring.\nUnexpected substring: %q\nHTML: %s", unexpected, html)
	}
}

// ContainsClass asserts that the HTML contains an element with the specified CSS class.
// This is more robust than a simple substring search as it looks for the class attribute pattern.
//
// Example:
//
//	html := MustRender(t, components.Badge(components.BadgeProps{Variant: "success"}, "Active"))
//	ContainsClass(t, html, "bg-success-100")
func ContainsClass(t *testing.T, html, class string) {
	t.Helper()

	// Extract all class attributes from the HTML
	// Handles both double and single quotes: class="foo bar" or class='foo bar'
	classAttrRegex := regexp.MustCompile(`class=["']([^"']+)["']`)
	matches := classAttrRegex.FindAllStringSubmatch(html, -1)

	// Check each class attribute
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		// Split the class attribute value by whitespace
		classes := strings.Fields(match[1])

		// Check if our target class is in the list
		for _, c := range classes {
			if c == class {
				return
			}
		}
	}

	t.Errorf("HTML does not contain class %q.\nHTML: %s", class, html)
}

// ContainsElement asserts that the HTML contains at least one element with the specified tag name.
//
// Example:
//
//	html := MustRender(t, components.Button(components.ButtonProps{}, "Click"))
//	ContainsElement(t, html, "button")
func ContainsElement(t *testing.T, html, tag string) {
	t.Helper()

	// Look for opening tag (handles both <tag> and <tag attributes>)
	pattern := fmt.Sprintf(`<%s[\s>]`, regexp.QuoteMeta(tag))
	matched, err := regexp.MatchString(pattern, html)
	if err != nil {
		t.Fatalf("invalid regex pattern: %v", err)
	}

	if !matched {
		t.Errorf("HTML does not contain <%s> element.\nHTML: %s", tag, html)
	}
}

// HasAttribute asserts that the HTML contains an element with the specified attribute and value.
// The value can be empty string to just check for attribute presence.
//
// Example:
//
//	html := MustRender(t, components.Button(components.ButtonProps{Type: "submit"}, "Submit"))
//	HasAttribute(t, html, "type", "submit")
//
//	html := MustRender(t, components.Button(components.ButtonProps{Disabled: true}, "Disabled"))
//	HasAttribute(t, html, "disabled", "") // checks for presence only
func HasAttribute(t *testing.T, html, attr, value string) {
	t.Helper()

	var pattern string
	if value == "" {
		// Check for attribute presence (handles boolean attributes like "disabled")
		// Matches: disabled, disabled="", disabled='', disabled="disabled"
		pattern = fmt.Sprintf(`\b%s(?:\s*=\s*(?:"[^"]*"|'[^']*'|\S+))?[\s>]`, regexp.QuoteMeta(attr))
	} else {
		// Check for attribute with specific value
		// Handles: attr="value", attr='value'
		patterns := []string{
			fmt.Sprintf(`%s="%s"`, regexp.QuoteMeta(attr), regexp.QuoteMeta(value)),
			fmt.Sprintf(`%s='%s'`, regexp.QuoteMeta(attr), regexp.QuoteMeta(value)),
		}

		for _, p := range patterns {
			matched, err := regexp.MatchString(p, html)
			if err != nil {
				t.Fatalf("invalid regex pattern: %v", err)
			}
			if matched {
				return
			}
		}

		t.Errorf("HTML does not contain attribute %s=%q.\nHTML: %s", attr, value, html)
		return
	}

	matched, err := regexp.MatchString(pattern, html)
	if err != nil {
		t.Fatalf("invalid regex pattern: %v", err)
	}

	if !matched {
		if value == "" {
			t.Errorf("HTML does not contain attribute %q.\nHTML: %s", attr, html)
		} else {
			t.Errorf("HTML does not contain attribute %s=%q.\nHTML: %s", attr, value, html)
		}
	}
}

// ContainsHTMLf is like ContainsHTML but with formatted message support.
func ContainsHTMLf(t *testing.T, html, expected, msgFormat string, args ...interface{}) {
	t.Helper()
	if !strings.Contains(html, expected) {
		msg := fmt.Sprintf(msgFormat, args...)
		t.Errorf("HTML does not contain expected substring. %s\nExpected substring: %q\nHTML: %s", msg, expected, html)
	}
}

// AssertRendersWithoutError is a helper to ensure a component renders without error.
// This is useful for smoke testing components.
//
// Example:
//
//	AssertRendersWithoutError(t, components.Modal(components.ModalProps{}, "Content"))
func AssertRendersWithoutError(t *testing.T, component interface{}) {
	t.Helper()

	// Check if it's a templ.Component
	templComponent, ok := component.(interface {
		Render(ctx interface{}, w interface{}) error
	})

	if !ok {
		t.Fatal("provided component does not implement templ.Component interface")
	}

	buf := new(strings.Builder)
	err := templComponent.Render(nil, buf)
	if err != nil {
		t.Fatalf("component failed to render: %v", err)
	}
}

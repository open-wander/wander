# Templ Test Utilities

Comprehensive testing utilities for Templ components in the UI modernization project.

## Overview

This package provides a set of helper functions and assertions designed specifically for testing Templ template components. It simplifies rendering components and making assertions about the generated HTML.

## Installation

```go
import "github.com/open-wander/wander/ui/tests/testutil"
```

## Rendering Functions

### RenderToString

Renders a Templ component to a string.

```go
html, err := testutil.RenderToString(component)
if err != nil {
    t.Fatal(err)
}
```

### RenderToBytes

Renders a Templ component to a byte slice.

```go
html, err := testutil.RenderToBytes(component)
if err != nil {
    t.Fatal(err)
}
```

### MustRender

Renders a Templ component to a string and fails the test if an error occurs. This is the recommended approach for most tests.

```go
html := testutil.MustRender(t, component)
// Test assertions on html...
```

## Assertion Functions

### ContainsHTML

Asserts that the HTML contains a specific substring.

```go
html := testutil.MustRender(t, button)
testutil.ContainsHTML(t, html, "Click me")
```

### NotContainsHTML

Asserts that the HTML does NOT contain a specific substring.

```go
html := testutil.MustRender(t, button)
testutil.NotContainsHTML(t, html, "btn-danger")
```

### ContainsClass

Asserts that the HTML contains an element with a specific CSS class. This performs exact class matching and won't match partial class names.

```go
html := testutil.MustRender(t, badge)
testutil.ContainsClass(t, html, "bg-success-100")  // Pass
testutil.ContainsClass(t, html, "success")         // Fail - not an exact match
```

Handles Tailwind's arbitrary value syntax:
```go
testutil.ContainsClass(t, html, "w-[100px]")  // Works correctly with brackets
```

### ContainsElement

Asserts that the HTML contains at least one element with the specified tag name.

```go
html := testutil.MustRender(t, button)
testutil.ContainsElement(t, html, "button")
```

### HasAttribute

Asserts that the HTML contains an element with a specific attribute and value.

```go
// Check attribute with value
testutil.HasAttribute(t, html, "type", "submit")

// Check boolean attribute (disabled, readonly, etc.)
testutil.HasAttribute(t, html, "disabled", "")

// Check HTMX attributes
testutil.HasAttribute(t, html, "hx-post", "/api/submit")
```

### AssertRendersWithoutError

Smoke test to ensure a component renders without error.

```go
testutil.AssertRendersWithoutError(t, component)
```

## Usage Examples

### Basic Component Test

```go
func TestButton_Primary(t *testing.T) {
    // Arrange: Create component
    button := components.Button(
        components.ButtonProps{
            Variant: "primary",
            Size:    "md",
        },
        "Click me",
    )

    // Act: Render
    html := testutil.MustRender(t, button)

    // Assert: Verify output
    testutil.ContainsElement(t, html, "button")
    testutil.ContainsClass(t, html, "btn")
    testutil.ContainsClass(t, html, "btn-primary")
    testutil.HasAttribute(t, html, "type", "button")
    testutil.ContainsHTML(t, html, "Click me")
}
```

### Testing Multiple Variants

```go
func TestButton_Variants(t *testing.T) {
    tests := []struct {
        name    string
        variant string
        class   string
    }{
        {"primary", "primary", "btn-primary"},
        {"danger", "danger", "btn-danger"},
        {"secondary", "secondary", "btn-secondary"},
    }

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            button := components.Button(
                components.ButtonProps{Variant: tc.variant},
                "Test",
            )

            html := testutil.MustRender(t, button)
            testutil.ContainsClass(t, html, tc.class)
        })
    }
}
```

### Testing State

```go
func TestButton_Disabled(t *testing.T) {
    button := components.Button(
        components.ButtonProps{Disabled: true},
        "Disabled",
    )

    html := testutil.MustRender(t, button)
    testutil.HasAttribute(t, html, "disabled", "")
}
```

### Testing HTMX Attributes

```go
func TestButton_HTMX(t *testing.T) {
    button := components.Button(
        components.ButtonProps{
            HxPost:   "/api/submit",
            HxTarget: "#result",
            HxSwap:   "innerHTML",
        },
        "Submit",
    )

    html := testutil.MustRender(t, button)
    testutil.HasAttribute(t, html, "hx-post", "/api/submit")
    testutil.HasAttribute(t, html, "hx-target", "#result")
    testutil.HasAttribute(t, html, "hx-swap", "innerHTML")
}
```

### Negative Assertions

```go
func TestButton_NotContainsUnwantedClass(t *testing.T) {
    button := components.Button(
        components.ButtonProps{Variant: "primary"},
        "Test",
    )

    html := testutil.MustRender(t, button)
    testutil.NotContainsHTML(t, html, "btn-danger")
    testutil.NotContainsHTML(t, html, "undefined")
}
```

## Best Practices

1. **Use MustRender for cleaner tests** - It automatically fails the test if rendering errors occur.

2. **Test behavior, not implementation** - Focus on what the component renders (classes, attributes, content) rather than internal structure.

3. **Use ContainsClass for CSS classes** - It performs exact matching and handles edge cases better than substring search.

4. **Table-driven tests for variants** - When testing multiple variants or states, use table-driven tests for clarity.

5. **Test critical paths** - Focus on:
   - Required attributes (type, name, etc.)
   - CSS classes that affect styling
   - HTMX/Alpine.js attributes for interactivity
   - Conditional rendering (disabled states, icons, etc.)

6. **Avoid brittle tests** - Don't test exact HTML structure or whitespace. Use semantic assertions.

## Performance

The utilities are designed to be fast:

```
BenchmarkRenderToString-16    13569415    79.32 ns/op    240 B/op    4 allocs/op
BenchmarkMustRender-16         7346515   162.0 ns/op     240 B/op    4 allocs/op
BenchmarkContainsHTML-16      10498664   115.2 ns/op       0 B/op    0 allocs/op
BenchmarkContainsClass-16        29635  43074 ns/op    17155 B/op  235 allocs/op
```

## Running Tests

```bash
# Run all testutil tests
cd ui-htmx && go test ./tests/testutil/... -v

# Run with benchmarks
cd ui-htmx && go test ./tests/testutil/... -v -bench=. -benchmem

# Run specific test
cd ui-htmx && go test ./tests/testutil -run TestContainsClass -v
```

## Implementation Notes

- **Context**: All rendering uses `context.Background()` which is appropriate for unit tests.
- **Class matching**: Uses regex to extract class attributes and performs exact token matching.
- **Attribute matching**: Handles both value attributes (`type="submit"`) and boolean attributes (`disabled`).
- **Special characters**: Properly escapes regex metacharacters in class names and attribute values.
- **Tailwind support**: Correctly handles Tailwind's arbitrary value syntax like `w-[100px]`.

## Future Enhancements

Potential additions for the future:

- `ContainsJSON(t, html, key, value)` - Assert JSON attributes (Alpine.js x-data)
- `HasDataAttribute(t, html, attr, value)` - Specialized data-* attribute assertion
- `MatchesSelector(t, html, selector)` - CSS selector matching
- `CountElements(t, html, tag, expected)` - Count specific elements
- Integration with HTML parser for more sophisticated queries

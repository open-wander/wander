# Link Component

A smart link component for the Wander HTMX UI that automatically handles internal and external URLs with appropriate HTMX navigation and security attributes.

## Overview

The `Link` component provides a unified interface for creating links that:
- Uses HTMX for SPA-like navigation on internal links
- Opens external links in new tabs with proper security
- Shows visual indicators for external links
- Supports multiple styling variants
- Integrates seamlessly with HTMX routing

## Component Structure

### LinkProps

```go
type LinkProps struct {
    Href      string // The URL to link to
    External  bool   // If true, opens in new tab with security attributes
    Variant   string // Styling variant: "default", "muted", "primary"
    Class     string // Additional CSS classes to apply
    HxGet     string // HTMX hx-get attribute for dynamic loading
    HxTarget  string // HTMX hx-target attribute (e.g., "#content")
    HxPushUrl bool   // If true, updates browser URL with HTMX navigation
}
```

## Usage Examples

### 1. Simple Internal Link

```go
@Link(LinkProps{Href: "/dashboard"}) {
    Go to Dashboard
}
```

### 2. Internal Link with HTMX Navigation

```go
@Link(LinkProps{
    Href: "/jobs",
    HxGet: "/api/jobs",
    HxTarget: "#main-content",
    HxPushUrl: true,
    Variant: "default",
}) {
    View Jobs
}
```

### 3. External Link with New Tab

```go
@Link(LinkProps{
    Href: "https://docs.example.com",
    External: true,
    Variant: "primary",
}) {
    Read Documentation
}
```

This renders as: **Read Documentation** ↗

### 4. Muted Link for Secondary Actions

```go
@Link(LinkProps{
    Href: "/settings",
    Variant: "muted",
}) {
    Settings
}
```

### 5. Custom Styled Link

```go
@Link(LinkProps{
    Href: "/profile",
    Variant: "default",
    Class: "text-lg font-bold",
}) {
    My Profile
}
```

## Variants

### default
- Standard blue link styling
- Color: `text-blue-600` with `hover:text-blue-800`
- Best for general navigation links

### muted
- Subtle neutral styling
- Color: `text-neutral-600` with `hover:text-neutral-900`
- Best for secondary actions and less important links

### primary
- Bold primary color styling
- Color: `text-primary-600` with `hover:text-primary-700`
- Font weight: `font-medium`
- Best for call-to-action links

## Features

### Internal Links (default)

When `External` is `false` or not set:
- No `target="_blank"` attribute
- Can use HTMX attributes for SPA-like navigation
- No external link icon

Example:
```go
@Link(LinkProps{
    Href: "/dashboard",
    HxGet: "/api/dashboard/content",
    HxTarget: "#content",
    HxPushUrl: true,
}) {
    Dashboard
}
```

Renders as:
```html
<a href="/dashboard"
   hx-get="/api/dashboard/content"
   hx-target="#content"
   hx-push-url="true"
   class="inline-flex items-center gap-1 transition-colors duration-150 text-blue-600 hover:text-blue-800 underline-offset-2 hover:underline">
    Dashboard
</a>
```

### External Links

When `External` is `true`:
- Opens in new tab (`target="_blank"`)
- Includes security attributes (`rel="noopener noreferrer"`)
- Shows external link icon (arrow-top-right-on-square)
- No HTMX attributes applied (even if set)

Example:
```go
@Link(LinkProps{
    Href: "https://github.com/open-wander/wander",
    External: true,
    Variant: "primary",
}) {
    View on GitHub
}
```

Renders as:
```html
<a href="https://github.com/open-wander/wander"
   target="_blank"
   rel="noopener noreferrer"
   class="inline-flex items-center gap-1 transition-colors duration-150 text-primary-600 hover:text-primary-700 font-medium underline-offset-2 hover:underline">
    View on GitHub
    <svg class="inline-block h-4 w-4 flex-shrink-0" ...>
        <!-- External link icon -->
    </svg>
</a>
```

## Helper Functions

### IsExternalURL(url string) bool

Automatically detects if a URL is external based on patterns:

```go
// Returns false (internal)
IsExternalURL("/dashboard")           // true
IsExternalURL("settings")             // true
IsExternalURL("#section")             // true
IsExternalURL("./relative/path")      // true
IsExternalURL("../parent/path")       // true

// Returns true (external)
IsExternalURL("https://example.com")  // true
IsExternalURL("http://site.com")      // true
IsExternalURL("mailto:test@mail.com") // true
IsExternalURL("ftp://files.com")      // true
```

Usage:
```go
url := "https://example.com"
@Link(LinkProps{
    Href: url,
    External: IsExternalURL(url),
}) {
    Link Text
}
```

## Integration with Navigation

### Breadcrumb Links

```go
@Link(LinkProps{
    Href: "/jobs",
    HxGet: "/api/jobs",
    HxTarget: "#main-content",
    HxPushUrl: true,
    Variant: "muted",
    Class: "text-sm",
}) {
    Jobs
}
```

### Sidebar Menu Links

```go
@Link(LinkProps{
    Href: "/dashboard",
    HxGet: "/api/dashboard",
    HxTarget: "#main-content",
    HxPushUrl: true,
    Variant: "default",
    Class: "w-full justify-start",
}) {
    Dashboard
}
```

### Card Action Links

```go
@Link(LinkProps{
    Href: fmt.Sprintf("/jobs/%s", job.ID),
    HxGet: fmt.Sprintf("/api/jobs/%s", job.ID),
    HxTarget: "#main-content",
    HxPushUrl: true,
    Variant: "primary",
}) {
    View Details
}
```

## Accessibility

The Link component:
- Uses semantic `<a>` elements
- Supports keyboard navigation
- Provides clear focus indicators
- Includes `aria-hidden="true"` on decorative icons
- Uses proper `rel` attributes for security

## Best Practices

1. **Use appropriate variants**
   - `default` for most navigation
   - `muted` for secondary actions
   - `primary` for important CTAs

2. **Set External for external URLs**
   - Always set `External: true` for cross-domain links
   - Or use `IsExternalURL()` helper to auto-detect

3. **Use HTMX for internal navigation**
   - Set `HxGet`, `HxTarget`, and `HxPushUrl` for SPA behavior
   - Improves performance and user experience

4. **Add custom classes sparingly**
   - Use the `Class` prop for layout adjustments
   - Avoid overriding variant colors

5. **Provide descriptive link text**
   - Use meaningful text, not "click here"
   - Screen readers rely on link text for context

## Example: Navigation Menu

```go
templ NavigationMenu() {
    <nav class="space-y-2">
        @Link(LinkProps{
            Href: "/dashboard",
            HxGet: "/api/dashboard",
            HxTarget: "#main-content",
            HxPushUrl: true,
            Class: "block px-4 py-2",
        }) {
            Dashboard
        }

        @Link(LinkProps{
            Href: "/jobs",
            HxGet: "/api/jobs",
            HxTarget: "#main-content",
            HxPushUrl: true,
            Class: "block px-4 py-2",
        }) {
            Jobs
        }

        @Link(LinkProps{
            Href: "https://docs.wander.dev",
            External: true,
            Variant: "muted",
            Class: "block px-4 py-2",
        }) {
            Documentation
        }
    </nav>
}
```

## Security Notes

External links automatically include:
- `target="_blank"` - Opens in new tab
- `rel="noopener"` - Prevents new page from accessing `window.opener`
- `rel="noreferrer"` - Prevents sending referrer information

This protects against:
- Tabnabbing attacks
- Referrer leakage
- Performance issues from shared JavaScript context

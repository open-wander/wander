# Wander UI Templates

This directory contains [Templ](https://templ.guide/) templates for the Wander UI, migrated from Ember.js to a modern server-side rendering stack.

## Directory Structure

```
templates/
├── layouts/        # Base HTML layouts and page wrappers
├── components/     # Reusable UI components (buttons, forms, etc.)
├── pages/          # Full page templates
└── partials/       # HTMX partial response templates
```

## Base Layouts

### BaseLayout

The fundamental HTML structure with `<html>`, `<head>`, and `<body>` tags. All pages should use this as the root template.

**Features:**
- HTML5 doctype with `lang="en"`
- Responsive viewport meta tag
- Tailwind CSS integration (`/static/css/output.css`)
- HTMX v2.0.8 (`/static/js/htmx.min.js`)
- Alpine.js v3 with defer (`/static/js/alpine.min.js`)
- Alpine patterns (`/static/js/alpine-patterns.js`)
- HTMX configuration via meta tag
- Full height layout (`h-full` on html and body)

**Usage:**
```go
@layouts.BaseLayout(
    layouts.BaseLayoutProps{
        Title:       "My Page - Wander",
        Description: "Page description for SEO",
    },
    myContent(),
)
```

### PageLayout

Full page structure with header, sidebar, and main content area. Wraps `BaseLayout` and provides the application shell.

**Features:**
- Responsive sidebar (hidden on mobile, visible on lg+ screens)
- Header with breadcrumbs and mobile menu button
- Main content area with scroll overflow
- Breadcrumb navigation
- Placeholder areas for navigation and user menu components

**Usage:**
```go
@layouts.PageLayout(
    layouts.PageLayoutProps{
        Title:       "Jobs - Wander",
        Description: "View and manage Nomad jobs",
        ActiveNav:   "jobs",
        Breadcrumbs: []layouts.Breadcrumb{
            {Label: "Home", URL: "/"},
            {Label: "Jobs", URL: "/jobs"},
            {Label: "my-job", URL: ""},
        },
    },
    jobsContent(),
)
```

## Component Pattern

Components should accept a props struct for parameters and use `templ.Component` for nested content.

**Example:**
```go
package components

type ButtonProps struct {
    Variant string // "primary", "secondary", "danger"
    Type    string // "button", "submit", "reset"
    Class   string // Additional CSS classes
}

templ Button(props ButtonProps, label string) {
    <button
        type={ props.Type }
        class={ "btn", "btn-" + props.Variant, props.Class }
    >
        { label }
    </button>
}
```

## Building Templates

### Generate Go Code

Templates must be compiled to Go code before use:

```bash
# From ui-htmx directory
make ui-generate

# Or watch for changes
make ui-watch
```

This runs `templ generate` which creates `*_templ.go` files alongside each `.templ` file.

### Development Workflow

1. Edit `.templ` files
2. Run `make ui-generate` to compile
3. Go build will include the generated code
4. Restart the server to see changes

For continuous development, run in separate terminals:
```bash
make ui-watch     # Auto-regenerate templates
make watch-css    # Auto-rebuild Tailwind CSS
```

## HTMX Integration

### Full Page vs Partial Responses

Handlers should detect the `HX-Request` header and return either:
- **Full page**: Use `PageLayout` or `BaseLayout`
- **Partial**: Return just the content fragment

**Example:**
```go
func JobsHandler(w http.ResponseWriter, r *http.Request) {
    isHtmx := r.Header.Get("HX-Request") == "true"

    if isHtmx {
        // Return just the jobs table
        templates.JobsTable(jobs).Render(r.Context(), w)
    } else {
        // Return full page
        templates.JobsPage(jobs).Render(r.Context(), w)
    }
}
```

### HTMX Configuration

The base layout includes HTMX configuration via meta tag:
- `historyCacheSize: 20` - Cache last 20 history entries
- `defaultSwapStyle: innerHTML` - Default swap behavior
- `defaultSwapDelay: 0` - No artificial delay
- `timeout: 30000` - 30 second timeout for requests

## Alpine.js Integration

Alpine.js is available for client-side state management. Use it for:
- Dropdown menus
- Modals and dialogs
- Form validation
- Tabs and accordions
- Client-side filtering/sorting

**Example:**
```templ
<div x-data="{ open: false }">
    <button @click="open = !open">Toggle</button>
    <div x-show="open">Content</div>
</div>
```

Reusable Alpine patterns are available in `/static/js/alpine-patterns.js` via `window.alpinePatterns`.

## Accessibility

All templates should follow accessibility best practices:
- Use semantic HTML elements (`nav`, `main`, `header`, `aside`)
- Include ARIA labels where appropriate
- Ensure keyboard navigation works
- Use proper heading hierarchy
- Provide text alternatives for icons
- Maintain WCAG AA color contrast

## Tailwind CSS

Styling uses Tailwind CSS utility classes. Custom colors are defined in `tailwind.config.js`:
- `primary` - Blue shades for main actions
- `success` - Green for successful states
- `warning` - Amber for warning states
- `error` - Red for error states
- `info` - Cyan for informational states
- `neutral` - Gray shades for text and backgrounds

**Example:**
```templ
<div class="bg-white rounded-lg shadow p-6">
    <h2 class="text-2xl font-bold text-neutral-900 mb-4">
        Title
    </h2>
    <p class="text-neutral-600">
        Content
    </p>
</div>
```

## Resources

- [Templ Documentation](https://templ.guide/)
- [HTMX Documentation](https://htmx.org/)
- [Alpine.js Documentation](https://alpinejs.dev/)
- [Tailwind CSS Documentation](https://tailwindcss.com/)

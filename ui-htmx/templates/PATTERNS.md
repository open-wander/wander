# Template Patterns Discovered

This document captures patterns and best practices discovered during template development.

## Layout Composition Pattern

### Two-Layer Layout System

1. **BaseLayout** - Foundational HTML structure
   - Provides `<html>`, `<head>`, `<body>`
   - Includes all scripts and stylesheets
   - Accepts content via `templ.Component` parameter
   - Sets HTMX configuration

2. **PageLayout** - Application shell
   - Wraps BaseLayout
   - Provides header, sidebar, main content structure
   - Manages breadcrumbs and navigation state
   - Accepts content for the main area

### Component Composition

Templates can be composed by accepting `templ.Component` parameters:

```go
// Parent template accepts child content
templ ParentTemplate(props Props, content templ.Component) {
    <div class="wrapper">
        @content
    </div>
}

// Usage
@ParentTemplate(props, childContent())
```

## Props Pattern

Each template component should define a struct for its parameters:

```go
type ComponentProps struct {
    Title       string
    Description string
    ActiveNav   string
    Breadcrumbs []Breadcrumb
}

templ Component(props ComponentProps, content templ.Component) {
    // Template implementation
}
```

**Benefits:**
- Type safety
- Clear documentation of required fields
- Easy to extend without breaking existing code
- IDE autocomplete support

## Conditional Rendering

Use Go's `if` statements directly in templates:

```templ
if props.Description != "" {
    <meta name="description" content={ props.Description }/>
}

if len(props.Breadcrumbs) > 0 {
    <nav>
        <!-- Breadcrumb navigation -->
    </nav>
}
```

## Iteration Pattern

Use Go's `for` loops for rendering lists:

```templ
for i, crumb := range props.Breadcrumbs {
    <li class="flex items-center">
        if i > 0 {
            <!-- Separator -->
        }
        <!-- Breadcrumb item -->
    </li>
}
```

## URL Safety

Always use `templ.SafeURL()` for href attributes:

```templ
<a href={ templ.SafeURL(crumb.URL) }>
    { crumb.Label }
</a>
```

## Private Helper Templates

Use lowercase template names for internal/private components:

```go
// Public API
templ PageLayout(props PageLayoutProps, content templ.Component) {
    @BaseLayout(baseProps, pageContent(props, content))
}

// Private helper - not meant to be called directly
templ pageContent(props PageLayoutProps, content templ.Component) {
    // Implementation
}
```

## CSS Class Composition

Use Tailwind utility classes directly in templates:

```templ
<div class="flex h-full">
    <aside class="hidden lg:flex lg:flex-shrink-0">
        <!-- Responsive sidebar: hidden on mobile, visible on large screens -->
    </aside>
</div>
```

## Accessibility Attributes

Include ARIA labels and semantic HTML:

```templ
<aside
    id="sidebar"
    class="hidden lg:flex lg:flex-shrink-0"
    aria-label="Sidebar navigation"
>
    <!-- Sidebar content -->
</aside>

<main
    id="main-content"
    class="flex-1 overflow-y-auto bg-neutral-50"
    role="main"
    tabindex="-1"
>
    <!-- Main content -->
</main>
```

## Script Loading Order

In BaseLayout:
1. **Tailwind CSS** - First, for immediate styling
2. **HTMX** - Loaded synchronously (needed for page initialization)
3. **Alpine.js** - Deferred (waits for DOM ready)
4. **Alpine patterns** - After Alpine.js (no defer needed if after Alpine)

```templ
<head>
    <link rel="stylesheet" href="/static/css/output.css"/>
    <script src="/static/js/htmx.min.js"></script>
    <script src="/static/js/alpine.min.js" defer></script>
    <script src="/static/js/alpine-patterns.js"></script>
</head>
```

## HTMX Integration

Configure HTMX via meta tag in body:

```templ
<body class="h-full" hx-ext="head-support">
    <meta name="htmx-config" content='{
        "historyCacheSize":20,
        "defaultSwapStyle":"innerHTML",
        "defaultSwapDelay":0,
        "timeout":30000
    }'/>
    <!-- Body content -->
</body>
```

## Full Height Layout

For app-like layouts that fill the viewport:

```templ
<html lang="en" class="h-full">
    <body class="h-full">
        <div id="app" class="h-full">
            <div class="flex h-full">
                <!-- Flexbox layout fills height -->
            </div>
        </div>
    </body>
</html>
```

## Responsive Design Pattern

Use Tailwind's responsive prefixes for mobile-first design:

```templ
<!-- Hidden on mobile, visible on large screens -->
<aside class="hidden lg:flex lg:flex-shrink-0">
    <!-- Sidebar -->
</aside>

<!-- Visible on mobile, hidden on large screens -->
<button class="lg:hidden inline-flex">
    <!-- Mobile menu button -->
</button>
```

## Package Organization

- `layouts/` - Base HTML structures and page wrappers
- `pages/` - Complete page templates (import and use layouts)
- `components/` - Reusable UI components
- `partials/` - HTMX partial response fragments

Each directory is its own Go package.

## Import Pattern

Pages import layouts:

```go
package pages

import "github.com/open-wander/wander/ui-htmx/templates/layouts"

templ MyPage() {
    @layouts.PageLayout(props, content())
}
```

Components can be imported similarly:

```go
package pages

import "github.com/open-wander/wander/ui-htmx/templates/components"

templ MyPage() {
    @components.Button(buttonProps, "Click Me")
}
```

## Build Integration

Makefile targets for template workflow:

```makefile
ui-generate: ## Generate Go code from Templ templates
    @$(TEMPL_BIN) generate

ui-watch: ## Watch Templ templates and regenerate on changes
    @$(TEMPL_BIN) generate --watch
```

## Future Patterns to Consider

1. **Error Handling**: Create error page templates
2. **Loading States**: Skeleton loaders for HTMX requests
3. **Modal Pattern**: Reusable modal component with Alpine.js
4. **Form Validation**: Client-side validation patterns
5. **Toast Notifications**: Global notification system
6. **Dark Mode**: CSS variable switching with Alpine.js

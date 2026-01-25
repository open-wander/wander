# Tailwind CSS Guide for Wander UI

## Color Palette

The Tailwind configuration includes a professional color palette optimized for the Wander UI.

### Primary (Blue)
- **Use for**: Primary actions, links, active states
- **Main color**: `primary-500` (#3b82f6)
- **Classes**: `text-primary-600`, `bg-primary-500`, `border-primary-400`, etc.
- **Shades**: 50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950

### Success (Green)
- **Use for**: Success messages, running jobs, healthy states
- **Main color**: `success-500` (#22c55e)
- **Classes**: `text-success-600`, `bg-success-500`, `badge-success`
- **Example**: Running allocation status badges

### Warning (Amber)
- **Use for**: Warning messages, pending states, caution
- **Main color**: `warning-500` (#f59e0b)
- **Classes**: `text-warning-600`, `bg-warning-500`, `badge-warning`
- **Example**: Pending job status badges

### Error (Red)
- **Use for**: Error messages, failed states, destructive actions
- **Main color**: `error-500` (#ef4444)
- **Classes**: `text-error-600`, `bg-error-500`, `badge-error`
- **Example**: Failed allocation status badges

### Info (Cyan)
- **Use for**: Informational messages, neutral highlights
- **Main color**: `info-500` (#06b6d4)
- **Classes**: `text-info-600`, `bg-info-500`, `badge-info`

### Neutral (Gray)
- **Use for**: Text, backgrounds, borders, subtle UI elements
- **Main color**: `neutral-500` (#737373)
- **Classes**: `text-neutral-900`, `bg-neutral-50`, `border-neutral-200`
- **Shades**: 50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950

## Component Classes

### Buttons

```html
<!-- Primary button -->
<button class="btn btn-primary">Primary Action</button>

<!-- Secondary button -->
<button class="btn btn-secondary">Secondary Action</button>

<!-- Danger button -->
<button class="btn btn-danger">Delete</button>

<!-- Success button -->
<button class="btn btn-success">Approve</button>

<!-- Ghost button -->
<button class="btn btn-ghost">Cancel</button>

<!-- Sizes -->
<button class="btn btn-primary btn-sm">Small</button>
<button class="btn btn-primary btn-lg">Large</button>
```

### Badges

```html
<!-- Status badges -->
<span class="badge badge-primary">Primary</span>
<span class="badge badge-success">Running</span>
<span class="badge badge-warning">Pending</span>
<span class="badge badge-error">Failed</span>
<span class="badge badge-info">Info</span>
<span class="badge badge-neutral">Inactive</span>
```

### Cards

```html
<!-- Basic card -->
<div class="card p-6">
  <h2 class="text-xl font-semibold mb-4">Card Title</h2>
  <p>Card content...</p>
</div>
```

### Alerts

```html
<!-- Alert types -->
<div class="alert alert-info">Info message</div>
<div class="alert alert-success">Success message</div>
<div class="alert alert-warning">Warning message</div>
<div class="alert alert-error">Error message</div>
```

### Forms

```html
<!-- Text input -->
<input type="text" class="input" placeholder="Enter text...">

<!-- With label -->
<label class="block">
  <span class="text-sm font-medium text-neutral-700">Label</span>
  <input type="text" class="input mt-1">
</label>
```

### Tables

```html
<table class="table">
  <thead>
    <tr>
      <th>Name</th>
      <th>Status</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Job 1</td>
      <td><span class="badge badge-success">Running</span></td>
    </tr>
  </tbody>
</table>
```

## Utility Classes

### HTMX Loading States

```html
<!-- Loading indicator (hidden by default, shown during HTMX requests) -->
<div class="htmx-indicator">
  <div class="animate-spin h-5 w-5 border-2 border-primary-500"></div>
</div>

<!-- Skeleton loader for placeholder content -->
<div class="skeleton h-4 w-full mb-2"></div>
<div class="skeleton h-4 w-3/4"></div>
```

### Scrollbars

```html
<!-- Thin scrollbar styling -->
<div class="scrollbar-thin overflow-auto">
  <!-- scrollable content -->
</div>
```

## Dark Mode Support

Dark mode is configured using class-based strategy. To enable dark mode, add the `dark` class to the `<html>` or `<body>` element:

```html
<html class="dark">
  <!-- Dark mode enabled -->
</html>
```

Dark mode utilities automatically work:

```html
<!-- Light: white background, Dark: neutral-800 background -->
<div class="bg-white dark:bg-neutral-800">
  <!-- Light: black text, Dark: white text -->
  <p class="text-neutral-900 dark:text-neutral-100">Content</p>
</div>
```

## Custom Configuration

The `tailwind.config.js` includes:

- **Content paths**: Scans `.templ` and `.go` files in templates/handlers
- **Extended colors**: All semantic color palettes
- **Fonts**: Inter (sans), JetBrains Mono (mono)
- **Plugins**:
  - `@tailwindcss/forms` - Better form defaults
  - `@tailwindcss/typography` - Prose styling

## Build Process

### Development
```bash
make watch-css
```
Watches for changes and rebuilds CSS automatically.

### Production
```bash
make build-css
```
Builds minified CSS with unused classes purged.

## Best Practices

1. **Use semantic colors**: `primary`, `success`, `warning`, `error` instead of `blue`, `green`, etc.
2. **Prefer component classes**: Use `.btn`, `.card`, `.badge` for consistency
3. **Avoid custom CSS**: Use Tailwind utilities whenever possible
4. **Mobile-first**: Use responsive prefixes (`md:`, `lg:`) for larger screens
5. **Dark mode**: Always include dark mode variants for new components
6. **Accessibility**: Ensure sufficient color contrast (WCAG AA minimum)

## Example Component

```templ
package components

templ JobStatusBadge(status string) {
  @if status == "running" {
    <span class="badge badge-success">Running</span>
  } else if status == "pending" {
    <span class="badge badge-warning">Pending</span>
  } else if status == "failed" {
    <span class="badge badge-error">Failed</span>
  } else {
    <span class="badge badge-neutral">{ status }</span>
  }
}
```

## Resources

- [Tailwind CSS Docs](https://tailwindcss.com/docs)
- [Tailwind Color Reference](https://tailwindcss.com/docs/customizing-colors)
- [@tailwindcss/forms](https://github.com/tailwindlabs/tailwindcss-forms)
- [@tailwindcss/typography](https://github.com/tailwindlabs/tailwindcss-typography)

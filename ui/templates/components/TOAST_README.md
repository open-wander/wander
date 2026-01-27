# Toast Notification System

A complete toast notification system built with Alpine.js for the Wander HTMX/Templ UI.

## Features

- **Multiple Variants**: info, success, warning, error
- **Auto-dismiss**: Configurable duration with progress bar
- **Manual Dismiss**: X button to close manually
- **Stacked Toasts**: Multiple toasts stack nicely with proper spacing
- **Slide-in Animations**: Smooth enter/exit transitions
- **HTMX Integration**: Can be triggered from server responses
- **Positioned Containers**: Place in any corner of the viewport
- **Alpine.js Store**: Global store for managing toasts
- **JavaScript API**: Trigger from vanilla JavaScript

## Quick Start

### 1. Add ToastContainer to Base Layout

Add the toast container once in your base layout (e.g., `layouts/base.templ`):

```go
templ BaseLayout(props BaseLayoutProps, content templ.Component) {
    <!DOCTYPE html>
    <html lang="en">
        <head>
            // ... head content
        </head>
        <body>
            <div id="app">
                @content
            </div>

            <!-- Toast Container (add this) -->
            @components.ToastContainer(components.ToastContainerProps{
                Position: "top-right", // or "top-left", "bottom-right", "bottom-left"
            })
        </body>
    </html>
}
```

### 2. Trigger Toasts

#### From Alpine.js Components

```html
<button @click="$store.toasts.add({
    variant: 'success',
    title: 'Success',
    message: 'Operation completed successfully',
    duration: 3000
})">
    Show Toast
</button>
```

#### From Server (HTMX Response Header)

In your Go handler:

```go
// Set HX-Trigger header
w.Header().Set("HX-Trigger", `{
    "showToast": {
        "variant": "success",
        "title": "Saved",
        "message": "Your changes have been saved",
        "duration": 3000
    }
}`)
```

#### From JavaScript

```javascript
// From anywhere in your JavaScript
window.toastStore.add({
    variant: 'error',
    title: 'Error',
    message: 'Something went wrong',
    duration: 5000
});
```

## API Reference

### ToastProps

| Property | Type   | Description                                          | Default |
|----------|--------|------------------------------------------------------|---------|
| variant  | string | Toast type: "info", "success", "warning", "error"    | "info"  |
| title    | string | Toast title (optional)                               | ""      |
| message  | string | Toast message content (optional)                     | ""      |
| duration | int    | Auto-dismiss duration in milliseconds (0 = manual)   | 0       |
| position | string | Container position (used by ToastContainer only)     | ""      |

### ToastContainerProps

| Property | Type   | Description                                          | Default      |
|----------|--------|------------------------------------------------------|--------------|
| position | string | Corner position: "top-right", "top-left", etc.       | "top-right"  |

### Alpine.js Store Methods

**$store.toasts.add(toast)**
- Adds a new toast notification
- Returns toast ID
- Parameters: toast object with variant, title, message, duration

**$store.toasts.remove(id)**
- Removes a specific toast by ID
- Parameters: toast ID (number)

**$store.toasts.clear()**
- Removes all toasts immediately

### JavaScript API

**window.toastStore.add(toast)**
- Same as Alpine.js store method
- Available after Alpine initializes

## Variants

### Info (Blue)
- Informational messages
- Updates, tips, general notifications

### Success (Green)
- Successful operations
- Confirmations
- Completed actions

### Warning (Yellow)
- Warnings
- Cautions
- Non-critical issues

### Error (Red)
- Errors
- Failed operations
- Critical issues

## Duration

- **duration: 0** - Manual dismiss only (no auto-close)
- **duration: 3000** - Auto-dismiss after 3 seconds (typical)
- **duration: 5000** - Auto-dismiss after 5 seconds (errors)

Progress bar shows remaining time for auto-dismiss.

## Positions

- **top-right** - Default, top right corner
- **top-left** - Top left corner
- **bottom-right** - Bottom right corner
- **bottom-left** - Bottom left corner

## Examples

See `toast_example.templ` for comprehensive examples including:
- All variant types
- Multiple toast stacking
- HTMX integration
- JavaScript API usage
- Static server-rendered toasts

## Implementation Details

### Files
- `toast.templ` - Main component with ToastContainer and Toast
- `toast_example.templ` - Comprehensive usage examples
- `alpine-patterns.js` - Alpine.js store implementation

### Alpine.js Store
The toast store is registered globally in `alpine-patterns.js`:
- Manages toast lifecycle
- Auto-dismiss timers
- Progress bar updates
- Animation coordination

### HTMX Integration
The system listens for `HX-Trigger` headers with `showToast` events:
```
HX-Trigger: {"showToast": {...}}
```

Events are dispatched to the ToastContainer which adds them to the store.

## Accessibility

- Proper ARIA attributes (`role="alert"`, `aria-live="polite"`)
- Screen reader announcements
- Keyboard accessible (X button can be focused/clicked)
- Color is not the only indicator (icons + text)

## Browser Support

Works with modern browsers that support:
- Alpine.js 3.x
- CSS transitions
- ES6 JavaScript

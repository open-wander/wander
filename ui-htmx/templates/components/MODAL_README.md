# Modal Component

A fully-featured modal dialog component built with Alpine.js for state management and Tailwind CSS for styling.

## Files

- `modal.templ` - Main modal component implementation
- `modal_example.templ` - Example usage patterns

## Components

### `Modal(props ModalProps)`

Main modal wrapper with backdrop and animations.

**Props:**
- `ID` (string) - Unique identifier for the modal
- `Title` (string) - Modal title text
- `Size` (string) - Modal width: "sm", "md", "lg", "xl", "full" (default: "md")
- `Closable` (bool) - Enable close button, escape key, and backdrop click to close
- `Class` (string) - Additional CSS classes

### `ModalHeader(props ModalHeaderProps)`

Modal header with title and optional close button. Automatically included in `Modal` component.

### `ModalBody()`

Scrollable content area. Place your modal content here.

### `ModalFooter()`

Action buttons area with right-aligned layout.

## Features

### State Management
- Uses Alpine.js `modal()` pattern from `alpine-patterns.js`
- `isOpen` state controls visibility
- `open()` method shows modal and locks body scroll
- `close()` method hides modal and restores scroll

### Accessibility
- ARIA dialog pattern with proper roles and labels
- Focus trap keeps keyboard navigation within modal
- Escape key support (when closable)
- Auto-focus first focusable element on open

### Animations
- Smooth fade transitions for backdrop
- Scale and slide animations for modal content
- CSS transitions configured via Tailwind

### Body Scroll Lock
- Prevents background scrolling when modal is open
- Automatically restored when modal closes

### Responsive Sizing
- Mobile-first design with full-width on small screens
- Configurable max-width on larger screens
- Sticky header and footer for long content

## Size Reference

| Size | Max Width | Tailwind Class |
|------|-----------|----------------|
| sm   | 384px     | sm:max-w-sm    |
| md   | 512px     | sm:max-w-md    |
| lg   | 640px     | sm:max-w-lg    |
| xl   | 768px     | sm:max-w-xl    |
| full | 90vw      | sm:max-w-[90vw]|

## Usage Patterns

### Basic Static Modal

```go
<div x-data="modal()">
    <button @click="open">Open Modal</button>

    @Modal(ModalProps{
        ID: "my-modal",
        Title: "Example Modal",
        Size: "md",
        Closable: true,
    }) {
        @ModalBody() {
            <p>Your content here</p>
        }
        @ModalFooter() {
            <button @click="close">Cancel</button>
            <button>Confirm</button>
        }
    }
</div>
```

### HTMX Lazy-Load Modal

Load modal content from server when opened:

```go
<div x-data="modal()">
    <button
        @click="open"
        hx-get="/api/modal-content"
        hx-target="#modal-body-content"
        hx-trigger="click once">
        Open Modal
    </button>

    @Modal(ModalProps{ID: "htmx-modal", Title: "Dynamic", Size: "lg"}) {
        @ModalBody() {
            <div id="modal-body-content">Loading...</div>
        }
    }
</div>
```

### Form Modal with HTMX Submission

```go
<div x-data="modal()">
    <button @click="open">Create Item</button>

    @Modal(ModalProps{ID: "form-modal", Title: "New Item", Size: "md"}) {
        <form
            hx-post="/api/items"
            hx-on::after-request="if(event.detail.successful) { close() }">
            @ModalBody() {
                <input type="text" name="name" required />
            }
            @ModalFooter() {
                <button @click="close" type="button">Cancel</button>
                <button type="submit">Create</button>
            }
        </form>
    }
</div>
```

### Non-Closable Modal (Required Action)

Force user to take action - no escape key or backdrop close:

```go
<div x-data="modal()">
    <button @click="open">Open Required Modal</button>

    @Modal(ModalProps{
        ID: "required-modal",
        Title: "Action Required",
        Size: "md",
        Closable: false, // No X button, escape, or backdrop close
    }) {
        @ModalBody() {
            <p>You must choose an option.</p>
        }
        @ModalFooter() {
            <button @click="close">Decline</button>
            <button @click="close">Accept</button>
        }
    }
</div>
```

### Multiple Modals (Nested/Sequential)

Each modal needs its own Alpine data context:

```go
<div x-data="modal()">
    <button @click="open">Open First Modal</button>

    @Modal(ModalProps{ID: "modal-1", Title: "First Modal"}) {
        @ModalBody() {
            <div x-data="modal()">
                <button @click="open">Open Second Modal</button>

                @Modal(ModalProps{ID: "modal-2", Title: "Second Modal"}) {
                    @ModalBody() {
                        <p>Nested modal content</p>
                    }
                }
            </div>
        }
    }
</div>
```

## Alpine.js Integration

### Methods Available in x-data="modal()"

```javascript
// Open the modal
open()

// Close the modal
close()

// Check state
isOpen // boolean
```

### Calling from HTMX Events

```html
<!-- Close modal after successful form submission -->
<form hx-post="/api/save" hx-on::after-request="if(event.detail.successful) { close() }">
```

### Programmatic Control

```javascript
// Get Alpine component and control it
const modal = Alpine.$data(document.querySelector('[x-data*="modal"]'));
modal.open();
modal.close();
```

## Styling Customization

### Custom CSS Classes

Add custom classes via the `Class` prop:

```go
@Modal(ModalProps{
    ID: "custom-modal",
    Title: "Custom",
    Class: "shadow-2xl border-2 border-primary-500",
})
```

### Override Colors

The modal uses Tailwind's semantic color tokens:
- `primary-*` for action buttons and focus rings
- `neutral-*` for text and borders

Configure in your Tailwind config to theme globally.

## Best Practices

### Accessibility
1. Always provide a descriptive `Title`
2. Use semantic HTML in modal body (headings, lists, etc.)
3. Ensure keyboard navigation works (focus trap is automatic)
4. Make sure action buttons have clear labels

### UX Guidelines
1. Use `Closable: false` sparingly - only for critical actions
2. Keep modals focused on single tasks
3. Use appropriate size for content (don't force scrolling unnecessarily)
4. Provide clear primary and secondary actions in footer
5. Consider mobile experience - test on small screens

### HTMX Integration
1. Use `hx-trigger="click once"` to prevent duplicate loads
2. Show loading indicators during fetch
3. Close modal on successful form submission
4. Handle errors gracefully within modal body

### Performance
1. Lazy-load content when possible (HTMX mode)
2. Don't nest too many modals (increases complexity)
3. Clean up large content after modal closes if needed

## Examples

See `modal_example.templ` for working examples:
1. Basic modal with static content
2. HTMX lazy-load modal
3. Form submission modal
4. All size variations
5. Non-closable required action modal
6. Scrollable content modal

## Troubleshooting

### Modal doesn't open
- Ensure `x-data="modal()"` is on parent element
- Check Alpine.js is loaded (alpine-patterns.js)
- Verify `@click="open"` is on trigger button

### Can't close modal
- Check `Closable: true` if you want close button
- Verify escape key handler isn't being blocked
- Ensure `@click="close"` is on close buttons

### Content not scrolling
- ModalBody has `max-h-[calc(100vh-16rem)]` with overflow-y-auto
- Check if content is wrapped properly in ModalBody
- Verify no conflicting overflow styles

### Focus trap not working
- Ensure `x-trap="isOpen"` is on modal content div (it's automatic)
- Check Alpine.js Focus plugin is loaded
- Verify there are focusable elements in modal

### HTMX content not loading
- Check network tab for failed requests
- Verify `hx-target` ID matches element in ModalBody
- Ensure `hx-trigger` is configured correctly
- Check server endpoint returns valid HTML

## Related Components

- **Dropdown** - For menu-style selection dialogs
- **Tabs** - For organizing modal content into sections
- **Alert/Toast** - For non-blocking notifications (future component)

## Alpine.js Pattern Source

The `modal()` function is defined in `/ui-htmx/static/js/alpine-patterns.js`:

```javascript
function modal(initialOpen = false) {
    return {
        isOpen: initialOpen,
        open() { /* ... */ },
        close() { /* ... */ },
        reinit() { /* ... */ }
    };
}
```

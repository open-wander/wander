# Alpine.js Component Patterns for Wander UI

This directory contains Alpine.js resources for the Wander UI modernization.

## Files

- **alpine.min.js** - Alpine.js v3 CSP build, vendored from the pinned
  `@alpinejs/csp` package by `npm run vendor:alpine`
- **alpine-patterns.js** - Reusable Alpine.js component patterns and helpers
- **htmx.min.js** - HTMX library for server interactions

## Quick Start

Include these scripts in your HTML template (order matters):

```html
<!-- Load Alpine.js patterns first -->
<script src="/static/js/alpine-patterns.js"></script>

<!-- Load Alpine.js (will auto-initialize) -->
<script defer src="/static/js/alpine.min.js"></script>

<!-- Load HTMX -->
<script src="/static/js/htmx.min.js"></script>
```

## Using Patterns in Templ Templates

### Dropdown Example

```go
templ DropdownComponent() {
    <div x-data="alpinePatterns.dropdown()">
        <button @click="toggle"
                @keydown.escape="close"
                :aria-expanded="open"
                class="px-4 py-2 bg-blue-500 text-white rounded">
            Select Option
        </button>

        <div x-show="open"
             x-cloak
             @click.away="close"
             class="absolute mt-2 w-48 bg-white shadow-lg rounded"
             role="menu">
            <a href="#"
               @click="selectItem('option1')"
               class="block px-4 py-2 hover:bg-gray-100"
               role="menuitem">
                Option 1
            </a>
            <a href="#"
               @click="selectItem('option2')"
               class="block px-4 py-2 hover:bg-gray-100"
               role="menuitem">
                Option 2
            </a>
        </div>
    </div>
}
```

### Modal Example

```go
templ ModalComponent() {
    <div x-data="alpinePatterns.modal()">
        <button @click="open"
                class="px-4 py-2 bg-green-500 text-white rounded">
            Open Modal
        </button>

        <div x-show="isOpen"
             x-cloak
             @keydown.escape.window="close"
             class="fixed inset-0 z-50 flex items-center justify-center"
             x-transition>

            <!-- Backdrop -->
            <div @click="close"
                 class="fixed inset-0 bg-black bg-opacity-50"></div>

            <!-- Modal Content -->
            <div class="relative bg-white rounded-lg p-6 max-w-md w-full mx-4 z-10"
                 @click.stop
                 x-trap="isOpen">
                <button @click="close"
                        class="absolute top-4 right-4 text-gray-500 hover:text-gray-700"
                        aria-label="Close">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                </button>

                <h2 class="text-xl font-bold mb-4">Modal Title</h2>
                <p>Modal content goes here</p>
            </div>
        </div>
    </div>
}
```

### Tabs Example

```go
templ TabsComponent() {
    <div x-data="alpinePatterns.tabs(['overview', 'details', 'settings'])">
        <div class="flex border-b" role="tablist">
            <button @click="setActive('overview')"
                    :class="{ 'border-b-2 border-blue-500 text-blue-600': activeTab === 'overview' }"
                    class="px-4 py-2"
                    role="tab"
                    :aria-selected="activeTab === 'overview'">
                Overview
            </button>
            <button @click="setActive('details')"
                    :class="{ 'border-b-2 border-blue-500 text-blue-600': activeTab === 'details' }"
                    class="px-4 py-2"
                    role="tab">
                Details
            </button>
            <button @click="setActive('settings')"
                    :class="{ 'border-b-2 border-blue-500 text-blue-600': activeTab === 'settings' }"
                    class="px-4 py-2"
                    role="tab">
                Settings
            </button>
        </div>

        <div class="p-4">
            <div role="tabpanel" x-show="activeTab === 'overview'">
                <h3 class="font-bold">Overview</h3>
                <p>Overview content here</p>
            </div>
            <div role="tabpanel" x-show="activeTab === 'details'">
                <h3 class="font-bold">Details</h3>
                <p>Details content here</p>
            </div>
            <div role="tabpanel" x-show="activeTab === 'settings'">
                <h3 class="font-bold">Settings</h3>
                <p>Settings content here</p>
            </div>
        </div>
    </div>
}
```

## HTMX + Alpine Integration

### Example: Lazy-Loading Tabs

```go
templ LazyTabsComponent() {
    <div x-data="alpinePatterns.tabs(['jobs', 'allocations', 'events'], { lazyLoad: true })">
        <div class="flex border-b" role="tablist">
            <button @click="loadTab('jobs', '/api/tabs/jobs')"
                    :class="{ 'border-b-2 border-blue-500': activeTab === 'jobs' }"
                    class="px-4 py-2">
                Jobs
            </button>
            <button @click="loadTab('allocations', '/api/tabs/allocations')"
                    :class="{ 'border-b-2 border-blue-500': activeTab === 'allocations' }"
                    class="px-4 py-2">
                Allocations
            </button>
            <button @click="loadTab('events', '/api/tabs/events')"
                    :class="{ 'border-b-2 border-blue-500': activeTab === 'events' }"
                    class="px-4 py-2">
                Events
            </button>
        </div>

        <div id="tab-content" class="p-4">
            <!-- HTMX will load content here -->
        </div>
    </div>
}
```

### Example: Modal with HTMX Content

```go
templ DynamicModalComponent() {
    <div x-data="alpinePatterns.modal()">
        <button hx-get="/api/modal-content"
                hx-target="#modal-body"
                hx-swap="innerHTML"
                @click="open"
                class="px-4 py-2 bg-blue-500 text-white rounded">
            Load Modal
        </button>

        <div x-show="isOpen"
             @keydown.escape.window="close"
             class="fixed inset-0 z-50 flex items-center justify-center">
            <div @click="close" class="fixed inset-0 bg-black bg-opacity-50"></div>
            <div id="modal-body"
                 class="relative bg-white rounded-lg p-6 max-w-2xl w-full mx-4"
                 @click.stop>
                <!-- HTMX loads content here -->
            </div>
        </div>
    </div>
}
```

### Example: Dropdown with Server-Loaded Options

```go
templ ServerDropdownComponent() {
    <div x-data="alpinePatterns.dropdown()"
         x-init="$watch('open', value => {
             if (value && !loaded) {
                 htmx.ajax('GET', '/api/dropdown-options', {target: '#dropdown-content'});
                 loaded = true;
             }
         })">
        <button @click="toggle" class="px-4 py-2 bg-blue-500 text-white rounded">
            Select Server Option
        </button>

        <div x-show="open"
             @click.away="close"
             id="dropdown-content"
             class="absolute mt-2 w-48 bg-white shadow-lg rounded">
            <!-- HTMX will populate options here -->
        </div>
    </div>
}
```

## When to Use What

### Use Alpine.js For:
- **UI State**: Open/close states for dropdowns, modals, tooltips
- **Active Selection**: Which tab is active, which item is selected
- **Client-Side Validation**: Form validation before submission
- **Animations**: Show/hide transitions, slide-ins, fades
- **Keyboard Navigation**: Arrow keys, Escape key handling
- **Local Interactions**: Toggling visibility, expanding/collapsing sections

### Use HTMX For:
- **Server Communication**: Fetching data, submitting forms
- **Content Updates**: Loading new HTML from server
- **Server-Side Validation**: Validating form data on the server
- **Navigation**: Updating page content without full reload
- **Real-Time Updates**: Polling, SSE for live data

### Use Both Together:
- Alpine manages UI state (is modal open?)
- HTMX loads content (fetch modal body from server)
- Alpine triggers HTMX requests (on tab click, load tab content)
- HTMX events update Alpine state (after successful submit, close modal)

## Important Notes

### x-cloak Directive
Add this CSS to prevent flash of unstyled content:

```css
[x-cloak] {
    display: none !important;
}
```

### HTMX Swaps and Alpine
When HTMX swaps content, Alpine will automatically initialize any new `x-data` components in the swapped HTML. No special configuration needed.

### Focus Management
Use Alpine's `x-trap` directive for focus trapping in modals:

```html
<div x-trap="isOpen">
    <!-- Focus will be trapped here when isOpen is true -->
</div>
```

### Accessibility
All patterns include ARIA attributes and keyboard navigation. Ensure you:
- Use semantic HTML (`<button>`, not `<div>`)
- Include ARIA roles (`role="menu"`, `role="tablist"`)
- Handle keyboard navigation (Escape, Arrow keys)
- Manage focus appropriately

## Available Patterns

See `alpine-patterns.js` for the complete list of patterns:

1. **dropdown()** - Dropdowns and select menus
2. **modal()** - Modal dialogs
3. **tabs()** - Tab interfaces
4. **toggle()** - Simple on/off toggles
5. **accordion()** - Expandable sections
6. **formValidation()** - Client-side form validation

Each pattern is thoroughly documented with usage examples in the source file.

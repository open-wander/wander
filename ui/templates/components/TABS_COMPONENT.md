# Tabs Component Documentation

## Overview

The Tabs component provides a complete tabbed interface implementation with support for both static (pre-rendered) and HTMX (lazy-loaded) content modes. It follows ARIA accessibility patterns and integrates seamlessly with Alpine.js for state management and HTMX for dynamic content loading.

## Features

### Accessibility
- Full ARIA tabs pattern implementation (role="tablist", role="tab", role="tabpanel")
- Keyboard navigation support (Arrow keys, Home, End)
- Proper aria-selected and aria-controls attributes
- Focus management with tabindex
- Screen reader friendly

### Two Operating Modes

#### 1. Static Mode
- All tab content is rendered upfront
- Alpine.js handles tab switching (no server requests)
- Instant switching between tabs
- Best for: Small amounts of content, offline functionality

#### 2. HTMX Mode
- Tab content is lazy-loaded from the server on demand
- Reduces initial page load size
- Shows loading indicator during fetch
- Best for: Large datasets, dynamic content, performance optimization

### Styling
- Tailwind CSS utility classes
- Active tab: border-b-2 border-primary-500 with text-primary-600
- Inactive tabs: text-neutral-500 with hover states
- Focus ring for keyboard navigation
- Responsive design

### Icon Support
- Optional Heroicons integration
- Built-in icons: home, user, chart, cog, document, clock
- Icons automatically change color with tab state
- Easy to extend with additional icons

## File Structure

```
ui-htmx/templates/components/tabs.templ          # Main tabs component
ui-htmx/templates/pages/tabs_demo.templ          # Demo page showcasing both modes
ui-htmx/templates/partials/tabs_demo.templ       # HTMX endpoint content for demo
ui-htmx/handlers/tabs_demo.go                    # Handler for demo routes
```

## Usage

### Basic Static Tabs

```go
@components.Tabs(components.TabsProps{
    Tabs: []components.Tab{
        {
            ID:      "overview",
            Label:   "Overview",
            Icon:    "home",
            Content: overviewContent(),  // templ.Component
            Active:  true,
        },
        {
            ID:      "details",
            Label:   "Details",
            Icon:    "document",
            Content: detailsContent(),
        },
        {
            ID:      "settings",
            Label:   "Settings",
            Icon:    "cog",
            Content: settingsContent(),
        },
    },
    ActiveTab: "overview",
    Mode:      "static",
    PanelID:   "content-panel",
})
```

### HTMX Lazy-Loading Tabs

```go
@components.Tabs(components.TabsProps{
    Tabs: []components.Tab{
        {
            ID:      "jobs",
            Label:   "Jobs",
            Icon:    "briefcase",
            URL:     "/api/jobs/123/overview",
            Content: initialJobsContent(),  // Optional initial content
            Active:  true,
        },
        {
            ID:      "allocations",
            Label:   "Allocations",
            Icon:    "cube",
            URL:     "/api/jobs/123/allocations",
        },
        {
            ID:      "history",
            Label:   "History",
            Icon:    "clock",
            URL:     "/api/jobs/123/history",
        },
    },
    ActiveTab: "jobs",
    Mode:      "htmx",
    PanelID:   "job-details-panel",
})
```

### Handler Implementation for HTMX Tabs

```go
// In your handler
func (h *JobHandler) OverviewTab(w http.ResponseWriter, r *http.Request) {
    jobID := chi.URLParam(r, "jobID")

    // Fetch data
    job := h.fetchJob(jobID)

    // Render only the partial content for the tab
    err := handlers.RenderPartial(w, r, partials.JobOverviewTab(job))
    if err != nil {
        http.Error(w, "Failed to render tab", http.StatusInternalServerError)
        return
    }
}
```

## Component API

### TabsProps

```go
type TabsProps struct {
    Tabs      []Tab   // List of tabs to render
    ActiveTab string  // ID of the initially active tab
    Mode      string  // "static" or "htmx"
    PanelID   string  // ID for the tab panel container (HTMX target)
}
```

### Tab

```go
type Tab struct {
    ID      string           // Unique identifier for the tab
    Label   string           // Display text for the tab button
    Icon    string           // Optional Heroicon name (e.g., "home", "user")
    URL     string           // For HTMX mode - the endpoint to fetch content
    Content templ.Component  // For static mode - pre-rendered content
    Active  bool             // Whether this tab is initially active
}
```

## Integration with Alpine.js

The component uses the `tabs()` Alpine.js function defined in `alpine-patterns.js`:

```javascript
// Automatically configured by the component
x-data="tabs(['tab1', 'tab2', 'tab3'], {
    defaultTab: 'tab1',
    lazyLoad: true
})"
```

### Available Alpine Methods

- `setActive(tabId)` - Switch to a specific tab
- `isActive(tabId)` - Check if a tab is active
- `loadTab(tabId, url)` - For HTMX mode: load tab content from URL
- `handleKeydown(event, currentTab)` - Keyboard navigation handler

## Keyboard Navigation

- **Left/Right Arrow**: Navigate between tabs
- **Home**: Jump to first tab
- **End**: Jump to last tab
- **Tab**: Focus next focusable element
- **Shift+Tab**: Focus previous focusable element

## Icons

### Built-in Icons

- `home` - Home icon
- `user` - User/profile icon
- `chart` - Bar chart/analytics icon
- `cog` - Settings/gear icon
- `document` - Document/file icon
- `clock` - Clock/history icon

### Adding New Icons

Edit `/ui-htmx/templates/components/tabs.templ` in the `tabIcon` template:

```go
templ tabIcon(iconName string, tabID string) {
    // ... existing code ...
    } else if iconName == "your-icon-name" {
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="YOUR_SVG_PATH"></path>
    }
    // ...
}
```

## HTMX Integration Details

### How It Works

1. User clicks tab button
2. Alpine.js updates `activeTab` state (visual feedback)
3. HTMX triggers GET request to the tab's URL
4. Server returns HTML fragment (partial template)
5. HTMX swaps content into the panel
6. Loading indicator shows/hides automatically

### HTMX Attributes on Tab Buttons

```html
hx-get="/api/jobs/123/overview"  <!-- Endpoint URL -->
hx-target="#job-panel"            <!-- Where to put the response -->
hx-swap="innerHTML"               <!-- How to swap content -->
hx-trigger="click"                <!-- When to trigger request -->
```

### Loading Indicator

The component includes a built-in loading spinner that appears during HTMX requests:

```html
<div id="tab-loading" class="htmx-indicator">
    <!-- Spinner + "Loading..." text -->
</div>
```

## Best Practices

### When to Use Static Mode
- Content is small and can be rendered quickly
- All tabs will likely be viewed by users
- Offline functionality is required
- SEO is important (all content in initial page load)

### When to Use HTMX Mode
- Tab content is large or expensive to generate
- Users typically only view 1-2 tabs
- Content is dynamic and changes frequently
- Optimizing initial page load time is critical

### Performance Tips

1. **Static Mode**: Keep individual tab content lightweight
2. **HTMX Mode**:
   - Implement server-side caching for tab endpoints
   - Add slight delay simulation if needed to prevent double-clicking
   - Consider pre-loading adjacent tabs on hover

### Accessibility Tips

1. Always provide meaningful labels
2. Use icons to supplement, not replace, text labels
3. Test keyboard navigation thoroughly
4. Ensure sufficient color contrast for active/inactive states

## Demo Page

View the complete demo at `/ui/demo/tabs` (once routes are configured).

The demo page (`/ui-htmx/templates/pages/tabs_demo.templ`) shows:
- Static tabs with different content types
- HTMX tabs with lazy-loading
- All accessibility features
- Icon usage examples
- Feature documentation

## Testing

### Manual Testing Checklist

- [ ] Click each tab to verify content switches
- [ ] Test keyboard navigation (arrows, Home, End)
- [ ] Verify ARIA attributes with screen reader
- [ ] Check loading indicator appears/disappears
- [ ] Test on mobile (if responsive design needed)
- [ ] Verify active tab styling
- [ ] Test with icons and without icons
- [ ] Check focus states

### Integration Testing

```go
// Example test structure
func TestTabsComponent(t *testing.T) {
    // Test that tabs render with correct attributes
    // Test that HTMX attributes are present in htmx mode
    // Test that Alpine.js data attribute is correctly formatted
}
```

## Troubleshooting

### Tabs Not Switching
- Check that Alpine.js is loaded
- Verify `alpine-patterns.js` is included
- Check browser console for JavaScript errors

### HTMX Requests Not Working
- Verify HTMX is loaded before the component
- Check that endpoints return valid HTML
- Use browser DevTools Network tab to inspect requests
- Verify `hx-target` ID matches the panel ID

### Styling Issues
- Ensure Tailwind CSS is compiled and loaded
- Check for CSS conflicts with existing styles
- Verify `primary-*` color classes are defined in Tailwind config

### Icons Not Showing
- Check that icon name matches one of the built-in icons
- Verify SVG paths are valid
- Check that icon color classes are applied

## Future Enhancements

Potential improvements to consider:

1. **URL Hash Sync**: Sync active tab with URL hash for bookmarking
2. **Animation**: Add transition effects when switching tabs
3. **Lazy Pre-loading**: Pre-fetch adjacent tabs on hover
4. **Vertical Tabs**: Support vertical tab orientation
5. **Nested Tabs**: Support tabs within tabs
6. **Close Button**: Add optional close button for tabs
7. **Drag to Reorder**: Allow tab reordering (advanced)

## Related Components

- **Accordion**: Similar collapsible pattern, vertical orientation
- **Modal**: For overlay content instead of inline tabs
- **Dropdown**: For selecting single options

## References

- [ARIA Tabs Pattern](https://www.w3.org/WAI/ARIA/apg/patterns/tabs/)
- [HTMX Documentation](https://htmx.org)
- [Alpine.js Documentation](https://alpinejs.dev)
- [Tailwind CSS](https://tailwindcss.com)
- [Heroicons](https://heroicons.com)

# EmptyState Component

The EmptyState component displays a centered no-data state with an icon, title, description, and optional action button. Perfect for empty search results, missing data, or initial states.

## Component

- **File**: `empty_state.templ`
- **Package**: `components`

## Props

```go
type EmptyStateProps struct {
    Icon        string // icon name (e.g., "briefcase", "folder", "search")
    Title       string // main title text
    Description string // descriptive text
    ActionLabel string // optional button text
    ActionURL   string // optional button URL (used with HTMX)
    Class       string // additional CSS classes
}
```

## Features

- Large centered icon with subtle styling
- Clear title and description text
- Optional action button with HTMX support
- Fully centered and responsive layout
- Dark mode support
- Minimum height of 400px for consistent spacing

## Usage Examples

### Basic Empty State (No Jobs)

```go
@EmptyState(EmptyStateProps{
    Icon: "briefcase",
    Title: "No jobs found",
    Description: "There are no jobs matching your criteria. Try adjusting your filters.",
})
```

### With Action Button (No Allocations)

```go
@EmptyState(EmptyStateProps{
    Icon: "folder",
    Title: "No allocations",
    Description: "This job doesn't have any allocations yet.",
    ActionLabel: "View All Jobs",
    ActionURL: "/jobs",
})
```

### Empty Search Results

```go
@EmptyState(EmptyStateProps{
    Icon: "search",
    Title: "No results found",
    Description: "We couldn't find anything matching your search. Try different keywords.",
    ActionLabel: "Clear Search",
    ActionURL: "/jobs",
})
```

### Custom Styling

```go
@EmptyState(EmptyStateProps{
    Icon: "exclamation-circle",
    Title: "No data available",
    Description: "Data will appear here once it becomes available.",
    Class: "bg-neutral-50 dark:bg-neutral-900 rounded-lg",
})
```

## Available Icons

Common icons for empty states:
- `briefcase` - For jobs
- `folder` - For allocations, groups, collections
- `search` - For search results
- `cube` - For nodes, resources
- `server` - For servers, infrastructure
- `document` - For files, documents
- `exclamation-circle` - For warnings
- `information-circle` - For info states

See `icon.templ` for the full list of available icons.

## Layout

The component is centered both horizontally and vertically with:
- Minimum height: 400px
- Vertical padding: 48px (py-12)
- Horizontal padding: 16px (px-4)
- Icon: 64x64px (h-16 w-16)
- Max description width: 448px (max-w-md)

## Action Button

When both `ActionLabel` and `ActionURL` are provided:
- Uses primary button variant
- Medium size
- HTMX enabled with `hx-get`
- Default target: `#main-content`
- Default swap: `innerHTML`

## Integration Example

```go
templ JobsPage(jobs []Job) {
    @Layout() {
        if len(jobs) == 0 {
            @EmptyState(EmptyStateProps{
                Icon: "briefcase",
                Title: "No jobs found",
                Description: "Get started by creating your first job.",
                ActionLabel: "Create Job",
                ActionURL: "/jobs/new",
            })
        } else {
            @JobsTable(jobs)
        }
    }
}
```

## Accessibility

- Icons use `aria-hidden="true"` (decorative)
- Semantic heading structure (h3 for title)
- Clear hierarchy with proper text sizing
- Sufficient color contrast in both light and dark modes

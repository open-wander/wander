# Icon Component

The Icon component renders inline SVG icons from Heroicons (outline style) with Tailwind CSS integration.

## Features

- **Inline SVG**: Full styling control with CSS/Tailwind
- **Size variants**: sm (16px), md (20px), lg (24px), xl (32px)
- **Color inheritance**: Uses `currentColor` for easy theming
- **Accessibility**: ARIA-hidden for decorative icons
- **40+ icons**: Common UI, navigation, status, and action icons

## Usage

```go
// Basic usage
@Icon(IconProps{Name: "home"})

// With size
@Icon(IconProps{Name: "check-circle", Size: "lg"})

// With custom classes (color, margin, etc.)
@Icon(IconProps{Name: "exclamation-triangle", Size: "md", Class: "text-red-500 mr-2"})
@Icon(IconProps{Name: "briefcase", Class: "text-blue-600"})

// In a button
<button class="flex items-center">
    @Icon(IconProps{Name: "plus", Size: "sm", Class: "mr-1"})
    <span>Add Item</span>
</button>

// Status badge
<div class="flex items-center">
    @Icon(IconProps{Name: "check-circle", Class: "text-green-500 mr-2"})
    <span>Success</span>
</div>
```

## Available Icons

### Navigation (6 icons)
- `home` - Home/dashboard
- `briefcase` - Jobs/work
- `server` - Servers/infrastructure
- `cube` - Allocations/containers
- `folder` - Directories/files
- `cog` - Settings/configuration

### Actions (7 icons)
- `play` - Start/run
- `stop` - Stop/halt
- `refresh` - Reload/sync
- `trash` - Delete
- `pencil` - Edit
- `plus` - Add/create
- `minus` - Remove/subtract

### Status (5 icons)
- `check-circle` - Success/completed
- `x-circle` - Error/failed
- `exclamation-circle` - Warning
- `information-circle` - Info/help
- `exclamation-triangle` - Alert/danger

### Arrows (8 icons)
- `chevron-down` - Dropdown/expand
- `chevron-right` - Next/forward
- `chevron-left` - Back/previous
- `chevron-up` - Collapse/up
- `arrow-right` - Navigate right
- `arrow-left` - Navigate left
- `arrow-up` - Navigate up
- `arrow-down` - Navigate down

### UI Elements (10 icons)
- `menu` - Hamburger menu
- `x` - Close/dismiss
- `search` - Search/find
- `external-link` - Open in new window
- `document` - File/document
- `clock` - Time/schedule
- `bell` - Notifications
- `user` - User profile
- `users` - Team/group
- `chart-bar` - Charts/analytics

### Additional (4 icons)
- `eye` - View/show
- `eye-slash` - Hide/hidden
- `link` - Link/connect
- `download` - Download
- `upload` - Upload
- `filter` - Filter/refine

## Size Reference

| Size | Class | Dimensions |
|------|-------|------------|
| sm   | h-4 w-4 | 16×16px |
| md   | h-5 w-5 | 20×20px (default) |
| lg   | h-6 w-6 | 24×24px |
| xl   | h-8 w-8 | 32×32px |

## Color Examples

```go
// Status colors
@Icon(IconProps{Name: "check-circle", Class: "text-green-500"})     // Success
@Icon(IconProps{Name: "x-circle", Class: "text-red-500"})          // Error
@Icon(IconProps{Name: "exclamation-circle", Class: "text-yellow-500"}) // Warning
@Icon(IconProps{Name: "information-circle", Class: "text-blue-500"})   // Info

// Neutral/muted
@Icon(IconProps{Name: "clock", Class: "text-neutral-400"})
@Icon(IconProps{Name: "user", Class: "text-neutral-600"})

// Interactive (hover states with Tailwind)
<button class="group">
    @Icon(IconProps{Name: "cog", Class: "text-neutral-500 group-hover:text-neutral-700"})
</button>
```

## Fallback Behavior

If an unknown icon name is provided, the component renders a question mark circle icon as a fallback.

```go
@Icon(IconProps{Name: "unknown-icon"})  // Renders question mark circle
```

## Integration with Existing Components

The Icon component replaces inline SVG patterns used in other components:

```go
// Before (inline SVG in tabs.templ)
<svg class="h-5 w-5" ...>
    <path d="..."/>
</svg>

// After (using Icon component)
@Icon(IconProps{Name: "home", Size: "md"})
```

## Extending with New Icons

To add new icons:

1. Visit [Heroicons](https://heroicons.com/) (outline style)
2. Copy the SVG path `d` attribute
3. Add new `else if` block in `iconPath()` in `icon.templ`
4. Run `make ui-generate` to compile

Example:
```go
} else if name == "new-icon" {
    <path stroke-linecap="round" stroke-linejoin="round" d="M..."/>
}
```

## Design Notes

- All icons use Heroicons v2 outline style
- 24×24 viewBox with 1.5 stroke-width
- Icons are decorative by default (aria-hidden="true")
- For semantic icons, wrap in element with aria-label
- Uses `currentColor` for stroke, inherits text color

## References

- [Heroicons Official Site](https://heroicons.com/)
- [Tailwind UI Icons](https://tailwindui.com/components/application-ui/elements/icons)
- [ARIA Icon Patterns](https://www.w3.org/WAI/ARIA/apg/patterns/button/)

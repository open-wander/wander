# Dropdown Component

A fully-featured dropdown menu component built with Alpine.js for state management and Templ for rendering. Supports icons, HTMX integration, keyboard navigation, and accessibility.

## Features

- **Alpine.js State Management** - Uses the `dropdown()` pattern from `alpine-patterns.js`
- **Click Outside to Close** - Automatically closes when clicking outside the menu
- **Keyboard Navigation** - Escape key closes the menu
- **Icon Support** - Heroicons integrated for visual clarity
- **Dividers** - Separate menu items into logical groups
- **Danger Styling** - Red styling for destructive actions (delete, remove, etc.)
- **HTMX Integration** - Direct support for HTMX GET/POST/DELETE actions
- **Accessibility** - Full ARIA attributes for screen readers
- **Smooth Transitions** - CSS transitions for open/close animations

## Components

### 1. `Dropdown` - Main Container

Wraps the entire dropdown and provides Alpine.js state.

```go
@Dropdown(DropdownProps{
    ID:       "my-dropdown",  // Required: unique identifier
    Position: "right",        // "left" or "right" alignment
    Width:    "md",          // "auto", "sm", "md", "lg"
    Class:    "ml-4",        // Optional additional classes
}) {
    // Children: DropdownTrigger and DropdownMenu
}
```

### 2. `DropdownTrigger` - Button/Element

The clickable element that opens the menu.

```go
@DropdownTrigger() {
    <button>Options</button>
}
```

### 3. `DropdownMenu` - Menu Panel

The popup menu container.

```go
@DropdownMenu(DropdownProps{Position: "right", Width: "md"}) {
    // Children: DropdownItemComponent and DropdownDividerItem
}
```

### 4. `DropdownItemComponent` - Menu Item

Individual menu items with various configurations.

```go
type DropdownItem struct {
    Label    string // Display text
    Href     string // Standard link URL
    Icon     string // Heroicon name (optional)
    Disabled bool   // Disable interaction
    Danger   bool   // Red styling for destructive actions
    HxPost   string // HTMX POST endpoint
    HxGet    string // HTMX GET endpoint
    HxDelete string // HTMX DELETE endpoint
    HxTarget string // HTMX target selector
    HxSwap   string // HTMX swap strategy (default: "innerHTML")
}
```

### 5. `DropdownDividerItem` - Separator

Visual divider between menu items.

```go
@DropdownDividerItem()
```

## Usage Examples

### Basic Dropdown with Links

```go
@Dropdown(DropdownProps{ID: "basic-menu", Position: "right", Width: "md"}) {
    @DropdownTrigger() {
        <button>Menu</button>
    }
    @DropdownMenu(DropdownProps{Position: "right", Width: "md"}) {
        @DropdownItemComponent(DropdownItem{Label: "Edit", Href: "/edit", Icon: "pencil"})
        @DropdownItemComponent(DropdownItem{Label: "View", Href: "/view", Icon: "eye"})
        @DropdownDividerItem()
        @DropdownItemComponent(DropdownItem{Label: "Delete", Icon: "trash", Danger: true, Href: "/delete"})
    }
}
```

### HTMX Actions Dropdown

```go
@Dropdown(DropdownProps{ID: "actions-menu", Position: "right", Width: "md"}) {
    @DropdownTrigger() {
        <button>Actions</button>
    }
    @DropdownMenu(DropdownProps{Position: "right", Width: "md"}) {
        @DropdownItemComponent(DropdownItem{
            Label:    "Approve",
            Icon:     "check",
            HxPost:   "/api/items/123/approve",
            HxTarget: "#item-123",
        })
        @DropdownItemComponent(DropdownItem{
            Label:    "Delete",
            Icon:     "trash",
            Danger:   true,
            HxDelete: "/api/items/123",
            HxTarget: "#item-123",
            HxSwap:   "outerHTML",
        })
    }
}
```

### User Menu Dropdown

```go
@Dropdown(DropdownProps{ID: "user-menu", Position: "right", Width: "sm"}) {
    @DropdownTrigger() {
        <img class="h-8 w-8 rounded-full" src="/avatar.jpg" alt="User"/>
    }
    @DropdownMenu(DropdownProps{Position: "right", Width: "sm"}) {
        @DropdownItemComponent(DropdownItem{Label: "Profile", Href: "/profile", Icon: "user"})
        @DropdownItemComponent(DropdownItem{Label: "Settings", Href: "/settings", Icon: "cog"})
        @DropdownDividerItem()
        @DropdownItemComponent(DropdownItem{Label: "Sign out", Href: "/logout", Icon: "logout"})
    }
}
```

### Compact "..." Menu

```go
@Dropdown(DropdownProps{ID: "dots-menu", Position: "left", Width: "auto"}) {
    @DropdownTrigger() {
        <button class="p-2">
            <svg class="h-5 w-5"><!-- Dots icon --></svg>
        </button>
    }
    @DropdownMenu(DropdownProps{Position: "left", Width: "auto"}) {
        @DropdownItemComponent(DropdownItem{Label: "Edit", Icon: "pencil", Href: "/edit"})
        @DropdownItemComponent(DropdownItem{Label: "Delete", Icon: "trash", Danger: true, HxDelete: "/api/delete"})
    }
}
```

## Available Icons

The component includes common Heroicons:

- `pencil`, `edit` - Edit icon
- `trash`, `delete` - Delete icon
- `eye`, `view` - View icon
- `duplicate`, `copy` - Copy icon
- `download` - Download icon
- `upload` - Upload icon
- `share` - Share icon
- `cog`, `settings` - Settings icon
- `user` - User icon
- `document` - Document icon
- `link` - Link icon
- `logout`, `sign-out` - Logout icon
- `dots-vertical`, `menu` - Menu icon

Default icon (info circle) is used for unrecognized icon names.

## Position Options

- `left` - Menu aligns to the left edge of the trigger
- `right` - Menu aligns to the right edge of the trigger (default)

## Width Options

- `auto` - Menu width fits content
- `sm` - Small width (12rem / 192px)
- `md` - Medium width (14rem / 224px) (default)
- `lg` - Large width (16rem / 256px)

## Alpine.js Integration

The dropdown uses the `dropdown()` pattern from `alpine-patterns.js`:

```javascript
dropdown() {
    return {
        open: false,
        toggle() { this.open = !this.open },
        close() { this.open = false },
        selectItem(value) {
            this.selectedValue = value;
            this.close();
        }
    }
}
```

### Accessing Alpine State

You can access the Alpine state in custom trigger buttons:

```go
@DropdownTrigger() {
    <button :class="{ 'bg-blue-500': open, 'bg-gray-500': !open }">
        <span x-text="open ? 'Close' : 'Open'"></span>
    </button>
}
```

## Accessibility

The component includes proper ARIA attributes:

- `role="button"` on trigger
- `aria-haspopup="true"` on trigger
- `:aria-expanded="open"` dynamic state
- `role="menu"` on menu
- `role="menuitem"` on items
- `role="separator"` on dividers
- `aria-disabled="true"` on disabled items

## Styling

The component uses Tailwind CSS classes. Key styling features:

- **Menu Shadow** - `shadow-lg` for depth
- **Menu Border** - `ring-1 ring-black ring-opacity-5`
- **Hover States** - `hover:bg-neutral-100` for normal items
- **Danger Styling** - `text-red-700 hover:bg-red-50` for destructive actions
- **Transitions** - Smooth scale and opacity animations
- **Disabled State** - `opacity-50 cursor-not-allowed`

## Best Practices

1. **Always provide an ID** - Required for ARIA and uniqueness
2. **Use icons for visual clarity** - Especially for common actions
3. **Group related items** - Use dividers to separate logical groups
4. **Mark destructive actions** - Use `Danger: true` for delete/remove/etc.
5. **Provide accessible labels** - Use descriptive text for screen readers
6. **Position appropriately** - Use `left` position for right-aligned triggers
7. **Choose appropriate width** - `auto` for compact menus, `md`/`lg` for text-heavy items

## Common Patterns

### Row Action Menu

```go
// In a table row
@Dropdown(DropdownProps{ID: "row-" + item.ID, Position: "left", Width: "auto"}) {
    @DropdownTrigger() {
        <button class="text-neutral-400 hover:text-neutral-500">
            <svg><!-- Dots vertical icon --></svg>
        </button>
    }
    @DropdownMenu(DropdownProps{Position: "left", Width: "auto"}) {
        @DropdownItemComponent(DropdownItem{Label: "Edit", Href: "/edit/" + item.ID})
        @DropdownItemComponent(DropdownItem{
            Label: "Delete",
            Danger: true,
            HxDelete: "/api/items/" + item.ID,
            HxTarget: "#row-" + item.ID,
            HxSwap: "outerHTML",
        })
    }
}
```

### Filter Dropdown

```go
@Dropdown(DropdownProps{ID: "filter-menu", Position: "right", Width: "md"}) {
    @DropdownTrigger() {
        <button>
            Filter
            <svg><!-- Chevron down --></svg>
        </button>
    }
    @DropdownMenu(DropdownProps{Position: "right", Width: "md"}) {
        @DropdownItemComponent(DropdownItem{
            Label: "Active",
            HxGet: "/items?filter=active",
            HxTarget: "#items-list",
        })
        @DropdownItemComponent(DropdownItem{
            Label: "Archived",
            HxGet: "/items?filter=archived",
            HxTarget: "#items-list",
        })
    }
}
```

## Files

- `/ui-htmx/templates/components/dropdown.templ` - Main component
- `/ui-htmx/templates/components/dropdown_example.templ` - Usage examples
- `/ui-htmx/static/js/alpine-patterns.js` - Alpine.js patterns (contains `dropdown()`)

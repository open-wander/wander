# Table Component

A comprehensive, sortable table component for the Wander HTMX UI migration.

## Files

- `table.templ` - Main table component implementation
- `table_example.templ` - Usage examples demonstrating all features
- `TABLE_README.md` - This documentation

## Components

### 1. Table (Main Wrapper)

The primary entry point for creating tables with responsive scrolling.

```go
@Table(TableProps{
    Columns: []Column{
        {Key: "name", Label: "Name", Sortable: true, Align: "left"},
        {Key: "status", Label: "Status", Sortable: true, Align: "center", Width: "w-32"},
    },
    SortColumn: "name",
    SortDir: "asc",
    Striped: true,
    Hover: true,
    SortURL: "/jobs",
}) {
    @TableBody() {
        @TableRow(true) {
            @TableCell("left", "") { Job Name }
            @TableCell("center", "w-32") { Running }
        }
    }
}
```

### 2. TableHeader

Automatically rendered by `Table()`. Generates sortable column headers with HTMX support.

**Features:**
- Sortable columns trigger HTMX GET requests
- Sort direction indicators (up/down arrows)
- Active column highlighting
- Keyboard accessible
- Focus ring styling

### 3. TableBody

Wraps table rows. Accepts children as `TableRow` components.

```go
@TableBody() {
    @TableRow(true) { ... }
    @TableRow(true) { ... }
}
```

### 4. TableRow

Individual table row with optional clickable behavior.

**Parameters:**
- `clickable` (bool) - Enables hover effects and cursor pointer

```go
@TableRow(true) {  // Clickable row
    @TableCell("left", "") { Data }
}

@TableRow(false) {  // Non-clickable row
    @TableCell("left", "") { Data }
}
```

### 5. TableCell

Individual table cell with alignment and width control.

**Parameters:**
- `align` (string) - "left", "center", "right", or "" for default (left)
- `width` (string) - Optional Tailwind width class (e.g., "w-32", "w-1/4")

```go
@TableCell("center", "w-32") {
    <span class="badge badge-success">Active</span>
}
```

### 6. EmptyState

Displays a message when the table has no data.

**Parameters:**
- `title` (string) - Main message
- `description` (string) - Optional subtitle

```go
@TableBody() {
    @EmptyState("No jobs found", "Try adjusting your filters")
}
```

### 7. TableLoadingIndicator

Shows a spinner during HTMX requests. Place outside the table.

```go
@Table(...) { ... }
@TableLoadingIndicator()
```

## Types

### Column

```go
type Column struct {
    Key      string // Data key for sorting
    Label    string // Display label
    Sortable bool   // Enable sorting
    Width    string // Optional width class (e.g., "w-32")
    Align    string // "left", "center", "right"
}
```

### TableProps

```go
type TableProps struct {
    Columns    []Column // Column definitions
    SortColumn string   // Current sort column key
    SortDir    string   // "asc" or "desc"
    Striped    bool     // Alternating row colors
    Compact    bool     // Reduced padding
    Hover      bool     // Row hover effect
    SortURL    string   // Base URL for sort links
    Class      string   // Additional CSS classes
}
```

## Features

### 1. Sortable Columns

Click column headers to sort. The component automatically toggles between ascending and descending order.

**HTMX Integration:**
- Sortable headers use `hx-get` to fetch sorted data
- URL format: `{SortURL}?sort={column}&dir={asc|desc}`
- Pushes URL to browser history with `hx-push-url="true"`
- Targets the entire table for swap with `hx-target="closest table"`

**Example Handler (Go):**
```go
func JobsHandler(w http.ResponseWriter, r *http.Request) {
    sortCol := r.URL.Query().Get("sort")
    sortDir := r.URL.Query().Get("dir")

    jobs := fetchJobs(sortCol, sortDir)

    // Render full table for HTMX requests
    components.Table(TableProps{
        Columns: columns,
        SortColumn: sortCol,
        SortDir: sortDir,
        ...
    }).Render(r.Context(), w)
}
```

### 2. Sort Direction Indicators

- **Active column:** Displays single arrow (up for asc, down for desc) in primary color
- **Inactive columns:** Displays double arrow in neutral color
- **Hover:** Inactive arrows change color on hover

### 3. Responsive Horizontal Scroll

Table wrapper includes `overflow-x-auto` and `scrollbar-thin` for horizontal scrolling on small screens while maintaining thin, styled scrollbars.

### 4. Styling Variants

**Striped Rows:**
```go
TableProps{
    Striped: true,  // Alternating row background colors
}
```

**Compact Spacing:**
```go
TableProps{
    Compact: true,  // Reduced padding in cells
}
```

**Row Hover:**
```go
TableProps{
    Hover: true,  // Enable hover effect on rows
}
```

### 5. Dark Mode Support

All components include dark mode styling using Tailwind's `dark:` variant.

## CSS Classes Available

From `input.css`:

- `.table` - Base table styles
- `.table-striped` - Striped row variant
- `.table-compact` - Compact spacing variant
- `.scrollbar-thin` - Thin scrollbar styling

## Usage Patterns

### Pattern 1: Standard Sortable Table

```go
@Table(TableProps{
    Columns: []Column{
        {Key: "name", Label: "Job Name", Sortable: true, Align: "left"},
        {Key: "status", Label: "Status", Sortable: true, Align: "center", Width: "w-32"},
        {Key: "created", Label: "Created", Sortable: true, Align: "right"},
    },
    SortColumn: "name",
    SortDir: "asc",
    Striped: true,
    Hover: true,
    SortURL: "/jobs",
}) {
    @TableBody() {
        for _, job := range jobs {
            @TableRow(true) {
                @TableCell("left", "") { { job.Name } }
                @TableCell("center", "w-32") {
                    <span class={ badgeClass(job.Status) }>{ job.Status }</span>
                }
                @TableCell("right", "") { { formatTime(job.Created) } }
            }
        }
    }
}
@TableLoadingIndicator()
```

### Pattern 2: Table with Empty State

```go
@Table(props) {
    @TableBody() {
        if len(items) == 0 {
            @EmptyState("No items found", "Try adjusting your search")
        } else {
            for _, item := range items {
                @TableRow(true) { ... }
            }
        }
    }
}
```

### Pattern 3: Mixed Sortable/Non-Sortable Columns

```go
Columns: []Column{
    {Key: "name", Label: "Name", Sortable: true, Align: "left"},
    {Key: "actions", Label: "Actions", Sortable: false, Align: "right"},
}
```

### Pattern 4: Custom Cell Content

```go
@TableCell("left", "") {
    <div class="flex items-center gap-3">
        <div class="avatar avatar-sm">JD</div>
        <div>
            <div class="font-medium">{ job.Name }</div>
            <div class="text-xs text-neutral-500">{ job.ID }</div>
        </div>
    </div>
}
```

## Accessibility

The table component follows ARIA best practices:

- Sortable headers use `<button>` elements for keyboard access
- Proper `<th scope="col">` attributes
- Focus ring styling with `focus:ring-2`
- Semantic table structure (`<table>`, `<thead>`, `<tbody>`, `<tr>`, `<th>`, `<td>`)

## Performance Considerations

1. **HTMX Swaps:** The entire table is swapped on sort, not the full page
2. **Indicator:** Loading spinner shows during requests via `hx-indicator`
3. **Scrolling:** Thin scrollbars prevent layout shifts
4. **Minimal JS:** No JavaScript required, pure HTMX

## Backend Integration

### Go Handler Example

```go
func (h *Handler) JobsTable(w http.ResponseWriter, r *http.Request) {
    // Parse sort parameters
    sortCol := r.URL.Query().Get("sort")
    if sortCol == "" {
        sortCol = "name"
    }

    sortDir := r.URL.Query().Get("dir")
    if sortDir != "asc" && sortDir != "desc" {
        sortDir = "asc"
    }

    // Fetch sorted data
    jobs, err := h.jobService.List(r.Context(), sortCol, sortDir)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Define columns
    columns := []components.Column{
        {Key: "name", Label: "Name", Sortable: true, Align: "left"},
        {Key: "status", Label: "Status", Sortable: true, Align: "center", Width: "w-32"},
        {Key: "created", Label: "Created", Sortable: true, Align: "right"},
    }

    // Check if HTMX request (partial vs full page)
    if r.Header.Get("HX-Request") == "true" {
        // Return just the table for HTMX swap
        components.Table(components.TableProps{
            Columns:    columns,
            SortColumn: sortCol,
            SortDir:    sortDir,
            Striped:    true,
            Hover:      true,
            SortURL:    "/jobs",
        }).Render(r.Context(), w)
    } else {
        // Return full page
        components.JobsPage(jobs, columns, sortCol, sortDir).Render(r.Context(), w)
    }
}
```

## Examples

See `table_example.templ` for complete working examples:

1. **ExampleJobsTable** - Full-featured table with all options
2. **ExampleEmptyTable** - Empty state demonstration
3. **ExampleCompactTable** - Compact variant
4. **ExampleNonSortableTable** - Table without sorting

## Testing

Generate and run examples:

```bash
# Generate templates
templ generate ui-htmx/templates/components/table.templ
templ generate ui-htmx/templates/components/table_example.templ

# Build
go build ./ui-htmx/templates/components/...

# Run example server (if available)
go run ./cmd/ui-server
```

## Migration from Ember/HDS

**Before (Ember + HDS):**
```handlebars
<Hds::Table @striped={{true}}>
  <:head>
    <tr>
      <Hds::Table::Th @sortable={{true}}>Name</Hds::Table::Th>
      <Hds::Table::Th @sortable={{true}}>Status</Hds::Table::Th>
    </tr>
  </:head>
  <:body>
    {{#each jobs as |job|}}
      <tr>
        <td>{{job.name}}</td>
        <td><Hds::Badge>{{job.status}}</Hds::Badge></td>
      </tr>
    {{/each}}
  </:body>
</Hds::Table>
```

**After (Templ + HTMX):**
```go
@Table(TableProps{
    Columns: []Column{
        {Key: "name", Label: "Name", Sortable: true, Align: "left"},
        {Key: "status", Label: "Status", Sortable: true, Align: "center"},
    },
    SortColumn: "name",
    SortDir: "asc",
    Striped: true,
    SortURL: "/jobs",
}) {
    @TableBody() {
        for _, job := range jobs {
            @TableRow(true) {
                @TableCell("left", "") { { job.Name } }
                @TableCell("center", "") {
                    <span class="badge badge-success">{ job.Status }</span>
                }
            }
        }
    }
}
```

## Future Enhancements

Potential improvements for future iterations:

- [ ] Column filtering
- [ ] Column resizing
- [ ] Column visibility toggle
- [ ] Sticky header on scroll
- [ ] Row selection (checkboxes)
- [ ] Bulk actions
- [ ] Pagination component integration
- [ ] Virtual scrolling for large datasets
- [ ] CSV/JSON export

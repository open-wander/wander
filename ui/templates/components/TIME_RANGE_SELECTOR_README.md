# Time Range Selector Component

A flexible time range selector component for chart data filtering with Alpine.js state management and HTMX integration.

## Features

- **Preset Time Ranges**: Quick-select buttons (5m, 15m, 1h, 6h, 24h, 7d)
- **Custom Range Picker**: Optional datetime picker for custom ranges
- **Event System**: Dispatches custom events for chart integration
- **URL Persistence**: Automatically updates URL params for shareable links
- **Dark Mode Support**: Full Tailwind dark mode styling
- **Accessible**: ARIA labels and keyboard navigation

## Basic Usage

### 1. Simple Time Range Selector

```go
@TimeRangeSelector(TimeRangeSelectorProps{
    ID: "chart-time-selector",
    Options: []TimeRangeOption{
        {ID: "5m", Label: "5m", Duration: "5m"},
        {ID: "1h", Label: "1h", Duration: "1h"},
        {ID: "24h", Label: "24h", Duration: "24h"},
    },
    DefaultOption: "1h",
})
```

### 2. With All Options

```go
@TimeRangeSelector(TimeRangeSelectorProps{
    ID: "advanced-selector",
    Options: components.DefaultTimeRangeOptions(),
    DefaultOption: "1h",
    UpdateURLParams: true,
    ShowCustomRange: true,
    Class: "mb-6",
})
```

### 3. Using Default Preset Options

```go
import "ui-htmx/templates/components"

// In your handler
props := components.TimeRangeSelectorProps{
    ID: "selector-1",
    Options: components.DefaultTimeRangeOptions(), // Returns: 5m, 15m, 1h, 6h, 24h, 7d
    DefaultOption: "1h",
}
```

## Integration with Charts

### CPU and Memory Chart Integration

The time range selector works seamlessly with CPU and Memory charts through custom events.

#### Example: Complete Page with Charts and Time Selector

```go
templ ClientDetailPage(client Client) {
    <div class="space-y-6">
        <!-- Time Range Selector -->
        <div class="flex justify-end">
            @TimeRangeSelector(TimeRangeSelectorProps{
                ID: "client-charts-time-range",
                Options: DefaultTimeRangeOptions(),
                DefaultOption: "1h",
                UpdateURLParams: true,
            })
        </div>

        <!-- Charts Grid -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- CPU Chart -->
            <div
                x-data="cpuChartWithTimeRange()"
                @timerange-change.window="handleTimeRangeChange($event)"
            >
                @CPUChart(CPUChartProps{
                    ID: "client-cpu-chart",
                    DataURL: "/api/clients/" + client.ID + "/cpu?range=1h",
                    Height: "80",
                })
            </div>

            <!-- Memory Chart -->
            <div
                x-data="memoryChartWithTimeRange()"
                @timerange-change.window="handleTimeRangeChange($event)"
            >
                @MemoryChart(MemoryChartProps{
                    ID: "client-memory-chart",
                    DataURL: "/api/clients/" + client.ID + "/memory?range=1h",
                    Height: "80",
                })
            </div>
        </div>
    </div>
}
```

#### Alpine.js Integration

Add this to your Alpine.js initialization or component:

```javascript
// Option 1: Extend existing chart component
document.addEventListener('alpine:init', () => {
    Alpine.data('cpuChartWithTimeRange', () => ({
        baseURL: '',

        init() {
            // Extract base URL from initial dataURL
            const chartElement = this.$el.querySelector('[x-data]');
            if (chartElement) {
                const initialData = Alpine.$data(chartElement);
                this.baseURL = initialData.dataURL || '';
            }
        },

        handleTimeRangeChange(event) {
            const { startTime, endTime } = event.detail;

            // Update chart data URL
            const chartElement = this.$el.querySelector('[x-data]');
            if (chartElement) {
                const chartData = Alpine.$data(chartElement);
                const url = new URL(this.baseURL, window.location.origin);
                url.searchParams.set('start', startTime.toISOString());
                url.searchParams.set('end', endTime.toISOString());

                // Update and refresh
                chartData.dataURL = url.toString();
                if (chartData.refreshChart) {
                    chartData.refreshChart();
                }
            }
        }
    }));
});
```

#### Option 2: Using Integration Helper

```javascript
import { setupChartTimeRange } from '/static/js/chart-time-range-integration.js';

// In your Alpine.js chart component init()
setupChartTimeRange(this, {
    baseURL: '/api/client/123/cpu',
    onRangeChange: (detail) => {
        console.log('Range changed:', detail);
    }
});
```

## Event Format

The component dispatches a `timerange-change` event (or custom event name) with the following detail:

```javascript
{
    range: "1h",              // Selected range ID
    label: "1h",              // Human-readable label
    duration: "1h",           // Duration value
    startTime: Date,          // Calculated start timestamp
    endTime: Date             // Calculated end timestamp (now)
}
```

### Listening to Events

```javascript
// Listen on specific element
document.getElementById('my-chart').addEventListener('timerange-change', (event) => {
    const { startTime, endTime, range } = event.detail;
    console.log(`Time range changed to ${range}`);
    updateChartData(startTime, endTime);
});

// Listen globally
document.addEventListener('timerange-change', (event) => {
    console.log('Global time range change:', event.detail);
});

// Listen in Alpine.js
<div @timerange-change.window="handleRangeChange($event)">
```

## Backend API Integration

### Expected Query Parameters

Your backend API should accept these query parameters:

```
GET /api/client/123/cpu?start=2024-01-26T14:00:00Z&end=2024-01-26T15:00:00Z
```

- `start`: ISO 8601 timestamp for range start
- `end`: ISO 8601 timestamp for range end

### Go Handler Example

```go
func (h *Handler) HandleCPUMetrics(w http.ResponseWriter, r *http.Request) {
    // Parse time range from query params
    startStr := r.URL.Query().Get("start")
    endStr := r.URL.Query().Get("end")

    var startTime, endTime time.Time
    var err error

    if startStr != "" && endStr != "" {
        startTime, err = time.Parse(time.RFC3339, startStr)
        if err != nil {
            http.Error(w, "Invalid start time", http.StatusBadRequest)
            return
        }

        endTime, err = time.Parse(time.RFC3339, endStr)
        if err != nil {
            http.Error(w, "Invalid end time", http.StatusBadRequest)
            return
        }
    } else {
        // Default to last 1 hour if not specified
        endTime = time.Now()
        startTime = endTime.Add(-1 * time.Hour)
    }

    // Fetch metrics for time range
    metrics := h.fetchCPUMetrics(clientID, startTime, endTime)

    // Return JSON
    json.NewEncoder(w).Encode(map[string]interface{}{
        "data": metrics,
    })
}
```

## Duration Format

Supported duration formats:

- `Xm` - minutes (e.g., `5m`, `15m`, `30m`)
- `Xh` - hours (e.g., `1h`, `6h`, `12h`, `24h`)
- `Xd` - days (e.g., `1d`, `7d`, `30d`)
- `Xw` - weeks (e.g., `1w`, `4w`)

## Component Props

### TimeRangeSelectorProps

| Prop | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `ID` | `string` | Yes | - | Unique identifier for the selector |
| `Options` | `[]TimeRangeOption` | Yes | - | Array of preset time range options |
| `DefaultOption` | `string` | No | First option | ID of initially selected option |
| `Class` | `string` | No | `""` | Additional CSS classes |
| `UpdateURLParams` | `bool` | No | `true` | Update URL query parameters |
| `EventName` | `string` | No | `"timerange-change"` | Custom event name |
| `ShowCustomRange` | `bool` | No | `false` | Show custom date picker |

### TimeRangeOption

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `ID` | `string` | Yes | Unique identifier (e.g., "5m") |
| `Label` | `string` | Yes | Display text (e.g., "5 minutes") |
| `Duration` | `string` | Yes | Duration format (e.g., "5m") |

## URL Parameter Persistence

When `UpdateURLParams` is enabled, the selector updates the URL:

```
# Preset range
https://app.example.com/clients/123?range=1h

# Custom range
https://app.example.com/clients/123?range=custom&start=2024-01-26T14:00:00Z&end=2024-01-26T15:00:00Z
```

This allows users to:
- Bookmark specific time ranges
- Share URLs with time context
- Browser back/forward navigation

## Custom Range Picker

Enable the custom range picker for advanced use cases:

```go
@TimeRangeSelector(TimeRangeSelectorProps{
    ID: "advanced-selector",
    Options: DefaultTimeRangeOptions(),
    ShowCustomRange: true,  // Enable custom picker
})
```

The custom picker provides:
- Start datetime input
- End datetime input
- Apply/Cancel buttons
- Validation (start must be before end)

## JavaScript Utilities

### Time Range Utils

Located in `/static/js/time-range-selector.js`:

```javascript
import {
    parseDuration,
    calculateTimeRange,
    formatTimeRange,
    formatDuration
} from '/static/js/time-range-selector.js';

// Parse duration string
const ms = parseDuration('1h'); // 3600000

// Calculate time range
const range = calculateTimeRange('1h');
// { start: Date, end: Date }

// Format for display
const formatted = formatTimeRange(range.start, range.end);
// "2:00 PM - 3:00 PM"
```

### Chart Integration Utils

Located in `/static/js/chart-time-range-integration.js`:

```javascript
import { setupChartTimeRange } from '/static/js/chart-time-range-integration.js';

// In Alpine.js component
setupChartTimeRange(this, {
    baseURL: '/api/metrics/cpu',
    eventName: 'timerange-change',
    onRangeChange: (detail) => {
        console.log('Range changed:', detail);
    }
});
```

## Styling

The component uses Tailwind CSS with full dark mode support:

```html
<!-- Light mode -->
bg-white text-neutral-700 border-neutral-300

<!-- Dark mode -->
dark:bg-neutral-800 dark:text-neutral-300 dark:border-neutral-600
```

### Custom Styling

Add custom classes via the `Class` prop:

```go
@TimeRangeSelector(TimeRangeSelectorProps{
    ID: "styled-selector",
    Options: DefaultTimeRangeOptions(),
    Class: "my-4 p-4 bg-neutral-100 rounded-lg",
})
```

## Examples

### Multiple Charts, One Selector

```go
templ DashboardPage() {
    <div x-data="{ timeRange: '1h' }">
        <!-- Single time range selector -->
        @TimeRangeSelector(TimeRangeSelectorProps{
            ID: "dashboard-time-range",
            Options: DefaultTimeRangeOptions(),
        })

        <!-- Multiple charts listening to same event -->
        <div class="grid grid-cols-2 gap-4 mt-6">
            <div @timerange-change.window="updateCPUChart($event)">
                @CPUChart(CPUChartProps{
                    ID: "cpu-1",
                    DataURL: "/api/cpu-1/metrics",
                })
            </div>

            <div @timerange-change.window="updateMemoryChart($event)">
                @MemoryChart(MemoryChartProps{
                    ID: "memory-1",
                    DataURL: "/api/memory-1/metrics",
                })
            </div>
        </div>
    </div>
}
```

### Compact Layout

```go
@TimeRangeSelector(TimeRangeSelectorProps{
    ID: "compact-selector",
    Options: []TimeRangeOption{
        {ID: "1h", Label: "1h", Duration: "1h"},
        {ID: "24h", Label: "24h", Duration: "24h"},
        {ID: "7d", Label: "7d", Duration: "7d"},
    },
    DefaultOption: "24h",
    Class: "flex items-center justify-end",
})
```

## Browser Compatibility

- Chrome/Edge: Full support
- Firefox: Full support
- Safari: Full support (datetime-local input requires iOS 14.5+)

## Accessibility

- Semantic HTML with proper ARIA labels
- Keyboard navigation support
- Screen reader compatible
- Focus indicators on all interactive elements

## Performance

- Event delegation for efficient memory usage
- Minimal DOM manipulation
- No external dependencies (except Alpine.js for state)
- Lazy-loaded JavaScript utilities

## Troubleshooting

### Charts not updating

1. Verify event listener is attached:
   ```javascript
   document.addEventListener('timerange-change', (e) => console.log(e));
   ```

2. Check chart has refresh method:
   ```javascript
   console.log(typeof chartInstance.refreshChart); // Should be 'function'
   ```

3. Verify URL format:
   ```javascript
   console.log('Data URL:', chartData.dataURL);
   ```

### URL params not persisting

- Ensure `UpdateURLParams: true` in props
- Check browser console for errors
- Verify URL structure is valid

### Custom range validation errors

- Start time must be before end time
- Both start and end must be selected
- Times must be valid dates

## Related Components

- [CPUChart](./cpu_chart.templ) - CPU utilization chart
- [MemoryChart](./memory_chart.templ) - Memory usage chart
- [LineChart](./line_chart.templ) - Generic line chart
- [GaugeChart](./gauge_chart.templ) - Gauge visualization

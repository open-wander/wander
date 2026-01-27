# LineChart Component

Time-series line chart component for Wander UI using D3.js.

## Features

- **Time-series visualization** - X axis with date/time data
- **Multiple series support** - Display multiple data series on the same chart
- **Interactive tooltips** - Hover over data points for detailed information
- **Gradient fills** - Optional gradient fill under line paths
- **Legend** - Automatic legend generation for multiple series
- **Grid lines** - Horizontal and vertical grid lines for readability
- **Smooth interpolation** - Multiple curve types for line smoothing
- **Responsive** - Auto-resizing with ResizeObserver
- **Auto-refresh** - Optional periodic data refresh
- **Dark mode** - Automatic theme support
- **Loading states** - Built-in loading and error states

## Files

- `/ui-htmx/static/js/line-chart.js` - D3-based LineChart class
- `/ui-htmx/templates/components/line_chart.templ` - Templ wrapper component
- `/ui-htmx/templates/components/line_chart_templ.go` - Generated Go code

## Usage

### Basic Example

```go
@LineChart(LineChartProps{
    ID:      "revenue-chart",
    DataURL: "/api/charts/revenue",
})
```

### Full Featured Example

```go
@LineChart(LineChartProps{
    ID:              "metrics-chart",
    DataURL:         "/api/charts/metrics",
    RefreshInterval: 30,              // Refresh every 30 seconds
    Height:          "96",            // Tailwind height class
    ShowLegend:      true,            // Show legend
    ShowGrid:        true,            // Show grid lines
    ShowGradient:    true,            // Gradient fill under lines
    CurveType:       "curveMonotoneX", // Smooth curve interpolation
    XAxisLabel:      "Date",
    YAxisLabel:      "Revenue ($)",
    MarginLeft:      60,              // Extra margin for Y axis labels
    MarginBottom:    40,              // Extra margin for X axis labels
})
```

### With HTMX Lazy Loading

```html
<div hx-get="/api/dashboard/revenue-section"
     hx-trigger="intersect once"
     hx-target="#revenue-container">
    <div id="revenue-container">
        @LineChart(LineChartProps{
            ID:      "revenue-chart",
            DataURL: "/api/charts/revenue",
        })
    </div>
</div>
```

## Data Format

The component expects JSON data from the `DataURL` endpoint in the following format:

```json
{
  "series": [
    {
      "name": "Revenue",
      "color": "#3b82f6",  // Optional, auto-assigned if not provided
      "data": [
        { "date": "2024-01-01", "value": 1000 },
        { "date": "2024-01-02", "value": 1200 },
        { "date": "2024-01-03", "value": 1100 },
        { "date": "2024-01-04", "value": 1400 }
      ]
    },
    {
      "name": "Expenses",
      "color": "#ef4444",
      "data": [
        { "date": "2024-01-01", "value": 800 },
        { "date": "2024-01-02", "value": 900 },
        { "date": "2024-01-03", "value": 850 },
        { "date": "2024-01-04", "value": 950 }
      ]
    }
  ]
}
```

### Data Requirements

- Each series must have a `name` and `data` array
- Data points must have `date` (ISO 8601 string or valid date format) and `value` (number)
- Data will be automatically sorted by date
- Invalid data points are filtered out
- Colors are auto-assigned from theme palette if not specified

## Props Reference

### LineChartProps

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `ID` | string | *required* | Unique identifier for the chart container |
| `DataURL` | string | *required* | URL to fetch chart data from |
| `RefreshInterval` | int | 0 | Auto-refresh interval in seconds (0 = disabled) |
| `Width` | string | "full" | Tailwind width class |
| `Height` | string | "96" | Tailwind height class |
| `Class` | string | "" | Additional CSS classes |
| `ShowLegend` | bool | true | Display legend for multiple series |
| `ShowGrid` | bool | true | Display grid lines |
| `ShowGradient` | bool | true | Gradient fill under lines |
| `CurveType` | string | "curveMonotoneX" | D3 curve interpolation type |
| `XAxisLabel` | string | "" | Label for X axis |
| `YAxisLabel` | string | "" | Label for Y axis |
| `MarginLeft` | int | 50 | Left margin for Y axis |
| `MarginBottom` | int | 40 | Bottom margin for X axis |

## Curve Types

The `CurveType` prop controls line interpolation:

- `curveLinear` - Straight lines between points (no smoothing)
- `curveMonotoneX` - Smooth curves that preserve monotonicity (default)
- `curveCatmullRom` - Smooth Catmull-Rom spline
- `curveNatural` - Natural cubic spline
- `curveStep` - Step function
- `curveBasis` - B-spline

## Backend Integration

### Go Handler Example

```go
type ChartDataPoint struct {
    Date  string  `json:"date"`
    Value float64 `json:"value"`
}

type ChartSeries struct {
    Name  string           `json:"name"`
    Color string           `json:"color,omitempty"`
    Data  []ChartDataPoint `json:"data"`
}

type ChartData struct {
    Series []ChartSeries `json:"series"`
}

func (h *Handler) GetRevenueChart(c echo.Context) error {
    data := ChartData{
        Series: []ChartSeries{
            {
                Name:  "Revenue",
                Color: "#3b82f6",
                Data: []ChartDataPoint{
                    {Date: "2024-01-01", Value: 1000},
                    {Date: "2024-01-02", Value: 1200},
                    {Date: "2024-01-03", Value: 1100},
                },
            },
        },
    }
    return c.JSON(http.StatusOK, data)
}
```

## JavaScript API

You can also use the LineChart class directly in JavaScript:

```javascript
import { LineChart } from '/static/js/line-chart.js';

const chart = new LineChart('chart-container', {
    showLegend: true,
    showGrid: true,
    showGradient: true,
    curveType: 'curveMonotoneX',
    marginLeft: 50
});

// Render with data
chart.render({
    series: [
        {
            name: "Series 1",
            data: [
                { date: "2024-01-01", value: 100 },
                { date: "2024-01-02", value: 150 }
            ]
        }
    ]
});

// Update with new data
chart.update(newData);

// Destroy when done
chart.destroy();
```

### Async Loading from URL

```javascript
import { renderLineChartFromUrl } from '/static/js/line-chart.js';

const chart = await renderLineChartFromUrl(
    'chart-container',
    '/api/charts/revenue',
    { showLegend: true }
);
```

## Styling

The component uses theme-aware colors from `chart-utils.js`:

- Automatically adapts to light/dark mode
- Uses colorblind-friendly palette for multiple series
- Customizable through Tailwind classes
- Grid and axis colors match theme

## Performance

- **Debounced resize** - 250ms debounce on window resize
- **Efficient updates** - Only re-renders when data changes
- **Auto cleanup** - ResizeObserver and timers cleaned up on destroy
- **Lazy loading** - Works with HTMX lazy loading patterns

## Dependencies

- D3.js v7+ (must be loaded before line-chart.js)
- chart-utils.js (for chart initialization and utilities)
- HTMX (for auto-initialization after swaps)
- Alpine.js (for component state management)

## Browser Support

- Modern browsers with ES6+ support
- ResizeObserver API
- ES modules
- Fetch API

## Troubleshooting

### Chart not rendering

- Check that D3.js is loaded before line-chart.js
- Verify the DataURL endpoint returns valid JSON
- Check browser console for errors
- Ensure container has dimensions (width/height)

### Auto-refresh not working

- Verify RefreshInterval is set (in seconds)
- Check that DataURL endpoint is accessible
- Monitor network tab for refresh requests

### Tooltips not showing

- Ensure the chart has data points
- Check that cursor is hovering over data points (circles)
- Verify no z-index conflicts with tooltip

### Colors not matching theme

- Check that dark mode class is applied to document root
- Verify getChartColors() is returning correct values
- Ensure theme colors are defined in chart-utils.js

## Examples

See the PRD document (`tasks/prd-ui-modernization-htmx.md`) for complete integration examples with the Wander application.

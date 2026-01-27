# ANSI Color Support for Log Viewer

## Overview

This implementation adds ANSI escape code parsing to the log viewer, enabling terminal color rendering in the browser. Log output from applications using ANSI color codes (common in CLI tools, Docker containers, and build systems) will now display with proper colors and text styles.

## Components

### 1. ANSI Parser (`ansi.js`)

A high-performance JavaScript parser that converts ANSI escape sequences to HTML with CSS classes.

**Features:**
- Standard colors (30-37, 40-47)
- Bright colors (90-97, 100-107)
- Text styles (bold, dim, italic, underline)
- Nested style handling
- Graceful handling of malformed sequences
- XSS protection via HTML escaping
- Static `strip()` method for removing ANSI codes

**Usage:**
```javascript
const parser = new AnsiParser();
const html = parser.parse("\x1b[31mError:\x1b[0m Something went wrong");
// Returns: '<span class="ansi-fg-red">Error:</span> Something went wrong'
```

**API:**
- `parse(text)` - Parse ANSI codes and return HTML
- `AnsiParser.strip(text)` - Static method to strip all ANSI codes from text

### 2. ANSI CSS Classes (`input.css`)

Added terminal color classes following VS Code's default dark theme colors:

**Text Styles:**
- `.ansi-bold` - Bold text (weight: 700)
- `.ansi-dim` - Dimmed text (opacity: 0.6)
- `.ansi-italic` - Italic text
- `.ansi-underline` - Underlined text

**Foreground Colors (30-37):**
- `.ansi-fg-black`, `.ansi-fg-red`, `.ansi-fg-green`, etc.

**Bright Foreground Colors (90-97):**
- `.ansi-fg-bright-black`, `.ansi-fg-bright-red`, etc.

**Background Colors (40-47, 100-107):**
- `.ansi-bg-red`, `.ansi-bg-bright-green`, etc.

### 3. Log Viewer Integration (`alpine-patterns.js`)

Modified the `logViewer` Alpine.js component to:
- Initialize ANSI parser in `init()` method
- Parse log messages in `handleLogLine()` method
- Store both parsed HTML and raw message for downloading
- Strip ANSI codes when downloading logs
- Gracefully fallback to HTML escaping if parser unavailable

### 4. Template Updates (`logs.templ`)

Updated the logs template to:
- Call `init()` on component initialization
- Use `x-html` instead of `x-text` for rendering parsed HTML

### 5. Layout Integration (`base.templ`)

Added `ansi.js` script to the base layout, loaded after Alpine patterns.

## Supported ANSI Codes

| Code | Description | CSS Class |
|------|-------------|-----------|
| 0 | Reset all | - |
| 1 | Bold | `ansi-bold` |
| 2 | Dim | `ansi-dim` |
| 3 | Italic | `ansi-italic` |
| 4 | Underline | `ansi-underline` |
| 30-37 | Foreground colors | `ansi-fg-{color}` |
| 40-47 | Background colors | `ansi-bg-{color}` |
| 90-97 | Bright foreground | `ansi-fg-bright-{color}` |
| 100-107 | Bright background | `ansi-bg-bright-{color}` |

## Examples

### Docker Logs
```
\x1b[32m✓\x1b[0m Container started successfully
\x1b[31m✗\x1b[0m Failed to connect to database
```

### Build Output
```
\x1b[1;32m[PASS]\x1b[0m test_user_login
\x1b[1;31m[FAIL]\x1b[0m test_payment_processing
```

### Application Logs
```
\x1b[36mINFO\x1b[0m Server listening on port 3000
\x1b[33mWARN\x1b[0m API rate limit approaching
\x1b[31mERROR\x1b[0m Database connection lost
```

## Testing

A test page is available at `/static/test-ansi.html` that demonstrates:
- Basic colors (30-37)
- Bright colors (90-97)
- Background colors (40-47, 100-107)
- Text styles (bold, dim, italic, underline)
- Combined styles
- Real-world log examples
- XSS protection

## Performance

The parser is designed for high-performance streaming:
- Regex-based parsing with minimal memory allocation
- Single-pass processing
- Efficient HTML escaping
- No external dependencies

## Security

**XSS Protection:**
All text content is HTML-escaped before processing ANSI codes, preventing script injection through log messages.

**Example:**
```javascript
parser.parse('\x1b[31m<script>alert("XSS")</script>\x1b[0m')
// Returns: '<span class="ansi-fg-red">&lt;script&gt;alert("XSS")&lt;/script&gt;</span>'
```

## Browser Compatibility

Works in all modern browsers that support:
- ES6 classes
- Regular expressions
- String.replace with callbacks
- Set data structure

## Future Enhancements

Potential improvements:
- 256-color support (ESC[38;5;{n}m)
- True color support (ESC[38;2;{r};{g};{b}m)
- Reverse video (code 7)
- Hidden text (code 8)
- Strikethrough (code 9)
- Custom color themes

## Files Modified

1. `/ui-htmx/static/js/ansi.js` - ANSI parser implementation
2. `/ui-htmx/static/js/alpine-patterns.js` - Log viewer integration
3. `/ui-htmx/static/css/input.css` - ANSI color CSS classes
4. `/ui-htmx/templates/layouts/base.templ` - Script tag addition
5. `/ui-htmx/templates/partials/allocationspartials/logs.templ` - Template updates

## Patterns Discovered

1. **HTML Escaping First**: Always escape HTML before processing ANSI codes to prevent XSS
2. **Keep Raw Message**: Store both parsed HTML and raw message for download functionality
3. **Graceful Degradation**: Fallback to plain text escaping if ANSI parser unavailable
4. **Single Parser Instance**: Reuse parser instance for better performance
5. **CSS Over Inline Styles**: Use CSS classes instead of inline styles for better performance and theming

## Known Limitations

1. Does not support 256-color mode or true color
2. Does not support cursor movement codes (not applicable for log display)
3. Does not support reverse video or other advanced formatting
4. Background colors may have readability issues on certain backgrounds

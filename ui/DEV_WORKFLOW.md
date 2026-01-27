# UI Development Workflow

Quick reference for developing the HTMX/Templ/Tailwind UI.

## Quick Start

### One-Command Development

From project root:
```bash
make ui-dev
```

This starts the hot-reload development server. The server will automatically:
- Watch for changes in `.go` and `.templ` files
- Generate Templ templates
- Rebuild the Go server
- Restart the server

Access the UI at: `http://localhost:3000/ui/`

### CSS Development

In a separate terminal, run:
```bash
make ui-css-watch
```

This watches Tailwind CSS files and rebuilds on changes.

## Full Development Setup

### Terminal 1: Hot-Reload Server
```bash
make ui-dev
```
Watches: `*.go`, `*.templ`
Output: `http://localhost:3000/ui/`

### Terminal 2: CSS Watcher
```bash
make ui-css-watch
```
Watches: `*.css`, template files for Tailwind classes
Output: `ui-htmx/static/css/output.css`

## Configuration Files

- **`.air.toml`**: Air hot-reload configuration
- **`cmd/dev/main.go`**: Development server entry point
- **`tailwind.config.js`**: Tailwind CSS configuration

## Common Tasks

### Generate Templ Templates (one-time)
```bash
make ui-generate
```

### Build CSS (one-time)
```bash
make ui-css-dev
```

### Format Templ Files
```bash
make ui-fmt
```

### Run Tests
```bash
cd ui-htmx
go test ./...
```

## Server Configuration

Default port: `3000`
Default log level: `info`

To change:
```bash
cd ui-htmx
go run cmd/dev/main.go -port 8080 -log-level debug
```

Or edit `.air.toml` to pass custom flags:
```toml
[build]
  args_bin = ["-port", "8080", "-log-level", "debug"]
```

## What Gets Rebuilt

| File Type | Triggers | Actions |
|-----------|----------|---------|
| `*.templ` | Air | Generate templates, rebuild Go server, restart |
| `*.go` | Air | Rebuild Go server, restart |
| `*.css`, `tailwind.config.js` | Tailwind | Rebuild CSS |
| Template files (for classes) | Tailwind | Rebuild CSS (if watch running) |

## Troubleshooting

### Air not rebuilding
- Check `ui-htmx/build-errors.log` for build errors
- Ensure templ CLI is installed: `go install github.com/a-h/templ/cmd/templ@latest`

### CSS not updating
- Ensure Tailwind is watching: `make ui-css-watch`
- Check `tailwind.config.js` content paths include your files
- Clear browser cache

### Port already in use
Change port in `.air.toml`:
```toml
[build]
  args_bin = ["-port", "3001"]
```

### Server not starting
Check that handlers and router compile:
```bash
cd ui-htmx
go build ./handlers/...
```

## File Structure

```
ui-htmx/
├── .air.toml              # Air configuration
├── cmd/
│   └── dev/
│       └── main.go        # Dev server entry point
├── handlers/              # HTTP handlers
├── templates/             # Templ templates
│   ├── layouts/
│   ├── components/
│   ├── pages/
│   └── partials/
├── static/
│   ├── css/
│   │   ├── input.css      # Tailwind input
│   │   └── output.css     # Generated CSS (gitignored)
│   └── js/
└── tmp/                   # Air build artifacts (gitignored)
    └── ui-htmx-dev        # Built dev server binary
```

## Next Steps

After making changes:
1. Verify in browser at `http://localhost:3000/ui/`
2. Check browser console for errors
3. Check server logs for backend errors
4. Run tests: `go test ./...`
5. Format code: `make ui-fmt`

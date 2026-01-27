# Tailwind CSS Build System

This document describes the Tailwind CSS build pipeline integration for the Wander UI-HTMX project.

## Overview

The Tailwind CSS build system uses the standalone Tailwind CLI (no Node.js required for the CLI itself), with npm only used for installing Tailwind plugins (`@tailwindcss/forms` and `@tailwindcss/typography`).

## Build Pipeline

### Input
- **Source**: `ui-htmx/static/css/input.css`
- **Config**: `ui-htmx/tailwind.config.js`
- **Content**: Scans for classes in:
  - `./templates/**/*.templ` - Templ template files
  - `./templates/**/*.go` - Generated Templ Go files
  - `./handlers/**/*.go` - Handler files (may contain class references)
  - `./static/js/**/*.js` - JavaScript files (Alpine.js patterns)

### Output
- **Production**: `ui-htmx/static/css/output.css` (minified, ~17KB)
- **Development**: `ui-htmx/static/css/output.css` (not minified, ~23KB)

### Tailwind CLI
- **Location**: `ui-htmx/bin/tailwindcss`
- **Version**: Latest standalone CLI (auto-detected for platform)
- **Installation**: Via `ui-htmx/install-tailwind.sh`

## Build Targets

### Root Makefile Targets (run from project root)

```bash
make ui-setup         # Setup UI dependencies (Tailwind CLI + npm packages)
make ui-css           # Build production CSS (minified)
make ui-css-dev       # Build development CSS (not minified)
make ui-css-watch     # Watch and rebuild CSS on changes
```

### ui-htmx/Makefile Targets (run from ui-htmx directory)

```bash
make setup            # Complete setup: install CLI, check deps, build CSS
make install-tailwind # Install Tailwind CSS CLI
make build-css        # Build production CSS (minified)
make build-css-dev    # Build development CSS (not minified)
make watch-css        # Watch and rebuild on changes
make clean            # Remove generated CSS
make verify-config    # Verify Tailwind config is valid
make check-deps       # Check npm dependencies
```

## Configuration

### Content Paths

The `tailwind.config.js` is configured to scan the following paths for class usage:

```javascript
content: [
  "./templates/**/*.templ",  // Templ template source files
  "./templates/**/*.go",     // Generated Templ Go files
  "./handlers/**/*.go",      // HTTP handlers
  "./static/js/**/*.js",     // JavaScript (Alpine.js patterns)
]
```

### Plugins

Two official Tailwind plugins are installed via npm:

- `@tailwindcss/forms` - Better default styles for form inputs
- `@tailwindcss/typography` - Typography plugin for prose content

### Custom Theme

The config extends Tailwind's default theme with:

- **Custom Colors**: primary, success, warning, error, info, neutral (with 50-950 shades)
- **Custom Fonts**: Inter (sans), JetBrains Mono (mono)
- **Custom Spacing**: 128, 144
- **Custom Animations**: spin-slow, pulse-slow
- **Dark Mode**: Class-based (`class` strategy)

## Custom Styles

The `input.css` file includes custom styles in three layers:

### @layer base
- Focus styles (ring-2 ring-primary-500)
- Smooth scrolling
- Body and heading defaults
- Link styles with dark mode support
- Code and pre block styles

### @layer components
- Button variants (btn, btn-primary, btn-secondary, etc.)
- Card component
- Badge variants
- Table styles
- Form input styles
- Alert variants

### @layer utilities
- Custom scrollbar styles
- HTMX loading indicators
- Skeleton loaders

## How It Works

1. **Content Scanning**: Tailwind scans all files in the content paths for class usage
2. **Tree Shaking**: Only used classes are included in the output
3. **Layers**: Base, components, and utilities are processed in order
4. **Minification**: Production builds are minified and optimized
5. **Watch Mode**: In development, rebuilds automatically on file changes

## Testing

Automated tests verify the build pipeline:

- `TestTailwindOutputExists` - Verifies output.css is generated
- `TestTailwindOutputNotEmpty` - Verifies output has content
- `TestTailwindBaseStyles` - Verifies Tailwind base styles are present
- `TestTailwindCustomClasses` - Verifies custom utility classes
- `TestTailwindConfigValid` - Verifies config file is properly configured
- `TestInputCSSExists` - Verifies input.css has required directives

Run tests:
```bash
cd ui-htmx/tests
go test -v -run TestTailwind
```

## Development Workflow

### First Time Setup

```bash
# From project root
make ui-setup

# This will:
# 1. Download Tailwind CLI binary
# 2. Install npm packages (@tailwindcss/forms, @tailwindcss/typography)
# 3. Build initial CSS
```

### Daily Development

```bash
# Terminal 1: Watch Templ templates
make ui-watch

# Terminal 2: Watch CSS
make ui-css-watch

# Terminal 3: Run server
make dev
```

### Production Build

```bash
# Build minified CSS
make ui-css

# Or from ui-htmx directory
cd ui-htmx && make build-css
```

## Generated Files (.gitignore)

The following files are generated and should not be committed:

- `ui-htmx/static/css/output.css` - Generated CSS output
- `ui-htmx/bin/tailwindcss` - Tailwind CLI binary
- `ui-htmx/node_modules/` - npm packages

## Patterns Discovered

### Tailwind Purging
- Tailwind only includes classes that are actually used in content files
- Custom component classes (like `.btn-primary`) are only included if used in templates
- The HTMX indicator classes are always included because they're in the utilities layer
- Comments are stripped in production builds

### Plugin Requirements
- Plugins MUST be installed via npm even though we use the standalone CLI
- The standalone CLI reads `node_modules/` for plugin resolution
- Without npm install, plugins in config will cause build errors

### Content Path Best Practices
- Include both `.templ` and `*_templ.go` files to catch all class usage
- Include handler files if they contain class strings
- Include JavaScript files for Alpine.js patterns that may reference classes

### Build Performance
- Initial build: ~200ms
- Watch mode rebuild: <100ms (incremental)
- Output size: ~17KB minified (with no templates using classes yet)
- Full framework with all components used: typically 20-40KB minified

## Integration Points

### With Templ
- Templ templates use Tailwind classes via string attributes
- Generated Go code contains class strings that Tailwind scans
- Both `.templ` and generated `.go` files are in content paths

### With HTMX
- Custom `.htmx-indicator` class for loading states
- `.htmx-request` class toggles indicator visibility
- Skeleton loaders for HTMX partial loading states

### With Alpine.js
- Alpine directives may contain dynamic class bindings
- JavaScript files are in content paths to catch these
- Example: `x-bind:class="{ 'bg-primary-500': isActive }"`

## Troubleshooting

### Build fails with plugin errors
```bash
# Reinstall npm packages
cd ui-htmx && npm install
```

### CSS not rebuilding in watch mode
```bash
# Kill watch process and restart
make ui-css-watch
```

### Classes not appearing in output
- Ensure the file using the class is in a content path
- Check for typos in class names
- Verify content path globs match your files
- Run `make verify-config` to check config validity

### Output CSS is too small
- Tailwind purges unused classes
- Add classes to templates to see them in output
- Custom component classes only appear if used

## Future Enhancements

1. **PostCSS Integration**: For autoprefixer and additional plugins
2. **CSS Modules**: Scoped styles for complex components
3. **Design Tokens**: Shared tokens between CSS and Go
4. **Critical CSS**: Extract above-the-fold CSS for faster loading
5. **Theme Switching**: Runtime theme selection (light/dark)

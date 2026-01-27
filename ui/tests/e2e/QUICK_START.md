# Quick Start Guide

## First Time Setup

```bash
# Navigate to e2e directory
cd ui-htmx/tests/e2e

# Install npm dependencies
npm install

# Install Playwright browsers
npx playwright install
```

## Running Tests

### Option 1: Via Go (Recommended)

This automatically starts the server and runs tests:

```bash
cd ui-htmx/tests/e2e
go test -v
```

### Option 2: Manual Server + Playwright

```bash
# Terminal 1: Start server
cd ui-htmx
go run cmd/dev/main.go -port 3001

# Terminal 2: Run tests
cd ui-htmx/tests/e2e
npm test
```

## Common Commands

```bash
# Run all tests
npm test

# Run in UI mode (interactive)
npm run test:ui

# Run in debug mode
npm run test:debug

# Run specific browser
npm run test:chromium
npm run test:firefox
npm run test:webkit

# Run with browser visible
npm run test:headed

# View test report
npm run report

# Generate test code
npm run codegen
```

## Writing Your First Test

Create a new file in `specs/`:

```typescript
// specs/my-feature.spec.ts
import { test, expect } from '@playwright/test';

test('my feature works', async ({ page }) => {
  await page.goto('/ui/my-feature');
  await expect(page.locator('h1')).toContainText('My Feature');
});
```

## Debugging

1. Run with debugger: `npm run test:debug`
2. Add `await page.pause()` in your test
3. Use Playwright Inspector to step through
4. Check `test-results/` for screenshots and traces

## Need Help?

See full documentation in [README.md](./README.md)

# End-to-End Testing with Playwright

This directory contains end-to-end tests for the Wander UI using Playwright. The tests run against a real Go HTTP server started by the test setup.

## Architecture

The e2e test setup consists of three main components:

1. **Go Test Server** (`setup.go`) - Starts the UI server using the same handlers as the dev server
2. **Playwright Tests** (`specs/*.spec.ts`) - Browser-based tests that interact with the UI
3. **Test Runner** - Coordinates between Go and Playwright

## Directory Structure

```
tests/e2e/
├── setup.go              # Go test server setup
├── setup_test.go         # Go integration with Playwright
├── package.json          # Playwright dependencies
├── playwright.config.ts  # Playwright configuration
├── specs/               # Test specifications
│   └── *.spec.ts
├── test-results/        # Test artifacts (screenshots, traces)
└── README.md           # This file
```

## Setup

### 1. Install Node Dependencies

```bash
cd ui-htmx/tests/e2e
npm install
npx playwright install
```

### 2. Install Playwright Browsers

Playwright needs browser binaries:

```bash
npx playwright install chromium firefox webkit
```

## Running Tests

### Run All Tests (via Go)

The recommended way is to run tests through Go, which handles server lifecycle:

```bash
cd ui-htmx/tests/e2e
go test -v
```

### Run Playwright Tests Directly

You can also run Playwright tests directly if the server is already running:

```bash
# Terminal 1: Start dev server
cd ui-htmx
go run cmd/dev/main.go -port 3001

# Terminal 2: Run Playwright tests
cd ui-htmx/tests/e2e
npm test
```

### Run Specific Browser

```bash
npm run test:chromium   # Chrome/Chromium only
npm run test:firefox    # Firefox only
npm run test:webkit     # WebKit/Safari only
npm run test:mobile     # Mobile Chrome
```

### Run in Headed Mode

See the browser while tests run:

```bash
npm run test:headed
```

### Run in UI Mode

Interactive test runner with time-travel debugging:

```bash
npm run test:ui
```

### Debug Mode

Run tests with Playwright Inspector for step-by-step debugging:

```bash
npm run test:debug
```

## Writing Tests

### Create a New Test

Create a new file in `specs/` directory:

```typescript
// specs/example.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Feature Name', () => {
  test('should do something', async ({ page }) => {
    // Navigate to page
    await page.goto('/ui/');

    // Interact with elements
    await page.click('text=Jobs');

    // Assert expectations
    await expect(page).toHaveURL(/\/ui\/jobs/);
    await expect(page.locator('h1')).toContainText('Jobs');
  });
});
```

### Test Structure

Follow the Arrange-Act-Assert pattern:

```typescript
test('descriptive test name', async ({ page }) => {
  // ARRANGE: Set up test state
  await page.goto('/ui/jobs');

  // ACT: Perform action
  await page.click('[data-testid="create-job"]');
  await page.fill('[name="job-name"]', 'test-job');
  await page.click('button:has-text("Submit")');

  // ASSERT: Verify outcome
  await expect(page.locator('.success-message')).toBeVisible();
  await expect(page).toHaveURL(/\/ui\/jobs\/test-job/);
});
```

### Best Practices

1. **Use data-testid attributes** for stable selectors
2. **Wait for navigation** after clicks that navigate
3. **Test user flows**, not implementation details
4. **Use page objects** for complex pages
5. **Keep tests independent** - each test should be able to run alone
6. **Clean up state** if tests modify data

### Debugging Tests

#### Visual Debugging

```bash
npm run test:debug
```

This opens Playwright Inspector where you can:
- Step through test actions
- Pause and resume execution
- Inspect DOM state at each step
- Edit and re-run tests

#### Trace Viewer

Tests automatically record traces on failure. View them:

```bash
npm run report
```

This shows:
- Screenshots at each step
- Network requests
- Console logs
- DOM snapshots

#### Manual Server + Headed Browser

For interactive development:

```bash
# Terminal 1: Start server
cd ui-htmx
go run cmd/dev/main.go -port 3001

# Terminal 2: Run in headed mode
cd ui-htmx/tests/e2e
npm run test:headed
```

## Test Server Configuration

The Go test server can be configured via `TestServerConfig`:

```go
srv, cleanup := e2e.StartTestServer(e2e.TestServerConfig{
    Port: 3001,              // Specific port (0 = random)
    LogLevel: "debug",       // Log verbosity
    Logger: customLogger,    // Custom logger
})
defer cleanup()
```

### Server Features

- Uses same handlers as dev server
- Supports all UI routes
- Serves static assets
- Handles HTMX requests
- WebSocket support for logs/exec

## Continuous Integration

Tests are designed to run in CI with:
- Retry on flaky failures (2 retries in CI)
- Single worker to avoid port conflicts
- Artifacts saved on failure
- JSON report for test results

### CI Configuration Example

```yaml
- name: E2E Tests
  run: |
    cd ui-htmx/tests/e2e
    npm ci
    npx playwright install --with-deps
    go test -v
```

## Common Patterns

### Testing HTMX Interactions

```typescript
test('HTMX partial page update', async ({ page }) => {
  await page.goto('/ui/jobs');

  // Click link that triggers HTMX request
  await page.click('[hx-get="/ui/jobs/detail"]');

  // Wait for HTMX to complete
  await page.waitForSelector('[hx-swap-oob]');

  // Verify content updated
  await expect(page.locator('#main-content')).toContainText('Job Detail');
});
```

### Testing Forms

```typescript
test('form submission', async ({ page }) => {
  await page.goto('/ui/jobs/new');

  // Fill form
  await page.fill('[name="name"]', 'my-job');
  await page.selectOption('[name="type"]', 'service');

  // Submit
  await page.click('button[type="submit"]');

  // Wait for success
  await expect(page.locator('.alert-success')).toBeVisible();
});
```

### Testing Navigation

```typescript
test('navigation preserves state', async ({ page }) => {
  await page.goto('/ui/');

  // Navigate through app
  await page.click('text=Jobs');
  await page.click('text=Create Job');

  // Use back button
  await page.goBack();

  // Verify state
  await expect(page).toHaveURL(/\/ui\/jobs$/);
});
```

## Troubleshooting

### Tests Timeout

- Increase timeout in `playwright.config.ts`
- Check if server is starting correctly
- Look at server logs for errors

### Flaky Tests

- Use `page.waitForLoadState('networkidle')` before assertions
- Use explicit waits instead of arbitrary sleeps
- Check for race conditions in HTMX swaps

### Port Already in Use

- Change port in `TestServerConfig`
- Use port 0 for random available port
- Kill processes: `lsof -ti:3001 | xargs kill`

### Browser Not Found

```bash
npx playwright install
```

## Resources

- [Playwright Documentation](https://playwright.dev/)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
- [HTMX Testing Guide](https://htmx.org/docs/#testing)
- [Go net/http Testing](https://golang.org/pkg/net/http/httptest/)

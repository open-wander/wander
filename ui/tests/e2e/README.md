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

## Visual Regression Testing

Visual regression testing captures screenshots of pages and components, comparing them to baseline images to detect unintended visual changes.

### What is Visual Regression Testing?

Visual regression testing helps catch:
- Unintended CSS changes
- Layout breakages
- Component rendering issues
- Responsive design problems
- Cross-browser rendering differences

### Running Visual Regression Tests

Visual tests are tagged with `@visual` and run only on Chromium to ensure consistent rendering.

```bash
# Run all visual regression tests
npm run test:visual

# Run with visible browser
npm run test:visual:headed

# Update baseline screenshots (after reviewing changes)
npm run test:visual:update

# Or use the Playwright flag
npm run test:update-snapshots
```

### How It Works

1. **Baseline Creation**: First run creates baseline screenshots in `__screenshots__/` directory
2. **Comparison**: Subsequent runs compare against baselines
3. **Threshold**: 20% pixel difference allowed (handles minor rendering differences)
4. **Max Diff Pixels**: Up to 100 pixels can differ (for anti-aliasing, fonts)
5. **CI Mode**: Stricter thresholds (10%, 50px) in CI environment

### Screenshot Coverage

Visual regression tests cover:

**Page-Level Screenshots:**
- Jobs list page (empty and populated states)
- Job detail page (all tabs)
- Allocations list page
- Allocation detail page (overview, logs, exec terminal)
- Clients list page
- Client detail page (all tabs)
- Servers list page
- Server detail page

**Component-Level Screenshots:**
- Navigation header
- Job table rows
- Allocation table rows
- Table component

**Responsive Screenshots:**
- Mobile viewport (375x667)
- Tablet viewport (768x1024)
- Desktop viewport (1280x720)

### Configuration

Visual regression settings in `playwright.config.ts`:

```typescript
expect: {
  toHaveScreenshot: {
    threshold: 0.2,         // 20% pixel difference allowed
    maxDiffPixels: 100,     // Max 100 pixels can differ
    animations: 'disabled', // Disable for consistent captures
  },
},
snapshotDir: './__screenshots__',
```

### Updating Baselines

When you intentionally change UI:

1. **Review Changes**: Run tests to see what changed
   ```bash
   npm run test:visual
   ```

2. **View Diff Report**: Open test report to see visual diffs
   ```bash
   npm run report
   ```

3. **Update Baselines**: If changes are correct, update baselines
   ```bash
   npm run test:visual:update
   ```

4. **Commit New Baselines**: Commit updated screenshots to git
   ```bash
   git add __screenshots__/
   git commit -m "Update visual regression baselines"
   ```

### Best Practices

1. **Review Before Updating**: Always review diffs before updating baselines
2. **Commit Baselines**: Version baseline screenshots in git
3. **Run Locally First**: Test visual changes locally before CI
4. **One Viewport Per Test**: Use consistent viewport sizes
5. **Wait for Stability**: Use `networkidle` and timeouts to ensure page is ready
6. **Disable Animations**: Set `animations: 'disabled'` for consistent captures
7. **Test Components Too**: Capture individual components, not just full pages
8. **Document Changes**: Add context to commits when updating baselines

### Troubleshooting Visual Tests

#### Tests Fail with Minor Differences

```bash
# Increase threshold in playwright.config.ts
threshold: 0.3  # Allow 30% difference
```

#### Font Rendering Differences

Fonts may render differently across machines. Solutions:
- Use web fonts with `font-display: block`
- Increase `maxDiffPixels` threshold
- Run tests in Docker for consistency

#### Animation/Loading State Captured

```bash
# Add longer wait in test
await page.waitForLoadState('networkidle');
await page.waitForTimeout(1000);  # Wait for animations
```

#### Screenshots Don't Match in CI

CI uses stricter thresholds. To debug:
1. Download artifacts from CI
2. Compare with local screenshots
3. Run in Docker locally to match CI environment

#### HTMX Content Not Loaded

```bash
# Wait for HTMX swap to complete
await page.waitForSelector('[hx-swap-oob]');
await page.waitForLoadState('networkidle');
```

### CI Integration

Visual regression tests run in CI with stricter thresholds:

```yaml
- name: Visual Regression Tests
  run: |
    cd ui-htmx/tests/e2e
    npm ci
    npx playwright install chromium --with-deps
    npm run test:visual

- name: Upload Screenshot Diffs
  if: failure()
  uses: actions/upload-artifact@v3
  with:
    name: screenshot-diffs
    path: ui-htmx/tests/e2e/test-results/
```

### File Structure

```
tests/e2e/
├── __screenshots__/                    # Baseline screenshots (versioned)
│   └── visual-regression.spec.ts/
│       ├── jobs-list.png
│       ├── job-detail-overview.png
│       └── ...
├── test-results/                       # Test output (not versioned)
│   └── visual-regression-*.spec.ts/
│       ├── jobs-list-actual.png        # Current screenshot
│       ├── jobs-list-diff.png          # Difference highlight
│       └── jobs-list-expected.png      # Baseline copy
└── specs/
    └── visual-regression.spec.ts       # Visual test specs
```

### When to Update Baselines

Update baselines when:
- Intentionally changing UI design
- Updating component styles
- Changing layout or spacing
- Adding new UI features
- Fixing rendering bugs

Do NOT update baselines when:
- Tests fail unexpectedly
- You haven't reviewed the changes
- Changes are unintentional
- You're not sure what changed

## Resources

- [Playwright Documentation](https://playwright.dev/)
- [Playwright Best Practices](https://playwright.dev/docs/best-practices)
- [Playwright Visual Comparisons](https://playwright.dev/docs/test-snapshots)
- [HTMX Testing Guide](https://htmx.org/docs/#testing)
- [Go net/http Testing](https://golang.org/pkg/net/http/httptest/)

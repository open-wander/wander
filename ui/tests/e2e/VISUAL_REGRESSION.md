# Visual Regression Testing Guide

Quick reference for visual regression testing in the Wander UI project.

## Quick Start

### First Time Setup

```bash
# Install dependencies
cd ui-htmx/tests/e2e
npm install
npx playwright install chromium

# Create baseline screenshots
npm run test:visual:update
```

### Running Tests

```bash
# Run all visual regression tests
npm run test:visual

# Run with visible browser (watch tests run)
npm run test:visual:headed

# View test results and diffs
npm run report
```

### Updating Baselines

When you intentionally change UI:

```bash
# 1. Run tests to see what changed
npm run test:visual

# 2. Review diffs in the report
npm run report

# 3. Update baselines if changes are correct
npm run test:visual:update

# 4. Commit new baselines
git add __screenshots__/
git commit -m "Update visual regression baselines for [feature]"
```

## What Gets Tested

### Full Page Screenshots
- Jobs list page (with/without jobs)
- Job detail page (Overview, Definition, Allocations tabs)
- Allocations list page
- Allocation detail (Overview, Logs, Exec terminal)
- Clients list page
- Client detail page (Overview, Allocations)
- Servers list page
- Server detail page

### Component Screenshots
- Navigation header
- Job table rows
- Allocation table rows
- Table components

### Responsive Screenshots
- Mobile (375x667)
- Tablet (768x1024)
- Desktop (1280x720)

## Configuration

### Thresholds
- **Development**: 20% pixel difference, max 100 pixels
- **CI**: 10% pixel difference, max 50 pixels

### Settings in playwright.config.ts
```typescript
expect: {
  toHaveScreenshot: {
    threshold: 0.2,         // 20% difference allowed
    maxDiffPixels: 100,     // Max pixels that can differ
    animations: 'disabled', // Disable animations
  },
}
```

## Common Scenarios

### Scenario 1: Changing Component Styles

```bash
# 1. Make CSS changes
# 2. Run visual tests
npm run test:visual

# 3. Review diffs
npm run report

# 4. Update if correct
npm run test:visual:update

# 5. Commit
git add __screenshots__/
git commit -m "style: update button colors"
```

### Scenario 2: Adding New Page

```bash
# 1. Add page implementation
# 2. Add visual test in specs/visual-regression.spec.ts
# 3. Create baseline
npm run test:visual:update

# 4. Commit everything together
git add specs/visual-regression.spec.ts __screenshots__/
git commit -m "feat: add new dashboard page with visual tests"
```

### Scenario 3: Test Fails Unexpectedly

```bash
# 1. View the diff
npm run report

# 2. If change is unintended, fix the bug
# 3. If change is expected, update baseline
npm run test:visual:update

# 4. If diff is tiny (fonts, anti-aliasing), increase threshold
# Edit playwright.config.ts: threshold: 0.3
```

### Scenario 4: CI Failure

```bash
# 1. Download artifacts from CI
# 2. Compare screenshots
# 3. Fix issue locally
# 4. Run visual tests
npm run test:visual

# 5. Push fix
```

## Best Practices

### Do
- Review all diffs before updating baselines
- Commit baselines with the feature changes
- Run tests locally before pushing
- Add visual tests for new pages/components
- Use descriptive test names
- Test both populated and empty states
- Test responsive breakpoints

### Don't
- Update baselines blindly
- Skip reviewing diffs
- Commit without understanding changes
- Test with animations enabled
- Use random viewport sizes
- Forget to wait for page stability

## Troubleshooting

### Tests fail with small differences

**Problem**: Fonts or anti-aliasing differ slightly

**Solution**: Increase threshold
```typescript
threshold: 0.3,  // Allow 30% difference
maxDiffPixels: 200,
```

### Content not loaded in screenshot

**Problem**: HTMX content loading after screenshot

**Solution**: Add explicit waits
```typescript
await page.waitForLoadState('networkidle');
await page.waitForTimeout(500);
```

### Screenshots differ between machines

**Problem**: Font rendering differs

**Solution**:
1. Use web fonts with `font-display: block`
2. Run in Docker for consistency
3. Increase threshold

### Animation captured in screenshot

**Problem**: Loading spinners or transitions visible

**Solution**: Increase wait time
```typescript
await page.waitForTimeout(1000);
```

## File Structure

```
ui-htmx/tests/e2e/
├── __screenshots__/                      # Committed baselines
│   └── visual-regression.spec.ts/
│       ├── jobs-list.png                 # Baseline images
│       ├── job-detail-overview.png
│       └── ...
├── test-results/                         # Not committed
│   └── visual-regression-*.spec.ts/
│       ├── jobs-list-actual.png          # Current screenshot
│       ├── jobs-list-diff.png            # Highlighted differences
│       └── jobs-list-expected.png        # Copy of baseline
└── specs/
    └── visual-regression.spec.ts         # Test definitions
```

## Writing New Visual Tests

### Template

```typescript
test('Page name matches baseline @visual', async ({ page }) => {
  // 1. Navigate to page
  await page.goto('/ui/your-page');

  // 2. Wait for stability
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(500);

  // 3. Take screenshot
  await expect(page).toHaveScreenshot('your-page.png', {
    fullPage: true,
    animations: 'disabled',
    threshold: 0.2,
    maxDiffPixels: 100,
  });
});
```

### Component Screenshot

```typescript
test('Component matches baseline @visual', async ({ page }) => {
  await page.goto('/ui/page');

  // Select specific component
  const component = page.locator('[data-component]');

  await expect(component).toHaveScreenshot('component.png', {
    animations: 'disabled',
    threshold: 0.2,
    maxDiffPixels: 100,
  });
});
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Visual Regression Tests

on: [pull_request]

jobs:
  visual-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Node
        uses: actions/setup-node@v3
        with:
          node-version: '18'

      - name: Install dependencies
        run: |
          cd ui-htmx/tests/e2e
          npm ci
          npx playwright install chromium --with-deps

      - name: Run visual tests
        run: |
          cd ui-htmx/tests/e2e
          npm run test:visual

      - name: Upload diffs on failure
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: visual-diffs
          path: ui-htmx/tests/e2e/test-results/
```

## FAQ

### Q: Why only Chromium?
**A**: Visual tests run only on Chromium to avoid cross-browser rendering differences. Functional tests still run on all browsers.

### Q: Should I commit baselines?
**A**: Yes! Baseline screenshots should be committed to git so CI can compare against them.

### Q: What if fonts look different in CI?
**A**: Either use web fonts, run tests in Docker, or increase the threshold to account for font rendering differences.

### Q: How do I test dark mode?
**A**: Add a test that sets dark mode class/preference before taking the screenshot.

### Q: Can I test specific sections?
**A**: Yes! Use locators to screenshot specific components instead of full pages.

### Q: How often should I update baselines?
**A**: Only when you intentionally change UI. Never update blindly.

## Additional Resources

- [Playwright Visual Comparisons Docs](https://playwright.dev/docs/test-snapshots)
- [Main E2E Testing README](README.md)
- [Test Specifications](specs/)

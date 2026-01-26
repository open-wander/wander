# Page Objects

This directory contains Page Object Models (POM) for Playwright e2e tests. Page objects provide a clean, maintainable abstraction layer over the UI, making tests easier to write and maintain.

## Architecture

```
pages/
├── base.page.ts         # Base page with common functionality
├── jobs.page.ts         # Jobs list and detail pages
├── allocations.page.ts  # Allocations list and detail pages
├── clients.page.ts      # Clients list and detail pages
└── index.ts             # Exports all page objects
```

## BasePage

The `BasePage` class provides common functionality used across all page objects:

### Navigation
- `goto(path)` - Navigate to a specific path
- `waitForLoad()` - Wait for page to be fully loaded
- `clickLink(text)` - Navigate by clicking a link
- `goBack()` / `goForward()` - Browser history navigation
- `reload()` - Reload the page

### HTMX Helpers
- `waitForHTMXSwap(targetSelector?)` - Wait for HTMX content swap
- `triggerHTMX(selector)` - Trigger HTMX element without clicking
- `waitForHTMXRequest()` - Wait for HTMX request to complete
- `clickHTMX(selector, targetSelector?)` - Click HTMX element and wait for swap

### Element Interaction
- `fill(selector, value)` - Fill a form field
- `clickButton(text)` - Click a button by text
- `select(selector, value)` - Select dropdown option
- `getText(selector)` - Get element text content
- `getElements(selector)` - Get all matching elements
- `isVisible(selector)` - Check if element is visible
- `hasClass(selector, className)` - Check if element has a class

### Assertions
- `verifyTitle(expectedText)` - Verify page title
- `verifyURL(expectedPath)` - Verify URL contains path

## Usage Examples

### Basic Example

```typescript
import { test, expect } from '@playwright/test';
import { JobsPage } from '../pages';

test('should search jobs', async ({ page }) => {
  const jobsPage = new JobsPage(page);

  // Navigate to jobs page
  await jobsPage.goto();

  // Search for jobs
  await jobsPage.searchJobs('my-job');

  // Verify results
  const count = await jobsPage.getJobCount();
  expect(count).toBeGreaterThan(0);
});
```

### Advanced HTMX Example

```typescript
test('should handle HTMX interactions', async ({ page }) => {
  const jobsPage = new JobsPage(page);
  await jobsPage.goto();

  // Filter with HTMX
  await jobsPage.filterByStatus('running');

  // Wait for HTMX to update content
  await jobsPage.waitForHTMXSwap();

  // Verify filtered results
  const count = await jobsPage.getJobCount();
  expect(count).toBeGreaterThanOrEqual(0);
});
```

### Multiple Page Objects

```typescript
test('should navigate between pages', async ({ page }) => {
  const jobsPage = new JobsPage(page);
  const allocationsPage = new AllocationsPage(page);

  // Start at jobs page
  await jobsPage.goto();
  await jobsPage.clickJobByIndex(0);

  // Navigate to allocations
  await jobsPage.goToAllocationsTab();

  // Now use allocations page object
  const allocCount = await allocationsPage.getAllocationCount();
  expect(allocCount).toBeGreaterThanOrEqual(0);
});
```

## JobsPage

Provides methods for interacting with the Jobs list and detail pages.

### Key Methods

- `goto()` - Navigate to jobs list
- `gotoJob(jobID)` - Navigate to specific job detail
- `searchJobs(query)` - Search for jobs
- `filterByStatus(status)` - Filter by job status
- `filterByType(type)` - Filter by job type
- `clickJob(jobName)` - Click job to view details
- `getJobCount()` - Get count of jobs in table
- `getJobNames()` - Get all job names
- `hasJob(jobName)` - Check if job exists
- `runJob()` - Run a job (from detail page)
- `stopJob()` - Stop a job (from detail page)

### Example

```typescript
const jobsPage = new JobsPage(page);
await jobsPage.goto();
await jobsPage.filterByStatus('running');
const count = await jobsPage.getJobCount();
```

## AllocationsPage

Provides methods for interacting with the Allocations list and detail pages.

### Key Methods

- `goto()` - Navigate to allocations list
- `gotoAllocation(allocID)` - Navigate to specific allocation
- `gotoLogs(allocID, taskName?)` - Navigate to allocation logs
- `gotoExec(allocID, taskName?)` - Navigate to exec terminal
- `searchAllocations(query)` - Search allocations
- `filterByStatus(status)` - Filter by allocation status
- `filterByClient(clientName)` - Filter by client
- `clickAllocation(allocName)` - Click allocation to view details
- `getAllocationCount()` - Get count of allocations
- `restartAllocation()` - Restart allocation
- `stopAllocation()` - Stop allocation
- `goToLogs()` - Navigate to logs tab
- `goToExec()` - Navigate to exec tab
- `typeInTerminal(text)` - Type in exec terminal

### Example

```typescript
const allocsPage = new AllocationsPage(page);
await allocsPage.gotoAllocation('abc123');
await allocsPage.goToExec();
await allocsPage.waitForTerminal();
await allocsPage.typeInTerminal('ls -la');
```

## ClientsPage

Provides methods for interacting with the Clients list and detail pages.

### Key Methods

- `goto()` - Navigate to clients list
- `gotoClient(clientID)` - Navigate to specific client
- `searchClients(query)` - Search clients
- `filterByDatacenter(datacenter)` - Filter by datacenter
- `filterByStatus(status)` - Filter by client status
- `filterByClass(nodeClass)` - Filter by node class
- `clickClient(clientName)` - Click client to view details
- `getClientCount()` - Get count of clients
- `getClientNames()` - Get all client names
- `hasClient(clientName)` - Check if client exists
- `toggleDrain()` - Toggle client drain status
- `toggleEligibility()` - Toggle client eligibility
- `areResourceChartsVisible()` - Check if charts are visible

### Example

```typescript
const clientsPage = new ClientsPage(page);
await clientsPage.goto();
await clientsPage.filterByDatacenter('dc1');
const count = await clientsPage.getClientCount();
```

## Best Practices

### 1. Use Page Objects for All UI Interactions

Bad:
```typescript
await page.locator('input[name="search"]').fill('test');
await page.locator('button').click();
```

Good:
```typescript
await jobsPage.searchJobs('test');
```

### 2. Wait for HTMX Swaps

When interacting with HTMX elements, always wait for the swap to complete:

```typescript
await jobsPage.filterByStatus('running');
await jobsPage.waitForHTMXSwap();
```

### 3. Handle Missing Elements Gracefully

Not all elements exist on every page. Check before interacting:

```typescript
const count = await jobsPage.getJobCount();
if (count > 0) {
  await jobsPage.clickJobByIndex(0);
}
```

### 4. Use Descriptive Method Names

Page object methods should be descriptive and focused on user actions:

```typescript
// Good
await jobsPage.searchJobs('my-job');
await jobsPage.filterByStatus('running');

// Bad
await jobsPage.fillInput('search', 'my-job');
await jobsPage.selectDropdown('status', 'running');
```

### 5. Return Page Objects for Chaining

For fluent APIs, consider returning `this`:

```typescript
async searchJobs(query: string): Promise<JobsPage> {
  await this.fill(this.selectors.searchInput, query);
  await this.waitForHTMXSwap();
  return this;
}

// Usage
await jobsPage
  .searchJobs('test')
  .filterByStatus('running');
```

## Adding New Page Objects

To add a new page object:

1. Create a new file: `pages/mypage.page.ts`
2. Extend `BasePage`
3. Define selectors
4. Implement page-specific methods
5. Export from `pages/index.ts`

### Template

```typescript
import { Locator } from '@playwright/test';
import { BasePage } from './base.page';

export class MyPage extends BasePage {
  private readonly selectors = {
    table: 'table',
    row: 'tbody tr',
    searchInput: 'input[type="search"]',
  };

  async goto(): Promise<void> {
    await super.goto('/ui/mypage');
  }

  async search(query: string): Promise<void> {
    await this.fill(this.selectors.searchInput, query);
    await this.waitForHTMXSwap();
  }

  async getRowCount(): Promise<number> {
    return await this.getCount(this.selectors.row);
  }
}
```

## Testing Page Objects

Page objects should be simple data access layers. Complex logic should be in tests, not page objects.

Test the behavior, not the implementation:

```typescript
// Good - tests user behavior
test('should find running jobs', async ({ page }) => {
  const jobsPage = new JobsPage(page);
  await jobsPage.goto();
  await jobsPage.filterByStatus('running');
  const count = await jobsPage.getJobCount();
  expect(count).toBeGreaterThanOrEqual(0);
});

// Bad - tests page object implementation
test('should call waitForHTMXSwap', async ({ page }) => {
  const jobsPage = new JobsPage(page);
  await jobsPage.filterByStatus('running');
  // Testing internal implementation
});
```

## Troubleshooting

### HTMX Not Swapping

If HTMX interactions fail, check:
1. Is HTMX loaded? Check for `window.htmx` in browser console
2. Does the element have HTMX attributes? (`hx-get`, `hx-post`, etc.)
3. Is the target selector correct?

### Element Not Found

If selectors don't match:
1. Inspect the actual HTML in the browser
2. Update the selector in the page object
3. Consider adding data attributes for testing (e.g., `data-testid`)

### Timing Issues

If tests are flaky:
1. Use `waitForHTMXSwap()` after HTMX interactions
2. Use `waitForSelector()` before checking elements
3. Increase timeouts for slow operations

## Further Reading

- [Playwright Page Object Model](https://playwright.dev/docs/pom)
- [HTMX Documentation](https://htmx.org/docs/)
- [Test Best Practices](../README.md)

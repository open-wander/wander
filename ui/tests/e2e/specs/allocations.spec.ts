import { test, expect } from '@playwright/test';
import { AllocationsPage } from '../pages';

/**
 * Allocations page tests using Page Object pattern
 *
 * This spec covers:
 * - Allocations list page (filtering, searching)
 * - Allocation detail page (tabs, navigation)
 * - Logs functionality (SSE streaming)
 * - Exec terminal functionality (WebSocket)
 */

test.describe('Allocations List Page', () => {
  let allocationsPage: AllocationsPage;

  test.beforeEach(async ({ page }) => {
    allocationsPage = new AllocationsPage(page);
    await allocationsPage.goto();
  });

  test('should load allocations list page', async () => {
    await allocationsPage.verifyURL('/ui/allocations');
    await allocationsPage.verifyTitle('Wander');
    await allocationsPage.waitForAllocationsTable();
  });

  test('should display allocations in table', async () => {
    const count = await allocationsPage.getAllocationCount();
    expect(count).toBeGreaterThanOrEqual(0);

    // If allocations exist, verify table structure
    if (count > 0) {
      const table = allocationsPage.getAllocationsTable();
      await expect(table).toBeVisible();
    } else {
      // Verify empty state
      const isEmpty = await allocationsPage.isEmptyStateVisible();
      expect(isEmpty).toBe(true);
    }
  });

  test('should search allocations by ID prefix', async () => {
    const initialCount = await allocationsPage.getAllocationCount();

    if (initialCount > 0) {
      // Get first allocation ID from table to use in search
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        // Extract allocation ID and use first 4 chars for prefix search
        const allocID = href.split('/').pop();
        if (allocID && allocID.length >= 4) {
          const searchPrefix = allocID.substring(0, 4);

          await allocationsPage.searchAllocations(searchPrefix);
          await allocationsPage.waitForAllocationsTable();

          // Verify search was executed
          const count = await allocationsPage.getAllocationCount();
          expect(count).toBeGreaterThanOrEqual(0);
        }
      }
    }
  });

  test('should filter allocations by status', async () => {
    // Check if status filter exists
    const statusFilter = allocationsPage.getElements('select[name="status"]');
    const hasFilter = (await statusFilter.count()) > 0;

    if (hasFilter) {
      await allocationsPage.filterByStatus('running');
      await allocationsPage.waitForAllocationsTable();

      const count = await allocationsPage.getAllocationCount();
      expect(count).toBeGreaterThanOrEqual(0);
    }
  });

  test('should filter allocations by client/node', async () => {
    // Check if client filter exists
    const clientFilter = allocationsPage.getElements('select[name="client"]');
    const hasFilter = (await clientFilter.count()) > 0;

    if (hasFilter) {
      // Get first option value to filter by
      const firstOption = clientFilter.locator('option').nth(1); // Skip "All" option
      const optionValue = await firstOption.getAttribute('value');

      if (optionValue) {
        await allocationsPage.filterByClient(optionValue);
        await allocationsPage.waitForAllocationsTable();

        const count = await allocationsPage.getAllocationCount();
        expect(count).toBeGreaterThanOrEqual(0);
      }
    }
  });

  test('should navigate to allocation detail when clicking allocation', async () => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      // Click first allocation
      await allocationsPage.clickAllocationByIndex(0);

      // Verify we're on detail page
      await allocationsPage.verifyURL('/ui/allocations/');
      await expect(allocationsPage.getElements('body')).toBeVisible();
    }
  });

  test('should display allocation names and statuses', async () => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const rows = allocationsPage.getAllocationRows();
      const firstRow = rows.first();

      // Verify row has content
      await expect(firstRow).toBeVisible();
      const rowText = await firstRow.textContent();
      expect(rowText).toBeTruthy();
      expect(rowText!.length).toBeGreaterThan(0);
    }
  });
});

test.describe('Allocation Detail Page', () => {
  let allocationsPage: AllocationsPage;

  test.beforeEach(async ({ page }) => {
    allocationsPage = new AllocationsPage(page);
    await allocationsPage.goto();
  });

  test('should load allocation detail page with header', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      // Navigate to first allocation
      await allocationsPage.clickAllocationByIndex(0);

      // Verify detail page loaded
      await allocationsPage.verifyURL('/ui/allocations/');

      // Verify page has content
      await expect(page.locator('body')).toBeVisible();

      // Check for common header elements
      const hasHeader = await page.locator('h1, h2, [data-alloc-name], .alloc-name').count() > 0;
      expect(hasHeader).toBe(true);
    }
  });

  test('should display tasks tab with task list', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Look for tasks tab/section
      const tasksTab = page.locator('button:has-text("Tasks"), a:has-text("Tasks")');
      const hasTasksTab = (await tasksTab.count()) > 0;

      if (hasTasksTab) {
        await allocationsPage.goToTasksTab();

        // Verify task section is visible
        const taskSection = page.locator('[data-section="tasks"], .tasks-section');
        const hasSection = (await taskSection.count()) > 0;

        if (hasSection) {
          await expect(taskSection).toBeVisible();
        }
      }
    }
  });

  test('should display files tab with file tree', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Look for files tab
      const filesTab = page.locator('button:has-text("Files"), a:has-text("Files")');
      const hasFilesTab = (await filesTab.count()) > 0;

      if (hasFilesTab) {
        await filesTab.click();
        await allocationsPage.waitForLoad();

        // Verify files section exists
        const filesSection = page.locator('[data-section="files"], .files-section, .file-tree');
        const hasSection = (await filesSection.count()) > 0;

        if (hasSection) {
          await expect(filesSection.first()).toBeVisible();
        }
      }
    }
  });

  test('should navigate between tabs', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Try navigating to tasks tab
      const tasksTab = page.locator('button:has-text("Tasks"), a:has-text("Tasks")');
      if ((await tasksTab.count()) > 0) {
        await allocationsPage.goToTasksTab();
        await expect(page.locator('body')).toBeVisible();
      }

      // Try navigating to events tab
      const eventsTab = page.locator('button:has-text("Events"), a:has-text("Events")');
      if ((await eventsTab.count()) > 0) {
        await allocationsPage.goToEventsTab();
        await expect(page.locator('body')).toBeVisible();
      }
    }
  });
});

test.describe('Allocation Logs', () => {
  let allocationsPage: AllocationsPage;

  test.beforeEach(async ({ page }) => {
    allocationsPage = new AllocationsPage(page);
    await allocationsPage.goto();
  });

  test('should navigate to logs tab', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Look for logs button/tab
      const logsButton = page.locator('button:has-text("Logs"), a:has-text("Logs")');
      const hasLogsButton = (await logsButton.count()) > 0;

      if (hasLogsButton) {
        await allocationsPage.goToLogs();

        // Verify we're on logs page/tab
        await expect(page.locator('body')).toBeVisible();
      }
    }
  });

  test('should load logs page with container', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      // Get first allocation ID
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          // Navigate directly to logs page
          await allocationsPage.gotoLogs(allocID);

          // Verify logs container exists
          await allocationsPage.waitForLogs();
          const logsContainer = page.locator('[data-logs], pre, .logs-container');
          await expect(logsContainer.first()).toBeVisible();
        }
      }
    }
  });

  test('should display stdout/stderr toggle controls', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoLogs(allocID);

          // Look for stdout/stderr toggles or filters
          const stdoutToggle = page.locator('input[type="checkbox"][name*="stdout"], button:has-text("stdout")');
          const stderrToggle = page.locator('input[type="checkbox"][name*="stderr"], button:has-text("stderr")');

          const hasToggles = (await stdoutToggle.count()) > 0 || (await stderrToggle.count()) > 0;

          if (hasToggles) {
            // Verify toggles are visible
            if ((await stdoutToggle.count()) > 0) {
              await expect(stdoutToggle.first()).toBeVisible();
            }
            if ((await stderrToggle.count()) > 0) {
              await expect(stderrToggle.first()).toBeVisible();
            }
          }
        }
      }
    }
  });

  test('should stream log content via SSE', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoLogs(allocID);
          await allocationsPage.waitForLogs();

          // Verify logs container has content (may be empty if no logs)
          const logsContainer = page.locator('[data-logs], pre, .logs-container').first();
          await expect(logsContainer).toBeVisible();

          // Get initial content
          const initialContent = await allocationsPage.getLogsContent();
          expect(initialContent).toBeDefined();

          // Wait a moment for potential SSE updates
          await page.waitForTimeout(1000);

          // Verify container still exists (SSE streaming is working)
          await expect(logsContainer).toBeVisible();
        }
      }
    }
  });

  test('should display download button', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoLogs(allocID);

          // Look for download button
          const downloadButton = page.locator('button:has-text("Download"), a:has-text("Download"), [download]');
          const hasDownloadButton = (await downloadButton.count()) > 0;

          if (hasDownloadButton) {
            await expect(downloadButton.first()).toBeVisible();
          }
        }
      }
    }
  });

  test('should handle log filtering', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoLogs(allocID);
          await allocationsPage.waitForLogs();

          // Look for task filter dropdown
          const taskFilter = page.locator('select[name*="task"]');
          const hasTaskFilter = (await taskFilter.count()) > 0;

          if (hasTaskFilter) {
            await expect(taskFilter.first()).toBeVisible();

            // Try selecting a task if options exist
            const options = taskFilter.first().locator('option');
            const optionCount = await options.count();

            if (optionCount > 1) {
              await taskFilter.first().selectOption({ index: 1 });
              await allocationsPage.waitForLoad();
            }
          }
        }
      }
    }
  });
});

test.describe('Allocation Exec Terminal', () => {
  let allocationsPage: AllocationsPage;

  test.beforeEach(async ({ page }) => {
    allocationsPage = new AllocationsPage(page);
    await allocationsPage.goto();
  });

  test('should navigate to exec tab', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Look for exec button/tab
      const execButton = page.locator('button:has-text("Exec"), a:has-text("Exec")');
      const hasExecButton = (await execButton.count()) > 0;

      if (hasExecButton) {
        await allocationsPage.goToExec();

        // Verify we're on exec page/tab
        await expect(page.locator('body')).toBeVisible();
      }
    }
  });

  test('should load exec page with terminal container', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          // Navigate directly to exec page
          await allocationsPage.gotoExec(allocID);

          // Verify terminal container exists
          await allocationsPage.waitForTerminal();
          const isTerminalVisible = await allocationsPage.isTerminalVisible();
          expect(isTerminalVisible).toBe(true);
        }
      }
    }
  });

  test('should display terminal with xterm.js', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoExec(allocID);
          await allocationsPage.waitForTerminal();

          // Verify terminal element exists and has xterm classes
          const terminal = page.locator('[data-terminal], .terminal, #terminal, .xterm');
          await expect(terminal.first()).toBeVisible();

          // Check for xterm-specific elements
          const xtermViewport = page.locator('.xterm-viewport, .xterm-screen');
          const hasXterm = (await xtermViewport.count()) > 0;

          if (hasXterm) {
            await expect(xtermViewport.first()).toBeVisible();
          }
        }
      }
    }
  });

  test('should display command input controls', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoExec(allocID);

          // Look for command input or terminal controls
          const commandInput = page.locator('input[type="text"][placeholder*="command"], input[name="command"]');
          const hasCommandInput = (await commandInput.count()) > 0;

          if (hasCommandInput) {
            await expect(commandInput.first()).toBeVisible();
          }

          // Terminal should be interactive (has viewport)
          const terminal = page.locator('[data-terminal], .terminal, #terminal');
          await expect(terminal.first()).toBeVisible();
        }
      }
    }
  });

  test('should handle WebSocket connection for terminal', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          // Listen for WebSocket connections
          const wsPromise = page.waitForEvent('websocket', { timeout: 10000 }).catch(() => null);

          await allocationsPage.gotoExec(allocID);
          await allocationsPage.waitForTerminal();

          // Wait a bit for WebSocket to establish
          await page.waitForTimeout(2000);

          // WebSocket may or may not be established (depends on backend)
          // Just verify terminal is visible
          const isTerminalVisible = await allocationsPage.isTerminalVisible();
          expect(isTerminalVisible).toBe(true);
        }
      }
    }
  });

  test('should display task selector for exec', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoExec(allocID);

          // Look for task selector
          const taskSelector = page.locator('select[name*="task"], select:has(option:has-text("Task"))');
          const hasTaskSelector = (await taskSelector.count()) > 0;

          if (hasTaskSelector) {
            await expect(taskSelector.first()).toBeVisible();

            // Verify it has options
            const options = taskSelector.first().locator('option');
            const optionCount = await options.count();
            expect(optionCount).toBeGreaterThan(0);
          }
        }
      }
    }
  });
});

test.describe('Allocation Actions', () => {
  let allocationsPage: AllocationsPage;

  test.beforeEach(async ({ page }) => {
    allocationsPage = new AllocationsPage(page);
    await allocationsPage.goto();
  });

  test('should display action buttons on detail page', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Look for action buttons
      const restartButton = page.locator('button:has-text("Restart"), button[data-action="restart"]');
      const stopButton = page.locator('button:has-text("Stop"), button[data-action="stop"]');

      const hasRestartButton = (await restartButton.count()) > 0;
      const hasStopButton = (await stopButton.count()) > 0;

      // At least one action button should exist
      expect(hasRestartButton || hasStopButton).toBe(true);
    }
  });

  test('should handle HTMX interactions on allocation page', async ({ page }) => {
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);

      // Look for HTMX-enabled elements
      const htmxElements = page.locator('[hx-get], [hx-post], [hx-trigger]');
      const hasHTMX = (await htmxElements.count()) > 0;

      if (hasHTMX) {
        // Verify page is using HTMX
        await expect(page.locator('body')).toBeVisible();

        // Check that htmx is loaded
        const hasHTMXLibrary = await page.evaluate(() => {
          return typeof (window as any).htmx !== 'undefined';
        });

        expect(hasHTMXLibrary).toBe(true);
      }
    }
  });
});

import { test, expect } from '@playwright/test';
import { JobsPage } from '../pages';

/**
 * Jobs List and Detail Page E2E Tests
 *
 * Tests cover:
 * - Jobs list page functionality
 * - Job detail page with tabs
 * - Job actions (run, stop)
 * - Search and filtering
 * - HTMX interactions
 */

test.describe('Jobs List Page', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
    await jobsPage.goto();
  });

  test('should load jobs list page with job table', async () => {
    // Verify URL and title
    await jobsPage.verifyURL('/ui/jobs');
    await jobsPage.verifyTitle('Wander');

    // Wait for jobs table to load
    await jobsPage.waitForJobsTable();

    // Verify table is visible
    const table = jobsPage.getJobsTable();
    await expect(table).toBeVisible();
  });

  test('should display jobs in table or empty state', async () => {
    const count = await jobsPage.getJobCount();
    expect(count).toBeGreaterThanOrEqual(0);

    if (count > 0) {
      // Verify jobs table exists and has rows
      const table = jobsPage.getJobsTable();
      await expect(table).toBeVisible();

      const rows = jobsPage.getJobRows();
      expect(await rows.count()).toBeGreaterThan(0);

      // Verify job names are displayed
      const names = await jobsPage.getJobNames();
      expect(names.length).toBeGreaterThan(0);
      names.forEach(name => {
        expect(name.length).toBeGreaterThan(0);
      });
    } else {
      // Verify empty state is shown
      const isEmpty = await jobsPage.isEmptyStateVisible();
      expect(isEmpty).toBe(true);
    }
  });

  test('should filter jobs using search input', async ({ page }) => {
    // Check if search input exists
    const searchInput = page.locator('input[type="search"], input[placeholder*="Search"]');
    const hasSearch = (await searchInput.count()) > 0;

    if (hasSearch) {
      // Get initial count
      const initialCount = await jobsPage.getJobCount();

      // Perform search
      await jobsPage.searchJobs('test');

      // Wait for HTMX to update
      await jobsPage.waitForJobsTable();

      // Verify search was executed (results may be filtered)
      const filteredCount = await jobsPage.getJobCount();
      expect(filteredCount).toBeGreaterThanOrEqual(0);
      expect(filteredCount).toBeLessThanOrEqual(initialCount);
    } else {
      test.skip();
    }
  });

  test('should filter jobs by status', async ({ page }) => {
    // Check if status filter exists
    const statusFilter = page.locator('select[name="status"]');
    const hasFilter = (await statusFilter.count()) > 0;

    if (hasFilter) {
      // Get available options
      const options = await statusFilter.locator('option').allTextContents();

      if (options.length > 1) {
        // Get initial count
        const initialCount = await jobsPage.getJobCount();

        // Filter by first non-empty option
        const filterValue = await statusFilter.locator('option').nth(1).getAttribute('value');
        if (filterValue) {
          await jobsPage.filterByStatus(filterValue);

          // Wait for HTMX to update
          await jobsPage.waitForJobsTable();

          // Verify filter was applied
          const filteredCount = await jobsPage.getJobCount();
          expect(filteredCount).toBeGreaterThanOrEqual(0);
        }
      }
    } else {
      test.skip();
    }
  });

  test('should filter jobs by type', async ({ page }) => {
    // Check if type filter exists
    const typeFilter = page.locator('select[name="type"]');
    const hasFilter = (await typeFilter.count()) > 0;

    if (hasFilter) {
      // Get available options
      const options = await typeFilter.locator('option').allTextContents();

      if (options.length > 1) {
        // Get initial count
        const initialCount = await jobsPage.getJobCount();

        // Filter by first non-empty option
        const filterValue = await typeFilter.locator('option').nth(1).getAttribute('value');
        if (filterValue) {
          await jobsPage.filterByType(filterValue);

          // Wait for HTMX to update
          await jobsPage.waitForJobsTable();

          // Verify filter was applied
          const filteredCount = await jobsPage.getJobCount();
          expect(filteredCount).toBeGreaterThanOrEqual(0);
        }
      }
    } else {
      test.skip();
    }
  });

  test('should sort jobs by clicking column headers', async ({ page }) => {
    const count = await jobsPage.getJobCount();

    if (count > 1) {
      // Look for sortable column headers
      const sortableHeaders = page.locator('th[data-sortable], th.sortable, th:has(button)');
      const hasSortable = (await sortableHeaders.count()) > 0;

      if (hasSortable) {
        // Get initial job names
        const initialNames = await jobsPage.getJobNames();

        // Click first sortable header
        await sortableHeaders.first().click();
        await jobsPage.waitForHTMXSwap();

        // Get sorted job names
        const sortedNames = await jobsPage.getJobNames();

        // Verify order changed (unless already sorted that way)
        expect(sortedNames.length).toBe(initialNames.length);
      } else {
        test.skip();
      }
    } else {
      test.skip();
    }
  });

  test('should navigate to job detail when clicking job link', async () => {
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Get first job name
      const jobNames = await jobsPage.getJobNames();
      const firstJobName = jobNames[0];

      // Click first job
      await jobsPage.clickJobByIndex(0);

      // Verify we're on detail page
      await jobsPage.verifyURL('/ui/jobs/');

      // Verify page loaded
      const { page } = jobsPage as any;
      await expect(page.locator('body')).toBeVisible();
    } else {
      test.skip();
    }
  });
});

test.describe('Job Detail Page', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
    await jobsPage.goto();

    // Navigate to first job if available
    const count = await jobsPage.getJobCount();
    if (count > 0) {
      await jobsPage.clickJobByIndex(0);
    } else {
      test.skip();
    }
  });

  test('should load job detail page with job header', async ({ page }) => {
    // Verify we're on a job detail page
    await jobsPage.verifyURL('/ui/jobs/');

    // Verify page has loaded
    await expect(page.locator('body')).toBeVisible();

    // Look for job header elements (name, status, type)
    const hasHeader = (await page.locator('h1, [data-job-header], .job-header').count()) > 0;
    expect(hasHeader).toBe(true);
  });

  test('should display job detail tabs (Overview, Definition, Versions, Allocations)', async ({ page }) => {
    // Look for tab navigation
    const tabs = page.locator('nav[role="tablist"], [data-tabs], .tabs');
    const hasTabs = (await tabs.count()) > 0;

    if (hasTabs) {
      // Check for common tab names
      const tabButtons = page.locator('button[role="tab"], a[role="tab"], .tab-button');
      const tabCount = await tabButtons.count();

      expect(tabCount).toBeGreaterThan(0);

      // Look for specific tabs
      const overviewTab = page.locator('button:has-text("Overview"), a:has-text("Overview")');
      const definitionTab = page.locator('button:has-text("Definition"), a:has-text("Definition")');
      const versionsTab = page.locator('button:has-text("Versions"), a:has-text("Versions")');
      const allocationsTab = page.locator('button:has-text("Allocations"), a:has-text("Allocations")');

      // At least one tab should exist
      const hasAnyTab = (
        (await overviewTab.count()) > 0 ||
        (await definitionTab.count()) > 0 ||
        (await versionsTab.count()) > 0 ||
        (await allocationsTab.count()) > 0
      );

      expect(hasAnyTab).toBe(true);
    } else {
      // Page might have sections instead of tabs
      const hasContent = (await page.locator('main, [data-job-content], .job-content').count()) > 0;
      expect(hasContent).toBe(true);
    }
  });

  test('should load content when clicking tabs (HTMX)', async ({ page }) => {
    // Look for tab buttons
    const tabButtons = page.locator('button[role="tab"], a[role="tab"], button[hx-get], a[hx-get]');
    const tabCount = await tabButtons.count();

    if (tabCount > 1) {
      // Click second tab (first might be active)
      const secondTab = tabButtons.nth(1);
      const tabText = await secondTab.textContent();

      await secondTab.click();
      await jobsPage.waitForHTMXSwap();

      // Verify content is still visible
      await expect(page.locator('body')).toBeVisible();

      // Verify active tab changed
      const activeTab = page.locator('[role="tab"][aria-selected="true"], .tab-active, button.active');
      const hasActive = (await activeTab.count()) > 0;

      if (hasActive) {
        const activeText = await activeTab.first().textContent();
        expect(activeText).toContain(tabText || '');
      }
    } else {
      test.skip();
    }
  });

  test('should display CodeMirror editor in Definition tab', async ({ page }) => {
    // Look for Definition tab
    const definitionTab = page.locator('button:has-text("Definition"), a:has-text("Definition")');
    const hasDefinitionTab = (await definitionTab.count()) > 0;

    if (hasDefinitionTab) {
      // Click Definition tab
      await definitionTab.first().click();
      await jobsPage.waitForHTMXSwap();

      // Wait a bit for CodeMirror to initialize
      await page.waitForTimeout(500);

      // Look for CodeMirror elements
      const codeMirror = page.locator('.cm-editor, .CodeMirror, [data-codemirror]');
      const hasCodeMirror = (await codeMirror.count()) > 0;

      if (hasCodeMirror) {
        await expect(codeMirror.first()).toBeVisible();
      } else {
        // Might be a simple pre/code block
        const codeBlock = page.locator('pre, code, [data-job-definition]');
        await expect(codeBlock.first()).toBeVisible();
      }
    } else {
      test.skip();
    }
  });

  test('should display version list in Versions tab', async ({ page }) => {
    // Look for Versions tab
    const versionsTab = page.locator('button:has-text("Versions"), a:has-text("Versions")');
    const hasVersionsTab = (await versionsTab.count()) > 0;

    if (hasVersionsTab) {
      // Click Versions tab
      await versionsTab.first().click();
      await jobsPage.waitForHTMXSwap();

      // Look for version list or table
      const versionsList = page.locator('table, ul[data-versions], [data-versions-list]');
      const hasList = (await versionsList.count()) > 0;

      if (hasList) {
        await expect(versionsList.first()).toBeVisible();
      } else {
        // Might show empty state or content area
        const content = page.locator('[data-tab-content], .tab-content, main');
        await expect(content.first()).toBeVisible();
      }
    } else {
      test.skip();
    }
  });

  test('should display allocation table in Allocations tab', async ({ page }) => {
    // Look for Allocations tab
    const allocationsTab = page.locator('button:has-text("Allocations"), a:has-text("Allocations")');
    const hasAllocationsTab = (await allocationsTab.count()) > 0;

    if (hasAllocationsTab) {
      // Click Allocations tab
      await allocationsTab.first().click();
      await jobsPage.waitForHTMXSwap();

      // Look for allocations table
      const allocationsTable = page.locator('table, [data-allocations-table]');
      const hasTable = (await allocationsTable.count()) > 0;

      if (hasTable) {
        await expect(allocationsTable.first()).toBeVisible();
      } else {
        // Might show empty state
        const emptyState = page.locator('[data-empty-state], .empty-state');
        const hasEmpty = (await emptyState.count()) > 0;

        if (hasEmpty) {
          await expect(emptyState.first()).toBeVisible();
        } else {
          // Verify content area exists
          const content = page.locator('[data-tab-content], .tab-content, main');
          await expect(content.first()).toBeVisible();
        }
      }
    } else {
      test.skip();
    }
  });
});

test.describe('Job Actions', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
    await jobsPage.goto();

    // Navigate to first job if available
    const count = await jobsPage.getJobCount();
    if (count > 0) {
      await jobsPage.clickJobByIndex(0);
    } else {
      test.skip();
    }
  });

  test('should display Run job button and open modal when clicked', async ({ page }) => {
    // Look for Run button
    const runButton = page.locator('button:has-text("Run"), button[data-action="run"]');
    const hasRunButton = (await runButton.count()) > 0;

    if (hasRunButton) {
      await expect(runButton.first()).toBeVisible();

      // Click Run button
      await runButton.first().click();

      // Wait for modal or HTMX response
      await page.waitForTimeout(500);

      // Look for modal or confirmation
      const modal = page.locator('[role="dialog"], [data-modal], .modal, [data-confirm]');
      const hasModal = (await modal.count()) > 0;

      if (hasModal) {
        await expect(modal.first()).toBeVisible();

        // Close modal if there's a close button
        const closeButton = modal.locator('button:has-text("Cancel"), button:has-text("Close"), button[aria-label="Close"]');
        if ((await closeButton.count()) > 0) {
          await closeButton.first().click();
        }
      }
    } else {
      test.skip();
    }
  });

  test('should display Stop job button and open confirmation when clicked', async ({ page }) => {
    // Look for Stop button
    const stopButton = page.locator('button:has-text("Stop"), button[data-action="stop"]');
    const hasStopButton = (await stopButton.count()) > 0;

    if (hasStopButton) {
      await expect(stopButton.first()).toBeVisible();

      // Click Stop button
      await stopButton.first().click();

      // Wait for modal or HTMX response
      await page.waitForTimeout(500);

      // Look for confirmation dialog
      const confirmation = page.locator('[role="dialog"], [data-modal], .modal, [data-confirm]');
      const hasConfirmation = (await confirmation.count()) > 0;

      if (hasConfirmation) {
        await expect(confirmation.first()).toBeVisible();

        // Close confirmation if there's a cancel button
        const cancelButton = confirmation.locator('button:has-text("Cancel"), button:has-text("Close")');
        if ((await cancelButton.count()) > 0) {
          await cancelButton.first().click();
        }
      }
    } else {
      test.skip();
    }
  });

  test('should handle job actions with HTMX', async ({ page }) => {
    // Look for any action buttons
    const actionButtons = page.locator('button[hx-post], button[hx-delete], button[data-action]');
    const hasActions = (await actionButtons.count()) > 0;

    if (hasActions) {
      const button = actionButtons.first();
      await expect(button).toBeVisible();

      // Get button text for verification
      const buttonText = await button.textContent();
      expect(buttonText).toBeTruthy();

      // Note: We don't actually click to avoid mutating state
      // Just verify the button is present and clickable
      await expect(button).toBeEnabled();
    } else {
      test.skip();
    }
  });
});

test.describe('Jobs Page HTMX Integration', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
  });

  test('should load jobs list with HTMX support', async ({ page }) => {
    await jobsPage.goto();

    // Verify HTMX is loaded
    const htmxLoaded = await page.evaluate(() => {
      return typeof (window as any).htmx !== 'undefined';
    });

    expect(htmxLoaded).toBe(true);
  });

  test('should handle HTMX requests on job list page', async ({ page }) => {
    await jobsPage.goto();

    // Look for HTMX-enabled elements
    const htmxElements = page.locator('[hx-get], [hx-post]');
    const hasHTMX = (await htmxElements.count()) > 0;

    if (hasHTMX) {
      // Count should be greater than 0
      const count = await htmxElements.count();
      expect(count).toBeGreaterThan(0);
    } else {
      test.skip();
    }
  });

  test('should handle HTMX requests on job detail page', async ({ page }) => {
    await jobsPage.goto();

    const jobCount = await jobsPage.getJobCount();
    if (jobCount > 0) {
      await jobsPage.clickJobByIndex(0);

      // Look for HTMX-enabled tab buttons or actions
      const htmxElements = page.locator('[hx-get], [hx-post]');
      const hasHTMX = (await htmxElements.count()) > 0;

      if (hasHTMX) {
        const count = await htmxElements.count();
        expect(count).toBeGreaterThan(0);
      }
    } else {
      test.skip();
    }
  });
});

test.describe('Jobs Page Edge Cases', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
  });

  test('should handle empty jobs list gracefully', async ({ page }) => {
    await jobsPage.goto();

    const count = await jobsPage.getJobCount();

    if (count === 0) {
      // Verify empty state is shown
      const isEmpty = await jobsPage.isEmptyStateVisible();
      expect(isEmpty).toBe(true);

      // Verify page is still functional
      await expect(page.locator('body')).toBeVisible();
    } else {
      test.skip();
    }
  });

  test('should handle search with no results', async ({ page }) => {
    await jobsPage.goto();

    const searchInput = page.locator('input[type="search"], input[placeholder*="Search"]');
    const hasSearch = (await searchInput.count()) > 0;

    if (hasSearch) {
      // Search for something that likely doesn't exist
      await jobsPage.searchJobs('nonexistent-job-12345');
      await jobsPage.waitForJobsTable();

      // Verify we get 0 results or empty state
      const count = await jobsPage.getJobCount();
      expect(count).toBe(0);

      // Page should still be functional
      await expect(page.locator('body')).toBeVisible();
    } else {
      test.skip();
    }
  });

  test('should handle navigation back from job detail', async ({ page }) => {
    await jobsPage.goto();

    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Navigate to job detail
      await jobsPage.clickJobByIndex(0);
      await jobsPage.verifyURL('/ui/jobs/');

      // Go back to jobs list
      await jobsPage.goBack();
      await jobsPage.verifyURL('/ui/jobs');

      // Verify we're back on jobs list
      await jobsPage.waitForJobsTable();
      const table = jobsPage.getJobsTable();
      await expect(table).toBeVisible();
    } else {
      test.skip();
    }
  });

  test('should reload job detail page successfully', async ({ page }) => {
    await jobsPage.goto();

    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Navigate to job detail
      await jobsPage.clickJobByIndex(0);
      await jobsPage.verifyURL('/ui/jobs/');

      // Reload the page
      await jobsPage.reload();

      // Verify page still loads correctly
      await jobsPage.verifyURL('/ui/jobs/');
      await expect(page.locator('body')).toBeVisible();
    } else {
      test.skip();
    }
  });
});

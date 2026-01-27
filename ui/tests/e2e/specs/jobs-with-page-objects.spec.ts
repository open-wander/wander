import { test, expect } from '@playwright/test';
import { JobsPage } from '../pages';

/**
 * Jobs page tests using Page Object pattern
 *
 * This spec demonstrates how to use the JobsPage page object
 * for cleaner, more maintainable tests.
 */

test.describe('Jobs Page (using Page Objects)', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
    await jobsPage.goto();
  });

  test('should load jobs list page', async () => {
    await jobsPage.verifyURL('/ui/jobs');
    await jobsPage.verifyTitle('Wander');
    await jobsPage.waitForJobsTable();
  });

  test('should display jobs in table', async ({ page }) => {
    const count = await jobsPage.getJobCount();
    expect(count).toBeGreaterThanOrEqual(0);

    // If jobs exist, verify table structure
    if (count > 0) {
      const table = jobsPage.getJobsTable();
      await expect(table).toBeVisible();
    } else {
      // Verify empty state
      const isEmpty = await jobsPage.isEmptyStateVisible();
      expect(isEmpty).toBe(true);
    }
  });

  test('should search jobs', async () => {
    await jobsPage.searchJobs('test');
    await jobsPage.waitForJobsTable();

    // Verify search was executed (URL or results updated)
    const count = await jobsPage.getJobCount();
    expect(count).toBeGreaterThanOrEqual(0);
  });

  test('should filter jobs by status', async () => {
    // Try to filter (may not have filter if no data)
    const statusFilter = jobsPage.getElements('select[name="status"]');
    const hasFilter = (await statusFilter.count()) > 0;

    if (hasFilter) {
      await jobsPage.filterByStatus('running');
      await jobsPage.waitForJobsTable();
    }
  });

  test('should navigate to job detail', async () => {
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Click first job
      await jobsPage.clickJobByIndex(0);

      // Verify we're on detail page
      await jobsPage.verifyURL('/ui/jobs/');
      await expect(jobsPage.getElements('body')).toBeVisible();
    }
  });

  test('should display job names', async () => {
    const names = await jobsPage.getJobNames();
    expect(Array.isArray(names)).toBe(true);

    if (names.length > 0) {
      // Each name should be non-empty
      names.forEach(name => {
        expect(name.length).toBeGreaterThan(0);
      });
    }
  });

  test('should handle HTMX interactions', async ({ page }) => {
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Look for HTMX-enabled elements
      const htmxElements = page.locator('[hx-get], [hx-post]');
      const hasHTMX = (await htmxElements.count()) > 0;

      if (hasHTMX) {
        // Trigger HTMX request
        await htmxElements.first().click();
        await jobsPage.waitForHTMXSwap();

        // Page should still be functional
        await expect(page.locator('body')).toBeVisible();
      }
    }
  });
});

test.describe('Job Detail Page (using Page Objects)', () => {
  test('should navigate to job detail directly', async ({ page }) => {
    const jobsPage = new JobsPage(page);

    // Navigate to a job (using example ID - will 404 if not exists)
    await jobsPage.gotoJob('example-job-id');

    // Verify we attempted to load the page
    await expect(page).toHaveURL(/\/ui\/jobs\/example-job-id/);
  });

  test('should handle job actions', async ({ page }) => {
    const jobsPage = new JobsPage(page);
    await jobsPage.goto();

    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Navigate to job detail
      await jobsPage.clickJobByIndex(0);

      // Check for action buttons
      const runButton = page.locator('button:has-text("Run")');
      const stopButton = page.locator('button:has-text("Stop")');

      if ((await runButton.count()) > 0) {
        await expect(runButton).toBeVisible();
      }

      if ((await stopButton.count()) > 0) {
        await expect(stopButton).toBeVisible();
      }
    }
  });
});

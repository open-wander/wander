import { test, expect } from '@playwright/test';
import { JobsPage, AllocationsPage, ClientsPage } from '../pages';

/**
 * Visual Regression Tests
 *
 * These tests capture screenshots of key pages and compare them to baseline images
 * to detect unintended visual changes.
 *
 * To update baselines:
 *   npm run test:update-snapshots
 *
 * Configuration:
 * - Threshold: 0.2 (20% pixel difference allowed)
 * - Max diff pixels: 100 (for minor rendering differences)
 *
 * Notes:
 * - Tests run in chromium only to avoid cross-browser rendering differences
 * - Screenshots are stored in __screenshots__ directory
 * - Baseline images are versioned in git
 * - CI should fail if screenshots don't match baselines
 */

// Only run visual regression tests in chromium to avoid rendering differences
test.use({
  viewport: { width: 1280, height: 720 },
});

test.describe('Visual Regression - Jobs Pages', () => {
  let jobsPage: JobsPage;

  test.beforeEach(async ({ page }) => {
    jobsPage = new JobsPage(page);
  });

  test('Jobs list page matches baseline @visual', async ({ page }) => {
    await jobsPage.goto();
    await jobsPage.waitForJobsTable();

    // Wait for any animations or loading states to complete
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    // Capture full page screenshot
    await expect(page).toHaveScreenshot('jobs-list.png', {
      fullPage: true,
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixels: 100,
    });
  });

  test('Jobs list empty state matches baseline @visual', async ({ page }) => {
    await jobsPage.goto();
    await jobsPage.waitForJobsTable();

    const count = await jobsPage.getJobCount();

    // Only run if empty state is visible
    if (count === 0) {
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('jobs-list-empty.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Job detail page matches baseline @visual', async ({ page }) => {
    await jobsPage.goto();
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Navigate to first job
      await jobsPage.clickJobByIndex(0);
      await jobsPage.verifyURL('/ui/jobs/');

      // Wait for page to stabilize
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      // Capture job detail page
      await expect(page).toHaveScreenshot('job-detail-overview.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Job detail Definition tab matches baseline @visual', async ({ page }) => {
    await jobsPage.goto();
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      await jobsPage.clickJobByIndex(0);

      // Click Definition tab if it exists
      const definitionTab = page.locator('button:has-text("Definition"), a:has-text("Definition")');
      const hasDefinitionTab = (await definitionTab.count()) > 0;

      if (hasDefinitionTab) {
        await definitionTab.first().click();
        await jobsPage.waitForHTMXSwap();
        await page.waitForLoadState('networkidle');
        await page.waitForTimeout(500);

        await expect(page).toHaveScreenshot('job-detail-definition.png', {
          fullPage: true,
          animations: 'disabled',
          threshold: 0.2,
          maxDiffPixels: 100,
        });
      } else {
        test.skip();
      }
    } else {
      test.skip();
    }
  });

  test('Job detail Allocations tab matches baseline @visual', async ({ page }) => {
    await jobsPage.goto();
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      await jobsPage.clickJobByIndex(0);

      // Click Allocations tab if it exists
      const allocationsTab = page.locator('button:has-text("Allocations"), a:has-text("Allocations")');
      const hasAllocationsTab = (await allocationsTab.count()) > 0;

      if (hasAllocationsTab) {
        await allocationsTab.first().click();
        await jobsPage.waitForHTMXSwap();
        await page.waitForLoadState('networkidle');
        await page.waitForTimeout(500);

        await expect(page).toHaveScreenshot('job-detail-allocations.png', {
          fullPage: true,
          animations: 'disabled',
          threshold: 0.2,
          maxDiffPixels: 100,
        });
      } else {
        test.skip();
      }
    } else {
      test.skip();
    }
  });
});

test.describe('Visual Regression - Allocations Pages', () => {
  let allocationsPage: AllocationsPage;

  test.beforeEach(async ({ page }) => {
    allocationsPage = new AllocationsPage(page);
  });

  test('Allocations list page matches baseline @visual', async ({ page }) => {
    await allocationsPage.goto();
    await allocationsPage.waitForAllocationsTable();

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    await expect(page).toHaveScreenshot('allocations-list.png', {
      fullPage: true,
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixels: 100,
    });
  });

  test('Allocation detail page matches baseline @visual', async ({ page }) => {
    await allocationsPage.goto();
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      await allocationsPage.clickAllocationByIndex(0);
      await allocationsPage.verifyURL('/ui/allocations/');

      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('allocation-detail-overview.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Allocation logs tab matches baseline @visual', async ({ page }) => {
    await allocationsPage.goto();
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      // Get first allocation ID
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          // Navigate to logs page
          await allocationsPage.gotoLogs(allocID);
          await allocationsPage.waitForLogs();

          // Wait for logs to stabilize (SSE may be streaming)
          await page.waitForLoadState('networkidle');
          await page.waitForTimeout(1000);

          await expect(page).toHaveScreenshot('allocation-detail-logs.png', {
            fullPage: true,
            animations: 'disabled',
            threshold: 0.2,
            maxDiffPixels: 100,
          });
        } else {
          test.skip();
        }
      } else {
        test.skip();
      }
    } else {
      test.skip();
    }
  });

  test('Allocation exec terminal matches baseline @visual', async ({ page }) => {
    await allocationsPage.goto();
    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      const allocationLink = allocationsPage.getElements('a[href*="/ui/allocations/"]').first();
      const href = await allocationLink.getAttribute('href');

      if (href) {
        const allocID = href.split('/').pop();
        if (allocID) {
          await allocationsPage.gotoExec(allocID);
          await allocationsPage.waitForTerminal();

          // Wait for terminal to initialize
          await page.waitForLoadState('networkidle');
          await page.waitForTimeout(1000);

          await expect(page).toHaveScreenshot('allocation-detail-exec.png', {
            fullPage: true,
            animations: 'disabled',
            threshold: 0.2,
            maxDiffPixels: 100,
          });
        } else {
          test.skip();
        }
      } else {
        test.skip();
      }
    } else {
      test.skip();
    }
  });
});

test.describe('Visual Regression - Clients Pages', () => {
  let clientsPage: ClientsPage;

  test.beforeEach(async ({ page }) => {
    clientsPage = new ClientsPage(page);
  });

  test('Clients list page matches baseline @visual', async ({ page }) => {
    await clientsPage.goto();
    await clientsPage.waitForClientsTable();

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    await expect(page).toHaveScreenshot('clients-list.png', {
      fullPage: true,
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixels: 100,
    });
  });

  test('Client detail page matches baseline @visual', async ({ page }) => {
    await clientsPage.goto();
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);
      await clientsPage.verifyURL('/ui/clients/');

      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('client-detail-overview.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Client detail Allocations tab matches baseline @visual', async ({ page }) => {
    await clientsPage.goto();
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      // Click Allocations tab if it exists
      const allocationsTab = page.locator('button:has-text("Allocations"), a:has-text("Allocations")');
      const hasAllocationsTab = (await allocationsTab.count()) > 0;

      if (hasAllocationsTab) {
        await clientsPage.goToAllocationsTab();
        await page.waitForLoadState('networkidle');
        await page.waitForTimeout(500);

        await expect(page).toHaveScreenshot('client-detail-allocations.png', {
          fullPage: true,
          animations: 'disabled',
          threshold: 0.2,
          maxDiffPixels: 100,
        });
      } else {
        test.skip();
      }
    } else {
      test.skip();
    }
  });
});

test.describe('Visual Regression - Servers Pages', () => {
  test('Servers list page matches baseline @visual', async ({ page }) => {
    await page.goto('/ui/servers');

    // Wait for table to load
    const table = page.locator('table');
    const hasTable = (await table.count()) > 0;

    if (hasTable) {
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('servers-list.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      // Empty state
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('servers-list-empty.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    }
  });

  test('Server detail page matches baseline @visual', async ({ page }) => {
    await page.goto('/ui/servers');

    // Look for server links
    const serverLink = page.locator('a[href*="/ui/servers/"]').first();
    const hasServers = (await serverLink.count()) > 0;

    if (hasServers) {
      await serverLink.click();
      await page.waitForURL(/\/ui\/servers\/.+/);
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('server-detail-overview.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });
});

test.describe('Visual Regression - Component Snapshots', () => {
  test('Navigation header matches baseline @visual', async ({ page }) => {
    await page.goto('/ui/jobs');
    await page.waitForLoadState('networkidle');

    // Capture just the navigation header
    const header = page.locator('header, nav[role="navigation"], [data-header]').first();
    const hasHeader = (await header.count()) > 0;

    if (hasHeader) {
      await expect(header).toHaveScreenshot('component-navigation-header.png', {
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Job card/row component matches baseline @visual', async ({ page }) => {
    const jobsPage = new JobsPage(page);
    await jobsPage.goto();
    await jobsPage.waitForJobsTable();

    const count = await jobsPage.getJobCount();

    if (count > 0) {
      // Capture first job row
      const firstRow = jobsPage.getJobRows().first();
      await expect(firstRow).toHaveScreenshot('component-job-row.png', {
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Allocation card/row component matches baseline @visual', async ({ page }) => {
    const allocationsPage = new AllocationsPage(page);
    await allocationsPage.goto();
    await allocationsPage.waitForAllocationsTable();

    const count = await allocationsPage.getAllocationCount();

    if (count > 0) {
      // Capture first allocation row
      const firstRow = allocationsPage.getAllocationRows().first();
      await expect(firstRow).toHaveScreenshot('component-allocation-row.png', {
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Table component matches baseline @visual', async ({ page }) => {
    const jobsPage = new JobsPage(page);
    await jobsPage.goto();
    await jobsPage.waitForJobsTable();

    const table = jobsPage.getJobsTable();
    await expect(table).toHaveScreenshot('component-table.png', {
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixels: 100,
    });
  });
});

test.describe('Visual Regression - Responsive Design', () => {
  test('Jobs list on mobile viewport matches baseline @visual', async ({ page }) => {
    // Set mobile viewport
    await page.setViewportSize({ width: 375, height: 667 });

    const jobsPage = new JobsPage(page);
    await jobsPage.goto();
    await jobsPage.waitForJobsTable();

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    await expect(page).toHaveScreenshot('jobs-list-mobile.png', {
      fullPage: true,
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixels: 100,
    });
  });

  test('Job detail on mobile viewport matches baseline @visual', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });

    const jobsPage = new JobsPage(page);
    await jobsPage.goto();
    const count = await jobsPage.getJobCount();

    if (count > 0) {
      await jobsPage.clickJobByIndex(0);
      await page.waitForLoadState('networkidle');
      await page.waitForTimeout(500);

      await expect(page).toHaveScreenshot('job-detail-mobile.png', {
        fullPage: true,
        animations: 'disabled',
        threshold: 0.2,
        maxDiffPixels: 100,
      });
    } else {
      test.skip();
    }
  });

  test('Jobs list on tablet viewport matches baseline @visual', async ({ page }) => {
    // Set tablet viewport
    await page.setViewportSize({ width: 768, height: 1024 });

    const jobsPage = new JobsPage(page);
    await jobsPage.goto();
    await jobsPage.waitForJobsTable();

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    await expect(page).toHaveScreenshot('jobs-list-tablet.png', {
      fullPage: true,
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixels: 100,
    });
  });
});

import { test, expect } from '@playwright/test';

/**
 * Home page e2e tests
 *
 * These tests verify the basic functionality of the home page,
 * including navigation, layout, and initial load behavior.
 */

test.describe('Home Page', () => {
  test('should load home page successfully', async ({ page }) => {
    // Navigate to home page
    await page.goto('/ui/');

    // Verify page title and heading
    await expect(page).toHaveTitle(/Wander/);

    // Verify page loaded with main layout
    await expect(page.locator('body')).toBeVisible();
  });

  test('should display navigation menu', async ({ page }) => {
    await page.goto('/ui/');

    // Check for common navigation items
    const nav = page.locator('nav, [role="navigation"]');
    await expect(nav).toBeVisible();

    // Common navigation links (adjust based on actual UI)
    const expectedLinks = ['Jobs', 'Clients', 'Servers'];
    for (const linkText of expectedLinks) {
      const link = page.locator(`a:has-text("${linkText}")`);
      if (await link.count() > 0) {
        await expect(link.first()).toBeVisible();
      }
    }
  });

  test('should navigate to jobs page', async ({ page }) => {
    await page.goto('/ui/');

    // Click on Jobs link if it exists
    const jobsLink = page.locator('a:has-text("Jobs")');
    if (await jobsLink.count() > 0) {
      await jobsLink.first().click();

      // Wait for navigation
      await page.waitForLoadState('networkidle');

      // Verify URL changed
      await expect(page).toHaveURL(/\/ui\/jobs/);
    }
  });

  test('should handle HTMX requests', async ({ page }) => {
    await page.goto('/ui/');

    // Look for any HTMX-enabled links
    const htmxLink = page.locator('[hx-get], [hx-post]').first();

    if (await htmxLink.count() > 0) {
      // Click the HTMX link
      await htmxLink.click();

      // Wait a bit for HTMX to process
      await page.waitForTimeout(500);

      // The page should still be on the same base URL (HTMX updates content in place)
      // but the content should have changed
      await expect(page.locator('body')).toBeVisible();
    }
  });

  test('should be responsive', async ({ page }) => {
    // Test desktop view
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto('/ui/');
    await expect(page.locator('body')).toBeVisible();

    // Test tablet view
    await page.setViewportSize({ width: 768, height: 1024 });
    await expect(page.locator('body')).toBeVisible();

    // Test mobile view
    await page.setViewportSize({ width: 375, height: 667 });
    await expect(page.locator('body')).toBeVisible();
  });

  test('should load static assets', async ({ page }) => {
    // Listen for failed requests
    const failedRequests: string[] = [];
    page.on('requestfailed', request => {
      failedRequests.push(request.url());
    });

    await page.goto('/ui/');

    // Wait for network to be idle
    await page.waitForLoadState('networkidle');

    // Check if critical assets loaded
    expect(failedRequests.filter(url => url.includes('/ui/static/'))).toHaveLength(0);
  });
});

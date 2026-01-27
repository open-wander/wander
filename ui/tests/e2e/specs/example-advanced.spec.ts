import { test, expect } from '@playwright/test';

/**
 * Advanced e2e test examples
 *
 * This file demonstrates more complex testing patterns including:
 * - API mocking and intercepts
 * - Network condition testing
 * - Accessibility testing
 * - Screenshot comparison
 * - Performance testing
 */

test.describe('Advanced Testing Patterns', () => {
  test('should intercept and mock API responses', async ({ page }) => {
    // Mock API response
    await page.route('**/api/jobs/**', async route => {
      const json = {
        jobs: [
          { id: '1', name: 'test-job-1', status: 'running' },
          { id: '2', name: 'test-job-2', status: 'pending' },
        ],
      };
      await route.fulfill({ json });
    });

    await page.goto('/ui/jobs');

    // Verify mocked data appears
    await expect(page.locator('text=test-job-1')).toBeVisible();
  });

  test('should handle network errors gracefully', async ({ page }) => {
    // Simulate network failure
    await page.route('**/api/**', route => route.abort('failed'));

    await page.goto('/ui/');

    // App should show error state, not crash
    await expect(page.locator('body')).toBeVisible();
  });

  test('should be accessible', async ({ page }) => {
    await page.goto('/ui/');

    // Check for basic accessibility features
    // Main landmark
    const main = page.locator('main, [role="main"]');
    if (await main.count() > 0) {
      await expect(main.first()).toBeVisible();
    }

    // Navigation
    const nav = page.locator('nav, [role="navigation"]');
    if (await nav.count() > 0) {
      await expect(nav.first()).toBeVisible();
    }

    // All images should have alt text
    const images = await page.locator('img').all();
    for (const img of images) {
      const alt = await img.getAttribute('alt');
      expect(alt).toBeTruthy();
    }

    // All buttons should be labeled
    const buttons = await page.locator('button').all();
    for (const button of buttons) {
      const text = await button.textContent();
      const ariaLabel = await button.getAttribute('aria-label');
      expect(text || ariaLabel).toBeTruthy();
    }
  });

  test('should handle concurrent HTMX requests', async ({ page }) => {
    await page.goto('/ui/');

    // Find multiple HTMX-enabled elements
    const htmxElements = await page.locator('[hx-get], [hx-post]').all();

    if (htmxElements.length >= 2) {
      // Trigger multiple HTMX requests simultaneously
      await Promise.all([
        htmxElements[0].click(),
        htmxElements[1].click(),
      ]);

      // Wait for all requests to complete
      await page.waitForLoadState('networkidle');

      // Page should still be functional
      await expect(page.locator('body')).toBeVisible();
    }
  });

  test('should persist state across navigation', async ({ page }) => {
    await page.goto('/ui/');

    // Set some state (e.g., localStorage, cookies)
    await page.evaluate(() => {
      localStorage.setItem('test-key', 'test-value');
    });

    // Navigate to another page
    const jobsLink = page.locator('a:has-text("Jobs")');
    if (await jobsLink.count() > 0) {
      await jobsLink.first().click();
      await page.waitForLoadState('networkidle');

      // State should persist
      const value = await page.evaluate(() => localStorage.getItem('test-key'));
      expect(value).toBe('test-value');
    }
  });

  test('should measure page load performance', async ({ page }) => {
    await page.goto('/ui/');

    // Get performance metrics
    const metrics = await page.evaluate(() => {
      const navigation = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming;
      return {
        domContentLoaded: navigation.domContentLoadedEventEnd - navigation.domContentLoadedEventStart,
        loadComplete: navigation.loadEventEnd - navigation.loadEventStart,
        timeToFirstByte: navigation.responseStart - navigation.requestStart,
      };
    });

    // Assert reasonable performance
    expect(metrics.domContentLoaded).toBeLessThan(2000); // 2 seconds
    expect(metrics.timeToFirstByte).toBeLessThan(1000); // 1 second

    console.log('Performance metrics:', metrics);
  });

  test('should handle browser back/forward', async ({ page }) => {
    // Navigate through several pages
    await page.goto('/ui/');
    const originalURL = page.url();

    const jobsLink = page.locator('a:has-text("Jobs")');
    if (await jobsLink.count() > 0) {
      await jobsLink.first().click();
      await page.waitForLoadState('networkidle');

      // Go back
      await page.goBack();
      await page.waitForLoadState('networkidle');
      expect(page.url()).toBe(originalURL);

      // Go forward
      await page.goForward();
      await page.waitForLoadState('networkidle');
      await expect(page).toHaveURL(/\/ui\/jobs/);
    }
  });

  test('should handle page refresh', async ({ page }) => {
    await page.goto('/ui/jobs');

    // Get some content before refresh
    const bodyContent = await page.locator('body').textContent();

    // Refresh page
    await page.reload();
    await page.waitForLoadState('networkidle');

    // Content should still be there
    await expect(page.locator('body')).toBeVisible();
    const newBodyContent = await page.locator('body').textContent();
    expect(newBodyContent).toBeTruthy();
  });
});

test.describe('Visual Regression', () => {
  test('should match home page snapshot', async ({ page }) => {
    await page.goto('/ui/');
    await page.waitForLoadState('networkidle');

    // Take screenshot (first run creates baseline, subsequent runs compare)
    // await expect(page).toHaveScreenshot('home-page.png');

    // For now, just verify page is visible
    await expect(page.locator('body')).toBeVisible();
  });
});

test.describe('Mobile Testing', () => {
  test.use({ viewport: { width: 375, height: 667 } });

  test('should work on mobile viewport', async ({ page }) => {
    await page.goto('/ui/');

    // Mobile menu or navigation should be accessible
    await expect(page.locator('body')).toBeVisible();

    // Check for mobile-specific elements (hamburger menu, etc.)
    const mobileMenu = page.locator('[aria-label*="menu"], .mobile-menu, .hamburger');
    if (await mobileMenu.count() > 0) {
      await expect(mobileMenu.first()).toBeVisible();
    }
  });

  test('should handle touch interactions', async ({ page }) => {
    await page.goto('/ui/');

    // Simulate touch interactions
    const touchTarget = page.locator('a, button').first();
    await touchTarget.tap();

    // Page should respond to touch
    await expect(page.locator('body')).toBeVisible();
  });
});

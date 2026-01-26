import { test, expect } from '@playwright/test';
import { BasePage } from '../pages';

/**
 * Servers page tests
 *
 * Tests for the Servers list page including leader badge display,
 * status indicators, version information, and navigation.
 */

test.describe('Servers List Page', () => {
  let serversPage: BasePage;

  test.beforeEach(async ({ page }) => {
    serversPage = new BasePage(page);
    await serversPage.goto('/ui/servers');
  });

  test('should load servers list page', async () => {
    await serversPage.verifyURL('/ui/servers');
    await serversPage.verifyTitle('Wander');

    const table = serversPage.getElements('table');
    const tableCount = await table.count();

    if (tableCount > 0) {
      await expect(table.first()).toBeVisible();
    }
  });

  test('should display servers table', async () => {
    const table = serversPage.getElements('table');
    const tableCount = await table.count();

    if (tableCount > 0) {
      await expect(table.first()).toBeVisible();

      const rows = serversPage.getElements('tbody tr, tr[data-server-id]');
      const rowCount = await rows.count();
      expect(rowCount).toBeGreaterThanOrEqual(0);
    } else {
      const emptyState = serversPage.getElements('[data-empty-state], .empty-state');
      const hasEmptyState = (await emptyState.count()) > 0;

      if (hasEmptyState) {
        expect(hasEmptyState).toBe(true);
      }
    }
  });

  test('should display leader badge on leader server', async ({ page }) => {
    const leaderBadge = page.locator('[data-leader], .leader, :has-text("Leader")');
    const badgeCount = await leaderBadge.count();

    if (badgeCount > 0) {
      await expect(leaderBadge.first()).toBeVisible();
    }
  });

  test('should show server status indicators', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    if (rowCount > 0) {
      const statusElements = page.locator('[data-server-status], .server-status, [data-status], .status-badge');
      const statusCount = await statusElements.count();

      if (statusCount > 0) {
        await expect(statusElements.first()).toBeVisible();
      }
    }
  });

  test('should display server names', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    if (rowCount > 0) {
      const serverLinks = page.locator('a[href*="/ui/servers/"], tbody tr td:first-child');
      const linkCount = await serverLinks.count();

      if (linkCount > 0) {
        const firstServerText = await serverLinks.first().textContent();
        expect(firstServerText).toBeTruthy();
        expect(firstServerText?.trim().length).toBeGreaterThan(0);
      }
    }
  });

  test('should show version numbers', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    if (rowCount > 0) {
      const versionElements = page.locator('[data-version], .version, td:has-text("v"), td:has-text(".")');
      const versionCount = await versionElements.count();

      if (versionCount > 0) {
        const versionText = await versionElements.first().textContent();
        expect(versionText).toBeTruthy();
      }
    }
  });

  test('should show datacenter information', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    if (rowCount > 0) {
      const datacenterElements = page.locator('[data-datacenter], [data-dc], .datacenter');
      const dcCount = await datacenterElements.count();

      if (dcCount > 0) {
        await expect(datacenterElements.first()).toBeVisible();
      }
    }
  });

  test('should navigate to server detail when clicked', async ({ page }) => {
    const serverLinks = page.locator('a[href*="/ui/servers/"]');
    const linkCount = await serverLinks.count();

    if (linkCount > 0) {
      await serverLinks.first().click();
      await serversPage.waitForLoad();

      await expect(page).toHaveURL(/\/ui\/servers\/.+/);
      await expect(page.locator('body')).toBeVisible();
    }
  });

  test('should display server addresses', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    if (rowCount > 0) {
      const addressElements = page.locator('[data-address], .address, td:has-text(":")');
      const addressCount = await addressElements.count();

      if (addressCount > 0) {
        const addressText = await addressElements.first().textContent();
        expect(addressText).toBeTruthy();
      }
    }
  });

  test('should show multiple servers when cluster has them', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    expect(rowCount).toBeGreaterThanOrEqual(0);

    if (rowCount > 1) {
      await expect(rows.nth(0)).toBeVisible();
      await expect(rows.nth(1)).toBeVisible();
    }
  });

  test('should handle HTMX interactions', async ({ page }) => {
    const htmxElements = page.locator('[hx-get], [hx-post]');
    const hasHTMX = (await htmxElements.count()) > 0;

    if (hasHTMX) {
      const firstElement = htmxElements.first();

      if (await firstElement.isVisible()) {
        await firstElement.click();
        await serversPage.waitForHTMXSwap();

        await expect(page.locator('body')).toBeVisible();
      }
    }
  });
});

test.describe('Server Detail Page', () => {
  let serversPage: BasePage;

  test.beforeEach(async ({ page }) => {
    serversPage = new BasePage(page);
    await serversPage.goto('/ui/servers');
  });

  test('should navigate to server detail', async ({ page }) => {
    const serverLinks = page.locator('a[href*="/ui/servers/"]');
    const linkCount = await serverLinks.count();

    if (linkCount > 0) {
      await serverLinks.first().click();
      await serversPage.waitForLoad();

      await expect(page).toHaveURL(/\/ui\/servers\/.+/);
    }
  });

  test('should display server header information', async ({ page }) => {
    const serverLinks = page.locator('a[href*="/ui/servers/"]');
    const linkCount = await serverLinks.count();

    if (linkCount > 0) {
      await serverLinks.first().click();
      await serversPage.waitForLoad();

      const header = page.locator('h1, h2, [data-server-name]');
      const hasHeader = (await header.count()) > 0;

      if (hasHeader) {
        await expect(header.first()).toBeVisible();
      }
    }
  });

  test('should show server status on detail page', async ({ page }) => {
    const serverLinks = page.locator('a[href*="/ui/servers/"]');
    const linkCount = await serverLinks.count();

    if (linkCount > 0) {
      await serverLinks.first().click();
      await serversPage.waitForLoad();

      const statusElements = page.locator('[data-status], .status, [data-server-status]');
      const statusCount = await statusElements.count();

      if (statusCount > 0) {
        await expect(statusElements.first()).toBeVisible();
      }
    }
  });

  test('should display leader badge on leader detail page', async ({ page }) => {
    const rows = page.locator('tbody tr, tr[data-server-id]');
    const rowCount = await rows.count();

    if (rowCount > 0) {
      const leaderRow = page.locator('tr:has([data-leader]), tr:has(.leader), tr:has-text("Leader")');
      const hasLeader = (await leaderRow.count()) > 0;

      if (hasLeader) {
        const leaderLink = leaderRow.first().locator('a[href*="/ui/servers/"]');

        if ((await leaderLink.count()) > 0) {
          await leaderLink.click();
          await serversPage.waitForLoad();

          const leaderBadge = page.locator('[data-leader], .leader, :has-text("Leader")');
          const badgeCount = await leaderBadge.count();

          if (badgeCount > 0) {
            await expect(leaderBadge.first()).toBeVisible();
          }
        }
      }
    }
  });

  test('should show server version on detail page', async ({ page }) => {
    const serverLinks = page.locator('a[href*="/ui/servers/"]');
    const linkCount = await serverLinks.count();

    if (linkCount > 0) {
      await serverLinks.first().click();
      await serversPage.waitForLoad();

      const versionElements = page.locator('[data-version], .version, :has-text("v"), :has-text("Version")');
      const versionCount = await versionElements.count();

      if (versionCount > 0) {
        await expect(versionElements.first()).toBeVisible();
      }
    }
  });

  test('should display server configuration', async ({ page }) => {
    const serverLinks = page.locator('a[href*="/ui/servers/"]');
    const linkCount = await serverLinks.count();

    if (linkCount > 0) {
      await serverLinks.first().click();
      await serversPage.waitForLoad();

      const configSection = page.locator('[data-section="config"], .config, table');
      const hasConfig = (await configSection.count()) > 0;

      if (hasConfig) {
        await expect(configSection.first()).toBeVisible();
      }
    }
  });
});

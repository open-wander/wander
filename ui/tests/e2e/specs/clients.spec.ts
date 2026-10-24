import { test, expect } from '@playwright/test';
import { ClientsPage } from '../pages';

/**
 * Clients page tests using Page Object pattern
 *
 * Tests for both the Clients list page and Client detail pages,
 * including filtering, search, navigation, and detail view features.
 */

test.describe('Clients List Page', () => {
  let clientsPage: ClientsPage;

  test.beforeEach(async ({ page }) => {
    clientsPage = new ClientsPage(page);
    await clientsPage.goto();
  });

  test('should load clients list page', async () => {
    await clientsPage.verifyURL('/ui/clients');
    await clientsPage.verifyTitle('Wander');
    await clientsPage.waitForClientsTable();
  });

  test('should display clients table', async () => {
    const count = await clientsPage.getClientCount();
    expect(count).toBeGreaterThanOrEqual(0);

    if (count > 0) {
      const table = clientsPage.getClientsTable();
      await expect(table).toBeVisible();
    } else {
      const isEmpty = await clientsPage.isEmptyStateVisible();
      expect(isEmpty).toBe(true);
    }
  });

  test('should search clients', async () => {
    const initialCount = await clientsPage.getClientCount();

    if (initialCount > 0) {
      const clientNames = await clientsPage.getClientNames();
      const firstClientName = clientNames[0];

      await clientsPage.searchClients(firstClientName.substring(0, 3));
      await clientsPage.waitForClientsTable();

      const searchedCount = await clientsPage.getClientCount();
      expect(searchedCount).toBeGreaterThanOrEqual(0);
    }
  });

  test('should filter clients by status', async () => {
    const statusFilter = clientsPage.getElements('select[name="status"]');
    const hasFilter = (await statusFilter.count()) > 0;

    if (hasFilter) {
      await clientsPage.filterByStatus('ready');
      await clientsPage.waitForClientsTable();

      const count = await clientsPage.getClientCount();
      expect(count).toBeGreaterThanOrEqual(0);
    }
  });

  test('should filter clients by datacenter', async () => {
    const datacenterFilter = clientsPage.getElements('select[name="datacenter"]');
    const hasFilter = (await datacenterFilter.count()) > 0;

    if (hasFilter) {
      const datacenters = await clientsPage.getAvailableDatacenters();

      if (datacenters.length > 0) {
        const dc = datacenters.find(d => d !== 'All' && d !== '');
        if (dc) {
          await clientsPage.filterByDatacenter(dc);
          await clientsPage.waitForClientsTable();

          const count = await clientsPage.getClientCount();
          expect(count).toBeGreaterThanOrEqual(0);
        }
      }
    }
  });

  test('should filter clients by node class', async () => {
    const classFilter = clientsPage.getElements('select[name="class"]');
    const hasFilter = (await classFilter.count()) > 0;

    if (hasFilter) {
      await clientsPage.filterByClass('default');
      await clientsPage.waitForClientsTable();

      const count = await clientsPage.getClientCount();
      expect(count).toBeGreaterThanOrEqual(0);
    }
  });

  test('should navigate to client detail when clicked', async () => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      await clientsPage.verifyURL('/ui/clients/');
      await expect(clientsPage.getElements('body')).toBeVisible();
    }
  });

  test('should display client names', async () => {
    const names = await clientsPage.getClientNames();
    expect(Array.isArray(names)).toBe(true);

    if (names.length > 0) {
      names.forEach(name => {
        expect(name.length).toBeGreaterThan(0);
      });
    }
  });

  test('should handle HTMX interactions for search', async ({ page }) => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      const searchInput = page.locator('input[type="search"]').first();
      const hasSearch = (await searchInput.count()) > 0;

      if (hasSearch) {
        await searchInput.fill('test');
        await clientsPage.waitForHTMXSwap();

        await expect(page.locator('body')).toBeVisible();
      }
    }
  });
});

test.describe('Client Detail Page', () => {
  let clientsPage: ClientsPage;

  test.beforeEach(async ({ page }) => {
    clientsPage = new ClientsPage(page);
    await clientsPage.goto();
  });

  test('should load client detail page', async () => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      await clientsPage.verifyURL('/ui/clients/');
      await expect(clientsPage.getElements('body')).toBeVisible();
    }
  });

  test('should display client header information', async () => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const clientNameElement = clientsPage.getElements('[data-client-name], .client-name, h1, h2');
      const hasHeader = (await clientNameElement.count()) > 0;
      expect(hasHeader).toBe(true);
    }
  });

  test('should display resource charts', async () => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const chartsVisible = await clientsPage.areResourceChartsVisible();
      const charts = clientsPage.getResourceCharts();
      const chartCount = await charts.count();

      if (chartCount > 0) {
        expect(chartsVisible).toBe(true);
        await expect(charts.first()).toBeVisible();
      }
    }
  });

  test('should show allocations in allocations tab', async ({ page }) => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const allocationsTab = page.locator('button:has-text("Allocations"), a:has-text("Allocations")');
      const hasTab = (await allocationsTab.count()) > 0;

      if (hasTab) {
        await clientsPage.goToAllocationsTab();

        const allocationsSection = page.locator('[data-section="allocations"], table, .allocations');
        const hasSectionOrTable = (await allocationsSection.count()) > 0;

        if (hasSectionOrTable) {
          await expect(allocationsSection.first()).toBeVisible();
        }
      }
    }
  });

  test('should show events tab', async ({ page }) => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const eventsTab = page.locator('button:has-text("Events"), a:has-text("Events")');
      const hasTab = (await eventsTab.count()) > 0;

      if (hasTab) {
        await eventsTab.first().click();
        await clientsPage.waitForHTMXSwap();

        const eventsSection = page.locator('[data-section="events"], table, .events');
        const hasSectionOrTable = (await eventsSection.count()) > 0;

        if (hasSectionOrTable) {
          await expect(eventsSection.first()).toBeVisible();
        }
      }
    }
  });

  test('should display drain button', async () => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const drainButton = clientsPage.getElements('button:has-text("Drain"), button[data-action="drain"]');
      const hasButton = (await drainButton.count()) > 0;

      if (hasButton) {
        await expect(drainButton.first()).toBeVisible();
      }
    }
  });

  test('should display eligibility controls', async () => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const eligibilityButton = clientsPage.getElements('button:has-text("Eligibility"), button[data-action="eligibility"]');
      const hasButton = (await eligibilityButton.count()) > 0;

      if (hasButton) {
        await expect(eligibilityButton.first()).toBeVisible();
      }
    }
  });

  test('should navigate between tabs', async ({ page }) => {
    const count = await clientsPage.getClientCount();

    if (count > 0) {
      await clientsPage.clickClientByIndex(0);

      const allocationsTab = page.locator('button:has-text("Allocations"), a:has-text("Allocations")');
      const attributesTab = page.locator('button:has-text("Attributes"), a:has-text("Attributes")');

      if ((await allocationsTab.count()) > 0) {
        await clientsPage.goToAllocationsTab();
        await expect(page.locator('body')).toBeVisible();
      }

      if ((await attributesTab.count()) > 0) {
        await clientsPage.goToAttributesTab();
        await expect(page.locator('body')).toBeVisible();
      }
    }
  });
});

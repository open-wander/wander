import { Locator } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * ClientsPage provides page object methods for the Clients list and detail pages.
 *
 * Routes:
 * - /ui/clients - Clients list
 * - /ui/clients/{clientID} - Client detail
 */
export class ClientsPage extends BasePage {
  // Selectors
  private readonly selectors = {
    clientsTable: 'table',
    clientRow: 'tr[data-client-id], tbody tr',
    clientLink: 'a[href*="/ui/clients/"]',
    searchInput: 'input[type="search"], input[placeholder*="Search"]',
    datacenterFilter: 'select[name="datacenter"]',
    statusFilter: 'select[name="status"]',
    classFilter: 'select[name="class"]',
    clientStatus: '[data-client-status], .client-status',
    clientName: '[data-client-name], .client-name',
    clientDatacenter: '[data-client-dc], .client-datacenter',
    drainButton: 'button:has-text("Drain"), button[data-action="drain"]',
    eligibilityButton: 'button:has-text("Eligibility"), button[data-action="eligibility"]',
    resourceCharts: '[data-chart], .chart',
    allocationsSection: '[data-section="allocations"]',
    attributesSection: '[data-section="attributes"]',
    metaSection: '[data-section="meta"]',
    emptyState: '[data-empty-state], .empty-state',
  };

  /**
   * Navigate to clients list page
   */
  async goto(): Promise<void> {
    await super.goto('/ui/clients');
  }

  /**
   * Navigate to specific client detail page
   */
  async gotoClient(clientID: string): Promise<void> {
    await super.goto(`/ui/clients/${clientID}`);
  }

  /**
   * Search for clients
   */
  async searchClients(query: string): Promise<void> {
    const searchInput = this.page.locator(this.selectors.searchInput).first();
    await searchInput.fill(query);
    await this.waitForHTMXSwap();
  }

  /**
   * Filter clients by datacenter
   */
  async filterByDatacenter(datacenter: string): Promise<void> {
    const datacenterFilter = this.page.locator(this.selectors.datacenterFilter).first();
    await datacenterFilter.selectOption(datacenter);
    await this.waitForHTMXSwap();
  }

  /**
   * Filter clients by status
   */
  async filterByStatus(status: string): Promise<void> {
    const statusFilter = this.page.locator(this.selectors.statusFilter).first();
    await statusFilter.selectOption(status);
    await this.waitForHTMXSwap();
  }

  /**
   * Filter clients by node class
   */
  async filterByClass(nodeClass: string): Promise<void> {
    const classFilter = this.page.locator(this.selectors.classFilter).first();
    await classFilter.selectOption(nodeClass);
    await this.waitForHTMXSwap();
  }

  /**
   * Click on a client to view details
   */
  async clickClient(clientName: string): Promise<void> {
    const clientLink = this.page.locator(`a:has-text("${clientName}")`).first();
    await clientLink.click();
    await this.waitForLoad();
  }

  /**
   * Click on a client by index
   */
  async clickClientByIndex(index: number): Promise<void> {
    const clientLink = this.page.locator(this.selectors.clientLink).nth(index);
    await clientLink.click();
    await this.waitForLoad();
  }

  /**
   * Get the count of clients in the table
   */
  async getClientCount(): Promise<number> {
    const rows = this.page.locator(this.selectors.clientRow);
    return await rows.count();
  }

  /**
   * Get all client names from the table
   */
  async getClientNames(): Promise<string[]> {
    const clientLinks = this.page.locator(this.selectors.clientLink);
    const count = await clientLinks.count();
    const names: string[] = [];

    for (let i = 0; i < count; i++) {
      const text = await clientLinks.nth(i).textContent();
      if (text) {
        names.push(text.trim());
      }
    }

    return names;
  }

  /**
   * Get client status
   */
  async getClientStatus(clientName: string): Promise<string | null> {
    const row = this.page.locator(`tr:has-text("${clientName}")`);
    const status = row.locator(this.selectors.clientStatus).first();
    return await status.textContent();
  }

  /**
   * Check if a client exists in the list
   */
  async hasClient(clientName: string): Promise<boolean> {
    const count = await this.page.locator(`a:has-text("${clientName}")`).count();
    return count > 0;
  }

  /**
   * Toggle client drain status
   */
  async toggleDrain(): Promise<void> {
    const drainButton = this.page.locator(this.selectors.drainButton).first();
    await drainButton.click();
    await this.waitForHTMXRequest();
  }

  /**
   * Toggle client eligibility
   */
  async toggleEligibility(): Promise<void> {
    const eligibilityButton = this.page.locator(this.selectors.eligibilityButton).first();
    await eligibilityButton.click();
    await this.waitForHTMXRequest();
  }

  /**
   * Check if resource charts are visible
   */
  async areResourceChartsVisible(): Promise<boolean> {
    return await this.isVisible(this.selectors.resourceCharts);
  }

  /**
   * Navigate to allocations tab
   */
  async goToAllocationsTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Allocations"), a:has-text("Allocations")');
  }

  /**
   * Navigate to attributes tab
   */
  async goToAttributesTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Attributes"), a:has-text("Attributes")');
  }

  /**
   * Navigate to metadata tab
   */
  async goToMetaTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Meta"), a:has-text("Metadata")');
  }

  /**
   * Check if empty state is shown
   */
  async isEmptyStateVisible(): Promise<boolean> {
    return await this.isVisible(this.selectors.emptyState);
  }

  /**
   * Get the clients table element
   */
  getClientsTable(): Locator {
    return this.page.locator(this.selectors.clientsTable);
  }

  /**
   * Wait for clients table to load
   */
  async waitForClientsTable(): Promise<void> {
    await this.waitForSelector(this.selectors.clientsTable);
  }

  /**
   * Get all client rows
   */
  getClientRows(): Locator {
    return this.page.locator(this.selectors.clientRow);
  }

  /**
   * Get resource charts
   */
  getResourceCharts(): Locator {
    return this.page.locator(this.selectors.resourceCharts);
  }

  /**
   * Get datacenters from filter dropdown
   */
  async getAvailableDatacenters(): Promise<string[]> {
    const datacenterFilter = this.page.locator(this.selectors.datacenterFilter).first();
    const options = await datacenterFilter.locator('option').all();
    const datacenters: string[] = [];

    for (const option of options) {
      const text = await option.textContent();
      if (text && text.trim() !== '') {
        datacenters.push(text.trim());
      }
    }

    return datacenters;
  }
}

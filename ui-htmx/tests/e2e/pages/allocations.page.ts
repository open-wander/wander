import { Locator } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * AllocationsPage provides page object methods for the Allocations list and detail pages.
 *
 * Routes:
 * - /ui/allocations - Allocations list
 * - /ui/allocations/{allocID} - Allocation detail
 * - /ui/allocations/{allocID}/logs - Allocation logs
 * - /ui/allocations/{allocID}/exec - Allocation exec terminal
 */
export class AllocationsPage extends BasePage {
  // Selectors
  private readonly selectors = {
    allocationsTable: 'table',
    allocationRow: 'tr[data-alloc-id], tbody tr',
    allocationLink: 'a[href*="/ui/allocations/"]',
    searchInput: 'input[type="search"], input[placeholder*="Search"]',
    statusFilter: 'select[name="status"]',
    clientFilter: 'select[name="client"]',
    allocationStatus: '[data-alloc-status], .alloc-status',
    allocationName: '[data-alloc-name], .alloc-name',
    taskGroup: '[data-task-group], .task-group',
    restartButton: 'button:has-text("Restart"), button[data-action="restart"]',
    stopButton: 'button:has-text("Stop"), button[data-action="stop"]',
    logsButton: 'button:has-text("Logs"), a:has-text("Logs")',
    execButton: 'button:has-text("Exec"), a:has-text("Exec")',
    logsContainer: '[data-logs], pre, .logs-container',
    terminalContainer: '[data-terminal], .terminal, #terminal',
    taskSection: '[data-section="tasks"]',
    eventsSection: '[data-section="events"]',
    emptyState: '[data-empty-state], .empty-state',
  };

  /**
   * Navigate to allocations list page
   */
  async goto(): Promise<void> {
    await super.goto('/ui/allocations');
  }

  /**
   * Navigate to specific allocation detail page
   */
  async gotoAllocation(allocID: string): Promise<void> {
    await super.goto(`/ui/allocations/${allocID}`);
  }

  /**
   * Navigate to allocation logs page
   */
  async gotoLogs(allocID: string, taskName?: string): Promise<void> {
    const path = taskName
      ? `/ui/allocations/${allocID}/${taskName}/logs`
      : `/ui/allocations/${allocID}/logs`;
    await super.goto(path);
  }

  /**
   * Navigate to allocation exec terminal
   */
  async gotoExec(allocID: string, taskName?: string): Promise<void> {
    const path = taskName
      ? `/ui/allocations/${allocID}/${taskName}/exec`
      : `/ui/allocations/${allocID}/exec`;
    await super.goto(path);
  }

  /**
   * Search for allocations
   */
  async searchAllocations(query: string): Promise<void> {
    const searchInput = this.page.locator(this.selectors.searchInput).first();
    await searchInput.fill(query);
    await this.waitForHTMXSwap();
  }

  /**
   * Filter allocations by status
   */
  async filterByStatus(status: string): Promise<void> {
    const statusFilter = this.page.locator(this.selectors.statusFilter).first();
    await statusFilter.selectOption(status);
    await this.waitForHTMXSwap();
  }

  /**
   * Filter allocations by client
   */
  async filterByClient(clientName: string): Promise<void> {
    const clientFilter = this.page.locator(this.selectors.clientFilter).first();
    await clientFilter.selectOption(clientName);
    await this.waitForHTMXSwap();
  }

  /**
   * Click on an allocation to view details
   */
  async clickAllocation(allocName: string): Promise<void> {
    const allocLink = this.page.locator(`a:has-text("${allocName}")`).first();
    await allocLink.click();
    await this.waitForLoad();
  }

  /**
   * Click on an allocation by index
   */
  async clickAllocationByIndex(index: number): Promise<void> {
    const allocLink = this.page.locator(this.selectors.allocationLink).nth(index);
    await allocLink.click();
    await this.waitForLoad();
  }

  /**
   * Get the count of allocations in the table
   */
  async getAllocationCount(): Promise<number> {
    const rows = this.page.locator(this.selectors.allocationRow);
    return await rows.count();
  }

  /**
   * Get allocation status
   */
  async getAllocationStatus(allocName: string): Promise<string | null> {
    const row = this.page.locator(`tr:has-text("${allocName}")`);
    const status = row.locator(this.selectors.allocationStatus).first();
    return await status.textContent();
  }

  /**
   * Check if an allocation exists in the list
   */
  async hasAllocation(allocName: string): Promise<boolean> {
    const count = await this.page.locator(`a:has-text("${allocName}")`).count();
    return count > 0;
  }

  /**
   * Restart an allocation
   */
  async restartAllocation(): Promise<void> {
    const restartButton = this.page.locator(this.selectors.restartButton).first();
    await restartButton.click();
    await this.waitForHTMXRequest();
  }

  /**
   * Stop an allocation
   */
  async stopAllocation(): Promise<void> {
    const stopButton = this.page.locator(this.selectors.stopButton).first();
    await stopButton.click();
    await this.waitForHTMXRequest();
  }

  /**
   * Navigate to logs tab
   */
  async goToLogs(): Promise<void> {
    const logsButton = this.page.locator(this.selectors.logsButton).first();
    await logsButton.click();
    await this.waitForLoad();
  }

  /**
   * Navigate to exec tab
   */
  async goToExec(): Promise<void> {
    const execButton = this.page.locator(this.selectors.execButton).first();
    await execButton.click();
    await this.waitForLoad();
  }

  /**
   * Get logs content
   */
  async getLogsContent(): Promise<string | null> {
    const logsContainer = this.page.locator(this.selectors.logsContainer).first();
    return await logsContainer.textContent();
  }

  /**
   * Check if terminal is visible
   */
  async isTerminalVisible(): Promise<boolean> {
    return await this.isVisible(this.selectors.terminalContainer);
  }

  /**
   * Type in terminal
   */
  async typeInTerminal(text: string): Promise<void> {
    const terminal = this.page.locator(this.selectors.terminalContainer).first();
    await terminal.click();
    await this.page.keyboard.type(text);
  }

  /**
   * Press Enter in terminal
   */
  async pressEnterInTerminal(): Promise<void> {
    await this.page.keyboard.press('Enter');
  }

  /**
   * Navigate to tasks tab
   */
  async goToTasksTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Tasks"), a:has-text("Tasks")');
  }

  /**
   * Navigate to events tab
   */
  async goToEventsTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Events"), a:has-text("Events")');
  }

  /**
   * Check if empty state is shown
   */
  async isEmptyStateVisible(): Promise<boolean> {
    return await this.isVisible(this.selectors.emptyState);
  }

  /**
   * Get the allocations table element
   */
  getAllocationsTable(): Locator {
    return this.page.locator(this.selectors.allocationsTable);
  }

  /**
   * Wait for allocations table to load
   */
  async waitForAllocationsTable(): Promise<void> {
    await this.waitForSelector(this.selectors.allocationsTable);
  }

  /**
   * Get all allocation rows
   */
  getAllocationRows(): Locator {
    return this.page.locator(this.selectors.allocationRow);
  }

  /**
   * Wait for logs to load
   */
  async waitForLogs(): Promise<void> {
    await this.waitForSelector(this.selectors.logsContainer);
  }

  /**
   * Wait for terminal to load
   */
  async waitForTerminal(): Promise<void> {
    await this.waitForSelector(this.selectors.terminalContainer);
  }
}

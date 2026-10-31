import { Locator } from '@playwright/test';
import { BasePage } from './base.page';

/**
 * JobsPage provides page object methods for the Jobs list and detail pages.
 *
 * Routes:
 * - /ui/jobs - Jobs list
 * - /ui/jobs/{jobID} - Job detail
 */
export class JobsPage extends BasePage {
  // Selectors
  private readonly selectors = {
    jobsTable: 'table',
    jobRow: 'tr[data-job-id], tbody tr',
    jobLink: 'a[href*="/ui/jobs/"]',
    searchInput: 'input[type="search"], input[placeholder*="Search"]',
    filterDropdown: 'select[name="status"], select[name="type"]',
    statusFilter: 'select[name="status"]',
    typeFilter: 'select[name="type"]',
    jobStatus: '[data-job-status], .job-status',
    jobName: '[data-job-name], .job-name',
    jobType: '[data-job-type], .job-type',
    runButton: 'button:has-text("Run"), button[data-action="run"]',
    stopButton: 'button:has-text("Stop"), button[data-action="stop"]',
    jobDefinition: 'pre, code, [data-job-definition]',
    allocationsSection: '[data-section="allocations"]',
    evaluationsSection: '[data-section="evaluations"]',
    deploymentsSection: '[data-section="deployments"]',
    emptyState: '[data-empty-state], .empty-state',
  };

  /**
   * Navigate to jobs list page
   */
  async goto(): Promise<void> {
    await super.goto('/ui/jobs');
  }

  /**
   * Navigate to specific job detail page
   */
  async gotoJob(jobID: string): Promise<void> {
    await super.goto(`/ui/jobs/${jobID}`);
  }

  /**
   * Search for jobs by name or ID
   */
  async searchJobs(query: string): Promise<void> {
    const searchInput = this.page.locator(this.selectors.searchInput).first();
    await searchInput.fill(query);
    // Wait for HTMX to update results
    await this.waitForHTMXSwap();
  }

  /**
   * Filter jobs by status
   */
  async filterByStatus(status: string): Promise<void> {
    const statusFilter = this.page.locator(this.selectors.statusFilter).first();
    await statusFilter.selectOption(status);
    await this.waitForHTMXSwap();
  }

  /**
   * Filter jobs by type
   */
  async filterByType(type: string): Promise<void> {
    const typeFilter = this.page.locator(this.selectors.typeFilter).first();
    await typeFilter.selectOption(type);
    await this.waitForHTMXSwap();
  }

  /**
   * Click on a job to view details
   */
  async clickJob(jobName: string): Promise<void> {
    const jobLink = this.page.locator(`a:has-text("${jobName}")`).first();
    await jobLink.click();
    await this.waitForLoad();
  }

  /**
   * Click on a job by index in the table
   */
  async clickJobByIndex(index: number): Promise<void> {
    const jobLink = this.page.locator(this.selectors.jobLink).nth(index);
    await jobLink.click();
    await this.waitForLoad();
  }

  /**
   * Get the count of jobs in the table
   */
  async getJobCount(): Promise<number> {
    const rows = this.page.locator(this.selectors.jobRow);
    return await rows.count();
  }

  /**
   * Get all job names from the table
   */
  async getJobNames(): Promise<string[]> {
    const jobLinks = this.page.locator(this.selectors.jobLink);
    const count = await jobLinks.count();
    const names: string[] = [];

    for (let i = 0; i < count; i++) {
      const text = await jobLinks.nth(i).textContent();
      if (text) {
        names.push(text.trim());
      }
    }

    return names;
  }

  /**
   * Get job status for a specific job
   */
  async getJobStatus(jobName: string): Promise<string | null> {
    const row = this.page.locator(`tr:has-text("${jobName}")`);
    const status = row.locator(this.selectors.jobStatus).first();
    return await status.textContent();
  }

  /**
   * Check if a job exists in the list
   */
  async hasJob(jobName: string): Promise<boolean> {
    const count = await this.page.locator(`a:has-text("${jobName}")`).count();
    return count > 0;
  }

  /**
   * Run a job (from job detail page)
   */
  async runJob(): Promise<void> {
    const runButton = this.page.locator(this.selectors.runButton).first();
    await runButton.click();
    await this.waitForHTMXRequest();
  }

  /**
   * Stop a job (from job detail page)
   */
  async stopJob(): Promise<void> {
    const stopButton = this.page.locator(this.selectors.stopButton).first();
    await stopButton.click();
    await this.waitForHTMXRequest();
  }

  /**
   * Get job definition text
   */
  async getJobDefinition(): Promise<string | null> {
    const definition = this.page.locator(this.selectors.jobDefinition).first();
    return await definition.textContent();
  }

  /**
   * Check if empty state is shown
   */
  async isEmptyStateVisible(): Promise<boolean> {
    return await this.isVisible(this.selectors.emptyState);
  }

  /**
   * Get the jobs table element
   */
  getJobsTable(): Locator {
    return this.page.locator(this.selectors.jobsTable);
  }

  /**
   * Navigate to allocations tab (on job detail page)
   */
  async goToAllocationsTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Allocations"), a:has-text("Allocations")');
  }

  /**
   * Navigate to evaluations tab (on job detail page)
   */
  async goToEvaluationsTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Evaluations"), a:has-text("Evaluations")');
  }

  /**
   * Navigate to deployments tab (on job detail page)
   */
  async goToDeploymentsTab(): Promise<void> {
    await this.clickHTMX('button:has-text("Deployments"), a:has-text("Deployments")');
  }

  /**
   * Wait for jobs table to load
   */
  async waitForJobsTable(): Promise<void> {
    await this.waitForSelector(this.selectors.jobsTable);
  }

  /**
   * Get all job rows
   */
  getJobRows(): Locator {
    return this.page.locator(this.selectors.jobRow);
  }
}

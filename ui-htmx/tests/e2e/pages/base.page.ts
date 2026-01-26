import { Page, Locator, expect } from '@playwright/test';

/**
 * BasePage provides common functionality for all page objects.
 *
 * Features:
 * - Navigation and page load helpers
 * - HTMX interaction helpers
 * - Common element selectors
 * - Wait utilities
 */
export class BasePage {
  protected page: Page;
  protected baseURL: string;

  constructor(page: Page) {
    this.page = page;
    this.baseURL = process.env.BASE_URL || 'http://localhost:3001';
  }

  /**
   * Navigate to a specific path
   */
  async goto(path: string): Promise<void> {
    await this.page.goto(path);
    await this.waitForLoad();
  }

  /**
   * Wait for page to be fully loaded
   */
  async waitForLoad(): Promise<void> {
    await this.page.waitForLoadState('domcontentloaded');
    // Wait a bit for HTMX to initialize
    await this.page.waitForTimeout(100);
  }

  /**
   * Get the page title
   */
  async getTitle(): Promise<string> {
    return await this.page.title();
  }

  /**
   * Get the current URL
   */
  getURL(): string {
    return this.page.url();
  }

  /**
   * Navigate by clicking a link
   */
  async clickLink(text: string): Promise<void> {
    const link = this.page.locator(`a:has-text("${text}")`);
    await link.click();
    await this.waitForNavigation();
  }

  /**
   * Wait for navigation to complete
   */
  async waitForNavigation(): Promise<void> {
    await this.page.waitForLoadState('networkidle');
  }

  /**
   * Check if element is visible
   */
  async isVisible(selector: string): Promise<boolean> {
    try {
      await this.page.locator(selector).waitFor({ state: 'visible', timeout: 5000 });
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Wait for HTMX swap to complete
   * This waits for the htmx:afterSwap event on the specified target
   */
  async waitForHTMXSwap(targetSelector?: string): Promise<void> {
    const target = targetSelector || 'body';
    await this.page.evaluate((selector) => {
      return new Promise((resolve) => {
        const element = document.querySelector(selector);
        if (element) {
          element.addEventListener('htmx:afterSwap', () => resolve(true), { once: true });
          // Timeout after 5 seconds
          setTimeout(() => resolve(true), 5000);
        } else {
          resolve(true);
        }
      });
    }, target);
  }

  /**
   * Trigger an HTMX element by selector
   * This is useful for triggering HTMX requests without clicking
   */
  async triggerHTMX(selector: string): Promise<void> {
    await this.page.evaluate((sel) => {
      const element = document.querySelector(sel);
      if (element && (window as any).htmx) {
        (window as any).htmx.trigger(element, 'htmx:trigger');
      }
    }, selector);
    await this.waitForHTMXSwap();
  }

  /**
   * Wait for HTMX request to complete
   * Waits for htmx:afterRequest event
   */
  async waitForHTMXRequest(): Promise<void> {
    await this.page.evaluate(() => {
      return new Promise((resolve) => {
        document.body.addEventListener('htmx:afterRequest', () => resolve(true), { once: true });
        // Timeout after 10 seconds
        setTimeout(() => resolve(true), 10000);
      });
    });
  }

  /**
   * Click an HTMX-enabled element and wait for swap
   */
  async clickHTMX(selector: string, targetSelector?: string): Promise<void> {
    await this.page.locator(selector).click();
    await this.waitForHTMXSwap(targetSelector);
  }

  /**
   * Fill a form field
   */
  async fill(selector: string, value: string): Promise<void> {
    await this.page.locator(selector).fill(value);
  }

  /**
   * Click a button by text
   */
  async clickButton(text: string): Promise<void> {
    await this.page.locator(`button:has-text("${text}")`).click();
  }

  /**
   * Select a dropdown option
   */
  async select(selector: string, value: string): Promise<void> {
    await this.page.locator(selector).selectOption(value);
  }

  /**
   * Get text content from an element
   */
  async getText(selector: string): Promise<string | null> {
    return await this.page.locator(selector).textContent();
  }

  /**
   * Get all matching elements
   */
  getElements(selector: string): Locator {
    return this.page.locator(selector);
  }

  /**
   * Wait for a specific selector to appear
   */
  async waitForSelector(selector: string, timeout: number = 5000): Promise<void> {
    await this.page.locator(selector).waitFor({ state: 'visible', timeout });
  }

  /**
   * Wait for a specific selector to disappear
   */
  async waitForSelectorToDisappear(selector: string, timeout: number = 5000): Promise<void> {
    await this.page.locator(selector).waitFor({ state: 'hidden', timeout });
  }

  /**
   * Take a screenshot
   */
  async screenshot(filename: string): Promise<void> {
    await this.page.screenshot({ path: filename, fullPage: true });
  }

  /**
   * Get the navigation menu
   */
  getNavigation(): Locator {
    return this.page.locator('nav, [role="navigation"]');
  }

  /**
   * Check if element has a specific class
   */
  async hasClass(selector: string, className: string): Promise<boolean> {
    const classes = await this.page.locator(selector).getAttribute('class');
    return classes ? classes.split(' ').includes(className) : false;
  }

  /**
   * Get count of elements matching selector
   */
  async getCount(selector: string): Promise<number> {
    return await this.page.locator(selector).count();
  }

  /**
   * Verify page title contains expected text
   */
  async verifyTitle(expectedText: string): Promise<void> {
    await expect(this.page).toHaveTitle(new RegExp(expectedText));
  }

  /**
   * Verify URL contains expected path
   */
  async verifyURL(expectedPath: string): Promise<void> {
    await expect(this.page).toHaveURL(new RegExp(expectedPath));
  }

  /**
   * Reload the page
   */
  async reload(): Promise<void> {
    await this.page.reload();
    await this.waitForLoad();
  }

  /**
   * Go back in browser history
   */
  async goBack(): Promise<void> {
    await this.page.goBack();
    await this.waitForLoad();
  }

  /**
   * Go forward in browser history
   */
  async goForward(): Promise<void> {
    await this.page.goForward();
    await this.waitForLoad();
  }
}

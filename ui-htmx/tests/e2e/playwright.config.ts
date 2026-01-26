import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Wander UI e2e tests
 *
 * The test server is started by the Go test setup (setup.go)
 * before tests run and stopped after tests complete.
 */
export default defineConfig({
  // Test directory
  testDir: './specs',

  // Maximum time one test can run
  timeout: 30 * 1000,

  // Test execution settings
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,

  // Reporter configuration
  reporter: [
    ['html', { open: 'never' }],
    ['list'],
    ['json', { outputFile: 'test-results.json' }]
  ],

  // Shared settings for all projects
  use: {
    // Base URL for the test server (started by Go setup)
    baseURL: process.env.BASE_URL || 'http://localhost:3001',

    // Collect trace on failure for debugging
    trace: 'on-first-retry',

    // Screenshot on failure
    screenshot: 'only-on-failure',

    // Video on failure
    video: 'retain-on-failure',

    // Maximum time for each action (click, fill, etc.)
    actionTimeout: 10 * 1000,

    // Navigation timeout
    navigationTimeout: 30 * 1000,
  },

  // Projects for different browsers
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // Enable viewport for responsive testing
        viewport: { width: 1280, height: 720 },
      },
    },

    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        viewport: { width: 1280, height: 720 },
      },
    },

    {
      name: 'webkit',
      use: {
        ...devices['Desktop Safari'],
        viewport: { width: 1280, height: 720 },
      },
    },

    // Mobile testing
    {
      name: 'mobile-chrome',
      use: {
        ...devices['Pixel 5'],
      },
    },

    {
      name: 'mobile-safari',
      use: {
        ...devices['iPhone 12'],
      },
    },

    // Tablet testing
    {
      name: 'tablet',
      use: {
        ...devices['iPad Pro'],
      },
    },
  ],

  // Folder for test artifacts
  outputDir: 'test-results/',

  // Run web server before starting tests
  // NOTE: The Go test server is started by setup.go, not by Playwright
  // This section is commented out to avoid conflicts
  // webServer: {
  //   command: 'go run ../../cmd/dev/main.go -port 3001',
  //   url: 'http://localhost:3001',
  //   reuseExistingServer: !process.env.CI,
  //   timeout: 120 * 1000,
  // },
});

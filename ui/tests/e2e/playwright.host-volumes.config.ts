import { defineConfig } from '@playwright/test';
import baseConfig from './playwright.config';

/**
 * Dynamic host volume pages run against a fake API started by
 * TestHostVolumeBrowserStates. The release configuration keeps its own
 * testMatch, so the live-cluster suite never runs this specification.
 */
export default defineConfig({
  ...baseConfig,
  testMatch: 'host-volumes.spec.ts',
  retries: 0,
  fullyParallel: false,
  workers: 1,
  reporter: [['list']],
});

import { expect, Page, test } from '@playwright/test';

// Synthetic fixtures served by the fake API in host_volumes_browser_test.go.
const nodeA = 'aaaaaaaa-1111-2222-3333-444444444444';
const nodeB = 'bbbbbbbb-1111-2222-3333-444444444444';
const volumeA = '11111111-aaaa-bbbb-cccc-000000000001';
const volumeB = '22222222-aaaa-bbbb-cccc-000000000002';
const allocation = 'dddddddd-1111-2222-3333-444444444444';

// The standalone test server serves only the UI, so the connection-status
// poll of the agent API answers 404 there. That is a harness artefact, not a
// page error, and is the only message ignored.
function captureBrowserErrors(page: Page) {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => {
    if (message.type() !== 'error' && message.type() !== 'warning') return;
    if (message.location().url.endsWith('/v1/agent/self')) return;
    errors.push(message.text());
  });
  return errors;
}

// Chromium logs one console error for every document answered with an error
// status. The error-state tests request those statuses on purpose, so they
// expect exactly these messages and nothing else.
const statusText: Record<number, string> = {
  403: 'Forbidden',
  404: 'Not Found',
  503: 'Service Unavailable',
};
function statusError(status: number) {
  return `Failed to load resource: the server responded with a status of ${status} (${statusText[status]})`;
}

test.describe('dynamic host volumes', () => {
  test('lists dynamic host volumes with columns, claim counts and a node filter', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await page.goto('/ui/host-volumes?namespace=team');

    await expect(page).toHaveTitle(/Dynamic host volumes/);
    await expect(page.getByRole('heading', { level: 1, name: 'Dynamic host volumes' })).toBeVisible();
    await expect(page.getByText('CSI volumes are listed under CSI')).toBeVisible();
    for (const name of ['Name', 'State', 'Namespace', 'Node', 'Node pool', 'Plugin', 'Capacity', 'Claims']) {
      await expect(page.getByRole('columnheader', { name, exact: true })).toBeVisible();
    }

    const rows = page.locator('tr[data-volume-id]');
    await expect(rows).toHaveCount(2);
    // Two volumes share a name; each shows only its own claims.
    await expect(rows.nth(0).locator('[data-claims]')).toHaveAttribute('data-claims', '2');
    await expect(rows.nth(1).locator('[data-claims]')).toHaveAttribute('data-claims', '1');
    await expect(rows.nth(0)).toContainText('mkdir');
    await expect(rows.nth(1)).toContainText('None (registered)');
    await expect(rows.nth(0)).toContainText('1.0 GB');

    await page.getByLabel('Node', { exact: true }).selectOption(nodeB);
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toHaveAttribute('data-volume-id', volumeB);
    await expect(page).toHaveURL(new RegExp(`node=${nodeB}`));
    // The selector still offers every node of the unfiltered list.
    await expect(page.locator('#host-volume-node-filter option')).toHaveCount(3);

    expect(browserErrors).toEqual([]);
  });

  test('shows every section of a volume and renders operator values as text', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    page.on('dialog', dialog => {
      browserErrors.push(`unexpected dialog: ${dialog.message()}`);
      void dialog.dismiss();
    });
    await page.goto('/ui/host-volumes?namespace=team');
    await page.locator(`tr[data-volume-id="${volumeA}"]`).getByRole('link', { name: 'data-a' }).click();

    await expect(page).toHaveURL(new RegExp(`/ui/host-volumes/${volumeA}\\?namespace=team`));
    await expect(page.getByRole('heading', { level: 1, name: 'data-a' })).toBeVisible();
    for (const section of [
      'Identity', 'Status', 'Placement', 'Plugin', 'Capacity', 'Requested capabilities',
      'Constraints', 'Attachments', 'Claims', 'Lifecycle',
    ]) {
      await expect(page.getByRole('heading', { level: 2, name: section })).toBeVisible();
    }
    await expect(page.getByText('Diagnostics')).toBeVisible();
    await expect(page.getByText('Plugin reported a healthy mount')).toBeVisible();
    await expect(page.locator(`a[href^="/ui/allocations/${allocation}"]`).first()).toBeVisible();
    await expect(page.locator(`a[href^="/ui/clients/${nodeA}"]`).first()).toBeVisible();
    await expect(page.getByText('claim-1')).toBeVisible();
    await expect(page.getByText('claim-3')).toHaveCount(0);
    await expect(page.getByText('4 Mar 2026 05:06:07 UTC')).toBeVisible();

    // Host path and parameters appear literally; nothing was injected.
    await expect(page.getByText('/srv/<script>window.__pwned = true</script>')).toBeVisible();
    await expect(page.getByText('"><img src=x onerror="window.__pwned = true">')).toBeVisible();
    await expect(page.locator('main script, #main-content script, img[src="x"]')).toHaveCount(0);
    expect(await page.evaluate(() => (window as unknown as { __pwned?: boolean }).__pwned)).toBeUndefined();

    expect(browserErrors).toEqual([]);
  });

  test('shows the empty states', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await page.goto('/ui/host-volumes?namespace=empty-ns');
    await expect(page.getByRole('heading', { name: 'No dynamic host volumes', exact: true })).toBeVisible();

    await page.goto('/ui/host-volumes?namespace=team&node=cccccccc');
    await expect(page.getByRole('heading', { name: 'No dynamic host volumes match this node' })).toBeVisible();

    expect(browserErrors).toEqual([]);
  });

  test('shows a loading indicator while a filter request is pending', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await page.goto('/ui/host-volumes?namespace=slow');
    const indicator = page.locator('#host-volumes-loading');
    const opacity = () => indicator.evaluate(element => getComputedStyle(element).opacity);
    await expect.poll(opacity).toBe('0');

    await page.getByLabel('Node', { exact: true }).selectOption(nodeA);
    await expect.poll(opacity, { timeout: 1000 }).toBe('1');
    await expect(indicator).toContainText('Loading host volumes');
    await expect(page.locator('tr[data-volume-id]')).toHaveCount(1);
    await expect.poll(opacity).toBe('0');

    expect(browserErrors).toEqual([]);
  });

  test('distinguishes an unavailable backend from an empty list', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    const response = await page.goto('/ui/host-volumes?namespace=broken');
    expect(response?.status()).toBe(503);
    await expect(page.getByText('could not load dynamic host volumes')).toBeVisible();
    await expect(page.getByText('No dynamic host volumes')).toHaveCount(0);

    expect(browserErrors).toEqual([statusError(503)]);
  });

  test('shows a forbidden state', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    const response = await page.goto('/ui/host-volumes?namespace=denied');
    expect(response?.status()).toBe(403);
    await expect(page.getByText('Access Denied')).toBeVisible();
    await expect(page.getByText('host-volume-read').first()).toBeVisible();

    expect(browserErrors).toEqual([statusError(403)]);
  });

  test('shows not found, unavailable and forbidden states for a volume', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    const cases: Array<[string, number, string]> = [
      ['missing-volume', 404, 'dynamic host volume was not found'],
      ['broken-volume', 503, 'could not load the dynamic host volume'],
      ['denied-volume', 403, 'Access Denied'],
    ];
    for (const [id, status, text] of cases) {
      const response = await page.goto(`/ui/host-volumes/${id}?namespace=team`);
      expect(response?.status(), id).toBe(status);
      await expect(page.getByText(text).first(), id).toBeVisible();
    }
    const response = await page.goto(`/ui/host-volumes/${volumeA}?namespace=*`);
    expect(response?.status()).toBe(404);
    await expect(page.getByText('single namespace').first()).toBeVisible();

    expect(browserErrors).toEqual([404, 503, 403, 404].map(statusError));
  });

  test('is reachable from the sidebar', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await page.goto('/ui/jobs');
    const link = page.getByRole('link', { name: 'Host Volumes' });
    if (!(await link.isVisible())) await page.getByRole('button', { name: /System/ }).click();
    await link.click();
    await expect(page).toHaveURL(/\/ui\/host-volumes/);
    await expect(page.getByRole('heading', { level: 1, name: 'Dynamic host volumes' })).toBeVisible();

    expect(browserErrors).toEqual([]);
  });
});

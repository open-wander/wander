import { Browser, expect, Page, test } from '@playwright/test';

function captureBrowserErrors(page: Page) {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => {
    if (message.type() === 'error' || message.type() === 'warning') errors.push(message.text());
  });
  return errors;
}

async function openResource(
  page: Page,
  listPath: string,
  rowSelector: string,
  linkSelector: string,
  detailURL: RegExp,
  detailText?: string,
) {
  await page.goto(listPath);
  const row = page.locator(rowSelector).first();
  await expect(row).toBeVisible();
  await row.locator(linkSelector).first().click();
  await expect(page).toHaveURL(detailURL);
  if (detailText) await expect(page.getByText(detailText).first()).toBeVisible();
  await expect(page.locator('body')).not.toContainText(/\bNomad\b/);
}

async function openAllocationView(page: Page, allocationHref: string, view: 'logs' | 'exec' | 'files', content: RegExp) {
  const target = new URL(allocationHref, 'http://wander.test');
  target.pathname = `${target.pathname}/${view}`;
  target.searchParams.set('task', 'fixture');
  const response = await page.goto(`${target.pathname}${target.search}`);
  expect(response?.status()).toBe(200);
  await expect(page).toHaveURL(new RegExp(`/${view}\\?.*task=fixture`));
  await expect(page.locator('body')).toContainText(content);
}

async function openDesktopPage(page: Page, path: string) {
  await page.setViewportSize({ width: 1600, height: 1000 });
  await page.goto(path);
}

async function openCheckedDesktopPage(page: Page, path: string) {
  const browserErrors = captureBrowserErrors(page);
  await openDesktopPage(page, path);
  return browserErrors;
}

async function expectColumnHeaders(page: Page, headings: string[]) {
  for (const heading of headings) {
    await expect(page.getByRole('columnheader', { name: heading, exact: true })).toBeVisible();
  }
}

async function expectURLParams(page: Page, expected: Record<string, string>) {
  await expect.poll(() => {
    const params = new URL(page.url()).searchParams;
    return Object.fromEntries(Object.keys(expected).map(name => [name, params.get(name)]));
  }).toEqual(expected);
}

async function expectNoDocumentOverflow(page: Page) {
  await expect.poll(() => page.evaluate(() => ({
    viewport: window.innerWidth,
    document: Math.max(document.documentElement.scrollWidth, document.body.scrollWidth),
  }))).toEqual(await page.evaluate(() => ({ viewport: window.innerWidth, document: window.innerWidth })));
}

async function openJobTab(page: Page, name: string, expectedText: string) {
  await page.getByRole('tab', { name }).click();
  await expect(page.locator('#job-detail-panel')).toContainText(expectedText);
}

async function expectScopedJobs(page: Page, scope: RegExp, visibleJob: string, hiddenJob: string) {
  await expect(page).toHaveURL(scope);
  await expect(page.locator(`[data-job-id="${visibleJob}"]`)).toBeVisible();
  await expect(page.locator(`[data-job-id="${hiddenJob}"]`)).toHaveCount(0);
}

async function expectTeamNamespaceJobs(page: Page) {
  await expectScopedJobs(page, /namespace=team-a/, 'wander-ui-team-smoke', 'wander-ui-smoke');
}

async function expectTeamJobDetail(page: Page) {
  await expect(page).toHaveURL(/\/ui\/jobs\/wander-ui-team-smoke/);
  await expect(page.getByRole('heading', { name: 'wander-ui-team-smoke' })).toBeVisible();
}

async function submitVariableValue(page: Page, value: string, action: string) {
  await page.getByPlaceholder('Value').fill(value);
  await page.getByRole('button', { name: action }).click();
}

async function newIsolatedNamespacePages(browser: Browser) {
  const baseURL = test.info().project.use.baseURL as string;
  const defaultContext = await browser.newContext({ baseURL });
  const teamContext = await browser.newContext({ baseURL });
  return {
    defaultPage: await defaultContext.newPage(),
    teamPage: await teamContext.newPage(),
    close: async () => {
      await defaultContext.close();
      await teamContext.close();
    },
  };
}

async function selectScope(page: Page, scope: 'namespace' | 'region', value: string) {
  await page.getByLabel(`Select ${scope}`).click();
  await page.getByText(value, { exact: true }).click();
}

async function openDialog(page: Page, triggerName: string, dialogName: string) {
  await page.getByRole('button', { name: triggerName, exact: true }).click();
  const dialog = page.getByRole('dialog', { name: dialogName });
  await expect(dialog).toBeVisible();
  return dialog;
}

async function exerciseAgentMonitor(page: Page, route: RegExp, level: string, resume = false) {
  await page.getByRole('link', { name: 'Monitor', exact: true }).click();
  await expect(page).toHaveURL(route);
  await expect(page.locator('[data-agent-monitor]')).toBeVisible();

  const monitorState = page.locator('[data-monitor-state]');
  const monitorToggle = page.locator('[data-monitor-toggle]');
  await expect(monitorState).toHaveText('connected');
  await expect(page.locator('[data-monitor-level]')).toHaveValue('info');
  await page.locator('[data-monitor-level]').selectOption(level);
  await expect(page).toHaveURL(new RegExp(`level=${level}`));
  await expect(monitorState).toHaveText('connected');
  await monitorToggle.click();
  await expect(monitorState).toHaveText('paused');

  if (resume) {
    await expect(monitorToggle).toHaveText('Resume');
    await monitorToggle.click();
    await expect(monitorState).toHaveText('connected');
  }
}

async function expectJobLifecycleFailure(
  page: Page,
  jobID: string,
  triggerName: string,
  dialogName: string,
  endpoint: string,
) {
  const message = `Simulated ${triggerName.toLowerCase()} failure`;
  await page.route(`**/api/jobs/${jobID}/${endpoint}*`, async route => {
    await route.fulfill({
      status: 502,
      headers: { 'HX-Trigger': JSON.stringify({ showToast: { message, type: 'error' } }) },
      body: '',
    });
  }, { times: 1 });
  await page.goto(`/ui/jobs/${jobID}?namespace=default&region=global`);
  const dialog = await openDialog(page, triggerName, dialogName);
  const response = page.waitForResponse(candidate => candidate.url().includes(`/api/jobs/${jobID}/${endpoint}`));
  await dialog.getByRole('button', { name: dialogName, exact: true }).click();
  expect((await response).status()).toBe(502);
  await expect(dialog).toBeVisible();
  const notification = page.getByRole('alert').filter({ hasText: message }).first();
  await expect(notification).toBeVisible();
  await expect(notification).toHaveClass(/bg-red-50/);
}

test.describe('Wander release smoke', () => {
  test('opens CSI volume claims when the cluster has a volume', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/csi/volumes?region=global');
    const volumeRows = page.locator('[data-volume-id]');
    test.skip((await volumeRows.count()) === 0, 'the disposable cluster has no CSI volume fixture');

    const volumeID = await volumeRows.first().getAttribute('data-volume-id');
    await volumeRows.first().locator('a[href^="/ui/csi/volumes/"]').click();
    await expect(page).toHaveURL(/\/ui\/csi\/volumes\/[^/]+/);
    await expect(page.getByText(volumeID ?? '', { exact: true }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Capability Constraints' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Read-only claims' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Read/write claims' })).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('opens CSI plugin details when the cluster has a plugin', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/csi/plugins?region=global');
    const pluginRows = page.locator('[data-plugin-id]');
    test.skip((await pluginRows.count()) === 0, 'the disposable cluster has no CSI plugin fixture');

    const pluginID = await pluginRows.first().getAttribute('data-plugin-id');
    await pluginRows.first().locator('a[href^="/ui/csi/plugins/"]').click();
    await expect(page).toHaveURL(/\/ui\/csi\/plugins\/[^/]+/);
    await expect(page.getByRole('heading', { name: pluginID ?? '' })).toBeVisible();
    await page.getByRole('link', { name: 'Allocations', exact: true }).click();
    await expect(page).toHaveURL(/\/allocations/);
    await expect(page.getByRole('heading', { name: 'Plugin Allocations' })).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('dispatches and purges a parameterized child job', async ({ page }) => {
    await openDesktopPage(page, '/ui/jobs/wander-ui-parameterized?namespace=dispatch-test&region=global');

    const dispatchDialog = await openDialog(page, 'Dispatch', 'Dispatch Job');
    await dispatchDialog.locator('input[name="meta.environment"]').fill('test');
    await dispatchDialog.locator('input[name="meta.version"]').fill('v1');
    await dispatchDialog.locator('textarea[name="payload"]').fill('browser-dispatch-payload');
    await dispatchDialog.getByRole('button', { name: 'Dispatch Job' }).click();

    await expect(page).toHaveURL(/\/ui\/jobs\/wander-ui-parameterized(?:%2F|\/)dispatch-/);
    const dispatchedURL = new URL(page.url());
    expect(dispatchedURL.searchParams.get('namespace')).toBe('dispatch-test');
    expect(dispatchedURL.searchParams.get('region')).toBe('global');
    await expect(page.getByRole('button', { name: 'Purge', exact: true })).toBeVisible();

    const purgeDialog = await openDialog(page, 'Purge', 'Purge Job');
    await expect(purgeDialog).toContainText('cannot be undone');
    await purgeDialog.getByRole('button', { name: 'Purge Job' }).click();

    await expect(page).toHaveURL(/\/ui\/jobs\?/);
    await expect(page.locator('[data-job-id^="wander-ui-parameterized/dispatch-"]')).toHaveCount(0);
  });

  test('renders the live job-kind matrix and forces a periodic launch', async ({ page }) => {
    const browserErrors = await openCheckedDesktopPage(page, '/ui/jobs?namespace=default&per_page=25');
    const kinds = [
      { id: 'wander-ui-smoke', type: 'service' },
      { id: 'wander-ui-batch', type: 'batch' },
      { id: 'wander-ui-system', type: 'system' },
      { id: 'wander-ui-sysbatch', type: 'sysbatch' },
      { id: 'wander-ui-periodic', type: 'batch' },
    ];

    for (const kind of kinds) {
      const row = page.locator(`[data-job-id="${kind.id}"]`);
      await expect(row).toBeVisible();
      await expect(row).toContainText(kind.type);
    }

    for (const id of ['wander-ui-smoke', 'wander-ui-system']) {
      await page.goto(`/ui/jobs/${id}`);
      await expect(page.getByRole('button', { name: 'Stop', exact: true })).toBeVisible();
    }

    for (const id of ['wander-ui-batch', 'wander-ui-sysbatch']) {
      await expect.poll(async () => {
        const response = await page.request.get(`/v1/job/${id}`);
        return (await response.json()).Status;
      }).toBe('dead');
      await page.goto(`/ui/jobs/${id}`);
      await expect(page.getByRole('button', { name: 'Start', exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Purge', exact: true })).toBeVisible();
    }

    await page.goto('/ui/jobs/wander-ui-periodic');
    await expect(page.locator('[data-overview-child-jobs]')).toBeVisible();
    await expect(page.locator('[data-overview-scheduling]')).toContainText('Periodic Schedule');
    const forceDialog = await openDialog(page, 'Force Launch', 'Force Launch');
    const forceResponse = page.waitForResponse(response => response.url().includes('/api/jobs/wander-ui-periodic/periodic/force'));
    await forceDialog.getByRole('button', { name: 'Force Launch' }).click();
    expect((await forceResponse).status()).toBe(200);
    await expect(forceDialog).toBeHidden();

    await page.goto('/ui/jobs/wander-ui-parameterized?namespace=dispatch-test&region=global');
    await expect(page.getByRole('button', { name: 'Dispatch', exact: true })).toBeVisible();
    await expect(page.locator('[data-overview-child-jobs]')).toBeVisible();
    await expect(page.locator('[data-overview-scheduling]')).toContainText('Dispatch Payload');
    expect(browserErrors).toEqual([]);
  });

  test('keeps lifecycle confirmations open and reports safe failures', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await expect.poll(async () => {
      const response = await page.request.get('/v1/job/wander-ui-batch');
      return (await response.json()).Status;
    }).toBe('dead');

    await expectJobLifecycleFailure(page, 'wander-ui-batch', 'Start', 'Start Job', 'start');
    await expectJobLifecycleFailure(page, 'wander-ui-smoke', 'Stop', 'Stop Job', 'stop');
    await expectJobLifecycleFailure(page, 'wander-ui-smoke', 'Run', 'Run Job', 'run');
    await expectJobLifecycleFailure(page, 'wander-ui-periodic', 'Force Launch', 'Force Launch', 'periodic/force');
    const unexpectedErrors = browserErrors.filter(error =>
      !error.includes('502 (Bad Gateway)') && !error.includes('Response Status Error Code 502'));
    expect(unexpectedErrors).toEqual([]);
  });

  test('renders and filters the scoped allocation matrix', async ({ page }) => {
    const browserErrors = await openCheckedDesktopPage(page, '/ui/allocations?namespace=default&region=global&per_page=10');
    await expectColumnHeaders(page, ['Short ID', 'Job', 'Type', 'Task Group', 'Job Version', 'Client', 'Status', 'Scheduling', 'Created', 'Logs']);

    const smokeAllocation = page.locator('[data-alloc-id]').filter({ hasText: 'wander-ui-smoke' }).first();
    await expect(smokeAllocation).toContainText('service');
    await expect(smokeAllocation).toContainText('web');
    await expect(smokeAllocation).toContainText('running');
    const logsURL = new URL((await smokeAllocation.getByRole('link', { name: 'View Logs' }).getAttribute('href'))!, 'http://wander.test');
    expect(logsURL.searchParams.get('namespace')).toBe('default');
    expect(logsURL.searchParams.get('region')).toBe('global');

    const filters = page.locator('#allocation-filters');
    await filters.locator('select[name="type"]').selectOption('service');
    await filters.locator('select[name="task_group"]').selectOption('web');
    await filters.locator('select[name="status"]').selectOption('running');
    await filters.locator('select[name="scheduling"]').selectOption('run');
    await filters.getByRole('button', { name: 'Apply filters' }).click();
    await expectURLParams(page, { namespace: 'default', region: 'global', type: 'service', task_group: 'web', status: 'running', scheduling: 'run' });
    await expect(page.locator('[data-alloc-id]').filter({ hasText: 'wander-ui-smoke' }).first()).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('switches to and navigates within a federated region', async ({ page }) => {
    test.skip(process.env.WANDER_UI_E2E_FEDERATED !== '1', 'requires the disposable federated fixture');
    const browserErrors = captureBrowserErrors(page);

    await page.goto('/ui/jobs?namespace=default&region=global');
    await expect(page.locator('[data-job-id="wander-ui-smoke"]')).toBeVisible();
    await expect(page.locator('[data-job-id="wander-ui-region-smoke"]')).toHaveCount(0);

    await selectScope(page, 'region', 'region-a');
    await expectScopedJobs(page, /region=region-a/, 'wander-ui-region-smoke', 'wander-ui-smoke');

    await page.locator('[data-job-id="wander-ui-region-smoke"] a').first().click();
    await expect(page).toHaveURL(/\/ui\/jobs\/wander-ui-region-smoke/);
    await expect(page).toHaveURL(/region=region-a/);
    await expect(page.getByRole('heading', { name: 'wander-ui-region-smoke' })).toBeVisible();

    await page.getByRole('link', { name: 'Jobs', exact: true }).click();
    await expect(page).toHaveURL(/region=region-a/);
    await expect(page.locator('[data-job-id="wander-ui-region-smoke"]')).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('promotes a canary deployment in a federated region without losing scope', async ({ page }) => {
    test.skip(process.env.WANDER_UI_E2E_FEDERATED !== '1', 'requires the disposable federated fixture');
    const deploymentID = process.env.WANDER_UI_E2E_REGION_DEPLOYMENT_ID;
    expect(deploymentID).toBeTruthy();
    const browserErrors = captureBrowserErrors(page);

    await openDesktopPage(page, `/ui/deployments/${deploymentID}?namespace=default&region=region-a`);
    await expectURLParams(page, { namespace: 'default', region: 'region-a' });
    await expect(page.getByRole('button', { name: 'Promote canaries' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Fail deployment' })).toBeVisible();

    const jobLink = page.getByRole('link', { name: 'wander-ui-region-deployment' });
    const jobURL = new URL((await jobLink.getAttribute('href'))!, 'http://wander.test');
    expect(jobURL.searchParams.get('namespace')).toBe('default');
    expect(jobURL.searchParams.get('region')).toBe('region-a');

    page.once('dialog', dialog => dialog.accept());
    const [promotion] = await Promise.all([
      page.waitForResponse(response => response.url().includes(`/api/deployments/${deploymentID}/promote`)),
      page.getByRole('button', { name: 'Promote canaries' }).click(),
    ]);
    expect(promotion.status()).toBe(200);
    await expectURLParams(page, { namespace: 'default', region: 'region-a' });
    expect(browserErrors).toEqual([]);
  });

  test('isolates namespace selection between browser sessions', async ({ browser }) => {
    const { defaultPage, teamPage, close } = await newIsolatedNamespacePages(browser);

    try {
      await defaultPage.goto('/ui/jobs?namespace=default');
      await teamPage.goto('/ui/jobs?namespace=default');

      await expect(defaultPage.locator('[data-job-id="wander-ui-smoke"]')).toBeVisible();
      await expect(defaultPage.locator('[data-job-id="wander-ui-team-smoke"]')).toHaveCount(0);

      await selectScope(teamPage, 'namespace', 'team-a');
      await expectTeamNamespaceJobs(teamPage);

      await teamPage.locator('[data-job-id="wander-ui-team-smoke"] a').click();
      await expectTeamJobDetail(teamPage);

      await teamPage.getByRole('link', { name: 'Jobs', exact: true }).click();
      await expectTeamNamespaceJobs(teamPage);

      await defaultPage.reload();
      await expectScopedJobs(defaultPage, /namespace=default/, 'wander-ui-smoke', 'wander-ui-team-smoke');
    } finally {
      await close();
    }
  });

  test('retains direct URL scope through navigation and HTMX actions', async ({ browser }) => {
    const baseURL = test.info().project.use.baseURL as string;
    const context = await browser.newContext({ baseURL });
    const page = await context.newPage();

    try {
      await page.goto('/ui/jobs?namespace=team-a&region=global');
      await expectTeamNamespaceJobs(page);

      const jobRow = page.locator('[data-job-id="wander-ui-team-smoke"]');
      const jobHref = await jobRow.locator('a').first().getAttribute('href');
      expect(jobHref).toContain('namespace=team-a');
      expect(jobHref).toContain('region=global');
      const scopedURLs = await page.evaluate(() => {
        const scopeURL = (window as typeof window & { wanderScopeURL: (url: string) => string }).wanderScopeURL;
        return {
          explicit: scopeURL('/ui/jobs?namespace=explicit'),
          external: scopeURL('https://example.com/ui/jobs'),
          staticAsset: scopeURL('/ui/static/css/output.css'),
        };
      });
      expect(scopedURLs.explicit).toContain('namespace=explicit');
      expect(scopedURLs.explicit).not.toContain('namespace=team-a');
      expect(scopedURLs.explicit).toContain('region=global');
      expect(scopedURLs.external).toBe('https://example.com/ui/jobs');
      expect(scopedURLs.staticAsset).toBe('/ui/static/css/output.css');
      await jobRow.locator('a').first().click();
      await expectTeamJobDetail(page);
      await expect(page).toHaveURL(/namespace=team-a/);
      await expect(page).toHaveURL(/region=global/);

      const stopAction = page.locator('[aria-labelledby="modal-title-stop-job-modal"] [hx-post]');
      await expect(stopAction).toHaveAttribute('hx-post', /namespace=team-a/);
      await expect(stopAction).toHaveAttribute('hx-post', /region=global/);

      await page.getByRole('tab', { name: 'Definition' }).click();
      await expect(page.locator('.wander-editor-wrapper')).toContainText('job "wander-ui-team-smoke"');
      await page.getByRole('link', { name: 'Jobs', exact: true }).click();
      await expect(page).toHaveURL(/namespace=team-a/);
      await expectTeamNamespaceJobs(page);
    } finally {
      await context.close();
    }
  });

  test('uses live namespaces for an isolated variable workflow', async ({ browser }) => {
    const variablePath = `wander-ui/browser-variable-${Date.now()}`;
    const { defaultPage, teamPage, close } = await newIsolatedNamespacePages(browser);

    try {
      await openDesktopPage(teamPage, '/ui/variables?namespace=default');
      await selectScope(teamPage, 'namespace', 'team-a');
      await teamPage.getByRole('link', { name: 'Create Variable' }).click();

      const namespaceSelect = teamPage.getByLabel('Namespace', { exact: true });
      await expect(namespaceSelect).toHaveValue('team-a');
      await expect(namespaceSelect.locator('option[value="team-a"]')).toHaveCount(1);
      await expect(namespaceSelect.locator('option[value="default"]')).toHaveCount(1);
      await expect(namespaceSelect.locator('option[value="production"], option[value="staging"], option[value="development"]')).toHaveCount(0);

      await teamPage.getByLabel('Path').fill(variablePath);
      await teamPage.getByPlaceholder('Key').fill('answer');
      await submitVariableValue(teamPage, 'forty-two', 'Create Variable');

      await expect(teamPage).toHaveURL(/namespace=team-a/);
      await expect(teamPage.getByRole('heading', { name: variablePath })).toBeVisible();
      await expect(teamPage.getByPlaceholder('Value')).toHaveValue('forty-two');

      await openDesktopPage(defaultPage, '/ui/variables?namespace=default');
      await expect(defaultPage.getByText(variablePath, { exact: true })).toHaveCount(0);

      await openDesktopPage(teamPage, '/ui/variables?namespace=team-a&region=global');
      await teamPage.getByRole('link', { name: 'Tree', exact: true }).click();
      await expect(teamPage).toHaveURL(/namespace=team-a&region=global&view=tree/);
      const treeRecord = teamPage.locator(`[data-variable-tree-path="${variablePath}"] a`);
      await expect(treeRecord).toHaveAttribute('href', /namespace=team-a&region=global/);
      await treeRecord.click();
      await expect(teamPage.getByRole('heading', { name: variablePath })).toBeVisible();
      const firstValue = teamPage.getByPlaceholder('Value').first();
      await expect(firstValue).toHaveAttribute('type', 'password');
      await teamPage.getByRole('button', { name: 'Reveal value' }).first().click();
      await expect(firstValue).toHaveAttribute('type', 'text');
      await firstValue.fill('draft');
      let unsavedPrompt = '';
      teamPage.once('dialog', async dialog => {
        unsavedPrompt = dialog.message();
        await dialog.dismiss();
      });
      await teamPage.getByRole('link', { name: 'Jobs', exact: true }).first().click();
      expect(unsavedPrompt).toContain('Discard unsaved variable changes?');
      await expect(teamPage.getByRole('heading', { name: variablePath })).toBeVisible();
      await teamPage.getByRole('button', { name: 'Reset' }).click();

      await teamPage.getByRole('button', { name: 'JSON', exact: true }).click();
      const jsonEditor = teamPage.getByLabel('Variable items JSON');
      await expect(jsonEditor).toHaveValue(/"answer": "forty-two"/);
      await jsonEditor.fill('{"answer":"updated","mode":"json"}');
      await teamPage.getByRole('button', { name: 'Save Variable' }).click();
      await expect(teamPage.getByText('Variable saved successfully')).toBeVisible();
      await teamPage.reload();
      await teamPage.getByRole('button', { name: 'JSON', exact: true }).click();
      await expect(teamPage.getByLabel('Variable items JSON')).toHaveValue(/"mode": "json"/);
      await teamPage.getByRole('button', { name: 'List', exact: true }).click();
      await expect(teamPage.getByPlaceholder('Value').first()).toHaveValue('updated');

      const deleteModal = await openDialog(teamPage, 'Delete', 'Delete Variable');
      await deleteModal.getByRole('button', { name: 'Delete Variable' }).click();
      await expect(teamPage).toHaveURL(/\/ui\/variables\?namespace=team-a&region=global$/);
      await expect(teamPage.getByText(variablePath, { exact: true })).toHaveCount(0);
    } finally {
      await teamPage.request.delete(`/ui/variables/${variablePath}?namespace=team-a`);
      await close();
    }
  });

  test('creates, selects, updates, and deletes a stored job template', async ({ page }) => {
    const browserErrors = await openCheckedDesktopPage(page, '/ui/jobs/run/templates?namespace=default');
    const templateName = `browser-template-${Date.now()}`;
    const initialSource = `job "${templateName}" {\n  datacenters = ["dc1"]\n}`;
    const updatedSource = `${initialSource.slice(0, -1)}\n  type = "service"\n}`;

    try {
      await page.getByRole('link', { name: 'Create Template' }).first().click();
      await page.getByRole('textbox', { name: 'Name', exact: true }).fill(templateName);
      await page.getByLabel('HCL job definition').fill(initialSource);
      await page.getByRole('button', { name: 'Create Template' }).click();

      await expect(page).toHaveURL(new RegExp(`/ui/jobs/run\\?.*template=${templateName}`));
      await expect(page.locator('textarea[x-ref="initialSource"]')).toHaveValue(initialSource);

      await page.goto('/ui/jobs/run/templates/manage?namespace=default');
      let card = page.locator('div[x-data="{ editing: false }"]').filter({ hasText: templateName });
      await card.getByRole('button', { name: 'Edit' }).click();
      await card.getByLabel('HCL job definition').fill(updatedSource);
      await card.getByRole('button', { name: 'Save Template' }).click();
      await expect(page).toHaveURL(/\/ui\/jobs\/run\/templates\/manage\?namespace=default/);
      await page.waitForLoadState('networkidle');

      await page.goto(`/ui/jobs/run?namespace=default&template=${templateName}`);
      await expect(page.locator('textarea[x-ref="initialSource"]')).toHaveValue(updatedSource);

      await page.goto('/ui/jobs/run/templates/manage?namespace=default');
      card = page.locator('div[x-data="{ editing: false }"]').filter({ hasText: templateName });
      page.once('dialog', dialog => dialog.accept());
      await card.getByRole('button', { name: 'Delete' }).click();
      await expect(page.getByRole('heading', { name: templateName, exact: true })).toHaveCount(0);
      expect(browserErrors).toEqual([]);
    } finally {
      await page.request.delete(`/ui/jobs/run/templates/manage/${templateName}?namespace=default`);
    }
  });

  test('renders live cluster navigation and resources', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await page.goto('/ui/');
    await expect(page).toHaveTitle(/Wander/);
    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
    await expect(page.locator('#global-connection-status')).toContainText('Connected');
    const searchPadding = await page.locator('#global-search').evaluate(element => {
      const style = window.getComputedStyle(element);
      return { left: style.paddingLeft, right: style.paddingRight };
    });
    expect(searchPadding).toEqual({ left: '40px', right: '80px' });
    const headerLayout = await page.locator('#header').evaluate(header => {
      const breadcrumb = header.querySelector<HTMLElement>('nav[aria-label="Breadcrumb"]')!.getBoundingClientRect();
      const search = header.querySelector<HTMLElement>('[data-global-search]')!.getBoundingClientRect();
      return { breadcrumbBottom: breadcrumb.bottom, searchTop: search.top };
    });
    expect(headerLayout.searchTop).toBeGreaterThanOrEqual(headerLayout.breadcrumbBottom);
    await expect(page.locator('footer')).toContainText('v0.1.1-dev');
    const shellLayout = await page.locator('body').evaluate(() => {
      const main = document.querySelector<HTMLElement>('main')!.getBoundingClientRect();
      const footer = document.querySelector<HTMLElement>('footer')!.getBoundingClientRect();
      return {
        mainBottom: main.bottom,
        footerTop: footer.top,
        footerBottom: footer.bottom,
        footerLeft: footer.left,
        footerRight: footer.right,
        viewportHeight: window.innerHeight,
        viewportWidth: window.innerWidth,
      };
    });
    expect(shellLayout.footerTop).toBeGreaterThanOrEqual(shellLayout.mainBottom);
    expect(shellLayout.footerBottom).toBe(shellLayout.viewportHeight);
    expect(shellLayout.footerLeft).toBe(0);
    expect(shellLayout.footerRight).toBe(shellLayout.viewportWidth);
    await expect(page.getByText('v0.1.1-dev', { exact: true })).toHaveCount(1);

    await openResource(page, '/ui/jobs', '[data-job-id="wander-ui-smoke"]', 'a[href^="/ui/jobs/wander-ui-smoke"]', /\/ui\/jobs\/wander-ui-smoke/, 'wander-ui-smoke');
    await expect(page.locator('[data-overview-allocation-summary]')).toContainText('1');
    await expect(page.locator('[data-overview-recent-allocations]')).toContainText('web');
    await expect(page.locator('[data-overview-placement-status]')).toContainText('no placement failures');
    await expect(page.locator('[data-overview-scheduling]')).toContainText('default');
    await expect(page.locator('body')).toContainText('test-purpose');
    await expect(page.locator('body')).toContainText('job-overview');
    await openResource(page, '/ui/allocations', '[data-alloc-id]:has-text("wander-ui-smoke")', 'a[href^="/ui/allocations/"]', /\/ui\/allocations\/[0-9a-f-]+/, 'fixture');
    await page.goto('/ui/clients?namespace=default&region=global');
    for (const heading of ['State', 'Node Pool', 'Class', 'DC', 'Version', 'Volumes', 'Running Allocs']) {
      await expect(page.getByRole('columnheader', { name: heading, exact: true })).toBeVisible();
    }
    await expect(page.getByLabel('Rows per page')).toHaveValue('25');
    await page.getByLabel('Node pool').selectOption('default');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expect(page).toHaveURL(/nodepool=default/);
    await expect(page).toHaveURL(/namespace=default/);
    await expect(page).toHaveURL(/region=global/);
    const clientRow = page.locator('[data-client-id]').first();
    await expect(clientRow).toContainText('CPU allocated');
    await expect(clientRow).toContainText('Memory allocated');
    await clientRow.locator('a[href^="/ui/clients/"]').first().click();
    await expect(page).toHaveURL(/\/ui\/clients\/[0-9a-f-]+/);
    await expect(page).toHaveURL(/namespace=default/);
    await expect(page).toHaveURL(/region=global/);
    await expect(page.getByRole('heading', { name: 'Host Volumes' })).toBeVisible();
    await expect(page.getByText('No host volumes configured')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Current Host Statistics' })).toBeVisible();
    await expect(page.getByText('Current client sample; historical samples are not provided by this API')).toBeVisible();
    await expect(page.getByText('Last 24 hours', { exact: true })).toHaveCount(0);
    const drainDialog = await openDialog(page, 'Drain Node', 'Drain Node');
    await expect(drainDialog.getByLabel('Drain strategy')).toHaveValue('default');
    await expect(drainDialog.getByLabel('Drain strategy').locator('option')).toHaveCount(4);
    await expect(drainDialog.getByLabel('Custom deadline')).toBeVisible();
    await expect(drainDialog.getByLabel('Ignore system jobs (system jobs will not be drained)')).toBeVisible();
    await expect(drainDialog.getByLabel('Operator message')).toBeVisible();
    await drainDialog.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(drainDialog).toBeHidden();
    await expect(page.getByRole('heading', { name: 'Node Metadata' })).toBeVisible();
    const metadataKey = 'wander.browser';
    const clientURL = new URL(page.url());
    const metadataURL = `${clientURL.pathname}/metadata${clientURL.search}`;
    try {
      await page.getByLabel('Key', { exact: true }).fill(metadataKey);
      await page.getByLabel('Value', { exact: true }).fill('release-smoke');
      await expect(page.getByRole('button', { name: 'Save metadata' })).toBeVisible();
      const setResponse = await page.request.post(metadataURL, {
        form: { operation: 'set', key: metadataKey, value: 'release-smoke' },
      });
      expect(setResponse.ok()).toBeTruthy();
      await page.reload();
      const metadataRow = page.locator(`[data-meta-key="${metadataKey}"]`);
      await expect(metadataRow).toBeVisible();
      await expect(metadataRow.getByRole('button', { name: 'Unset' })).toBeVisible();
      const unsetResponse = await page.request.post(metadataURL, {
        form: { operation: 'unset', key: metadataKey },
      });
      expect(unsetResponse.ok()).toBeTruthy();
      await page.reload();
      await expect(page.getByText(metadataKey, { exact: true })).toHaveCount(0);
    } finally {
      await page.request.post(metadataURL, {
        form: { operation: 'unset', key: metadataKey },
      });
    }
    await page.getByRole('tab', { name: /Allocations/ }).click();
    const clientAllocationFilters = page.locator('form').filter({ has: page.getByLabel('Task group') });
    await clientAllocationFilters.getByLabel('Status').selectOption('running');
    const filteredClientURL = new URL(page.url());
    filteredClientURL.searchParams.set('tab', 'allocations');
    filteredClientURL.searchParams.set('allocation_status', 'running');
    await page.goto(`${filteredClientURL.pathname}${filteredClientURL.search}`);
    await expect(page).toHaveURL(/tab=allocations/);
    await expect(page).toHaveURL(/allocation_status=running/);
    await expect(page).toHaveURL(/namespace=default/);
    await expect(page).toHaveURL(/region=global/);
    await expect(page.getByRole('tab', { name: /Allocations/ })).toHaveAttribute('aria-selected', 'true');
    const scopedAllocationLink = page.locator('[data-alloc-id] a[href^="/ui/allocations/"]').first();
    await expect(scopedAllocationLink).toHaveAttribute('href', /namespace=default/);
    await expect(scopedAllocationLink).toHaveAttribute('href', /region=global/);

    await page.goto('/ui/servers');
    await expect(page.locator('tbody tr').first()).toBeVisible();

    await openResource(
      page,
      '/ui/services',
      'tbody tr',
      'a[href^="/ui/services/wander-ui-http"]',
      /\/ui\/services\/wander-ui-http/,
      'wander-ui-http',
    );
    await expect(page.getByRole('link', { name: 'wander-ui-smoke', exact: true }).first()).toBeVisible();

    await page.goto('/ui/jobs/wander-ui-smoke');
    await openJobTab(page, 'Services', 'wander-ui-http');
    await openJobTab(page, 'Clients', 'Clients observed in this job');
    await expect(page.locator('#job-detail-panel tbody tr')).toHaveCount(1);
    await openJobTab(page, 'Deployments', 'Time unavailable');
    await expect(page.locator('#job-detail-panel a[href^="/ui/deployments/"]').first()).toBeVisible();
    await openJobTab(page, 'Evaluations', 'Priority: 50');
    await expect(page.locator('#job-detail-panel a[href^="/ui/evaluations/"]').first()).toBeVisible();
    await openJobTab(page, 'Variables', 'browser-smoke');
    await expect(page.locator('#job-detail-panel a[href^="/ui/variables/"]').first()).toBeVisible();
    await expect(page.locator('#job-detail-panel')).not.toContainText('job-linked-variable');

    await page.goto('/ui/topology');
    await expect(page.getByLabel('Metrics')).toBeVisible();
    await expect(page.getByLabel('Allocation Status')).toBeVisible();
    await expect(page.getByLabel('Node Pool')).toBeVisible();
    await expect(page.getByLabel('Datacenter')).toBeVisible();
    await expect(page.getByLabel('Class')).toBeVisible();
    await expect(page.getByLabel('State')).toBeVisible();
    await expect(page.getByLabel('Version')).toBeVisible();
    await page.getByLabel('Metrics').selectOption('memory');
    await page.getByLabel('Allocation Status').selectOption('running');
    await page.getByLabel('Node Pool').selectOption('default');
    await page.getByLabel('Datacenter').selectOption('dc1');
    await page.getByLabel('State').selectOption('ready');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expectURLParams(page, { metric: 'memory', allocation_status: 'running', node_pool: 'default', datacenter: 'dc1', state: 'ready' });
    const topologyNode = page.locator('[data-topology-node-id]').first();
    await expect(topologyNode).toContainText('Allocations');
    await expect(topologyNode).toContainText('Memory allocated');
    await expect(topologyNode).not.toContainText('CPU allocated');
    await topologyNode.getByRole('link', { name: 'Select client' }).click();
    await expect(page.locator('[data-topology-node-id][data-selected="true"]')).toHaveCount(1);
    const topologyAllocation = page.locator('[data-topology-allocation-id]').first();
    await expect(topologyAllocation).toBeVisible();
    await topologyAllocation.getByRole('link', { name: 'Select allocation' }).click();
    await expect(page.locator('[data-topology-allocation-id][data-selected="true"]')).toHaveCount(1);
    await expect(page.locator('body')).toContainText('Region: global');
    await expect(page.locator('body')).not.toContainText('Last heartbeat: Unknown');

    for (const route of [
      '/ui/variables',
      '/ui/csi/volumes',
      '/ui/csi/plugins',
      '/ui/deployments',
      '/ui/evaluations',
      '/ui/settings',
      '/ui/jobs/run',
    ]) {
      const response = await page.goto(route);
      expect(response?.status(), route).toBe(200);
      await expect(page.locator('body'), route).not.toContainText(/\bNomad\b/);
    }
    const variablesInput = page.getByLabel('HCL Variables');
    await expect(variablesInput).toBeVisible();
    await variablesInput.fill('image = "busybox:stable"');
    await page.getByRole('button', { name: 'JSON', exact: true }).click();
    await expect(variablesInput).toBeHidden();
    await page.getByRole('button', { name: 'HCL', exact: true }).click();
    await expect(variablesInput).toHaveValue('image = "busybox:stable"');
    expect(browserErrors).toEqual([]);
  });

  test('searches live scoped resources and follows a result', async ({ page }) => {
    const browserErrors = await openCheckedDesktopPage(page, '/ui/jobs?namespace=default&region=global');
    const search = page.locator('#global-search');
    await expect(search).toHaveAttribute('name', 'q');
    await search.pressSequentially('wander-ui-smoke');
    const jobResult = page.locator('a[href^="/ui/jobs/wander-ui-smoke?"]').first();
    await expect(jobResult).toHaveAttribute('href', /namespace=default&region=global/);
    await jobResult.click();
    await expect(page.getByRole('heading', { name: 'wander-ui-smoke', exact: true })).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('paginates the live evaluations list without losing scope', async ({ page }) => {
    const browserErrors = await openCheckedDesktopPage(page, '/ui/evaluations?namespace=default&region=global&per_page=1');
    await expect(page.getByText(/Showing\s+1\s+to\s+1\s+of\s+\d+\s+results/)).toBeVisible();
    await page.getByRole('link', { name: 'Next' }).last().click();
    await expect(page).toHaveURL(/page=2/);
    await expect(page).toHaveURL(/namespace=default/);
    await expect(page).toHaveURL(/region=global/);
    await expect(page.getByText(/Showing\s+2\s+to\s+2\s+of\s+\d+\s+results/)).toBeVisible();
    await expect(page.locator('#evaluations-list-container [aria-current="page"]')).toHaveText('2');
    expect(browserErrors).toEqual([]);
  });

  test('reviews and submits an existing job edit with retained variables', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    const jobID = `wander-ui-edit-${Date.now()}`;
    const initialSource = `variable "marker" {\n  type = string\n}\n\njob "${jobID}" {\n  datacenters = ["dc1"]\n  type = "batch"\n\n  meta {\n    edit-marker = var.marker\n  }\n\n  group "work" {\n    task "complete" {\n      driver = "raw_exec"\n      config {\n        command = "/usr/bin/true"\n      }\n    }\n  }\n}`;
    const updatedSource = initialSource.replace('type = "batch"', 'type = "batch"\n  priority = 60');

    try {
      const create = await page.request.post('/ui/jobs/_new/run?namespace=default&region=global', {
        data: {
          definition: initialSource,
          format: 'hcl',
          variables: 'marker = "initial"',
        },
      });
      expect(create.ok(), await create.text()).toBeTruthy();

      await openDesktopPage(page, `/ui/jobs/${jobID}?namespace=default&region=global`);
      await openJobTab(page, 'Definition', 'Job Definition');
      await expect(page.getByRole('button', { name: 'Job Spec', exact: true })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Full Definition', exact: true })).toBeVisible();
      await expect(page.getByLabel('HCL Variables')).toHaveValue('marker = "initial"');
      await expect(page.getByText('Version:').locator('..')).toContainText('0');

      await page.getByRole('button', { name: 'Edit', exact: true }).click();
      await page.locator('.wander-textarea').fill(updatedSource);
      await page.getByLabel('HCL Variables').fill('marker = "reviewed"');
      await page.getByRole('button', { name: 'Review changes', exact: true }).click();
      await expect(page.getByRole('heading', { name: 'Plan Result' })).toBeVisible();
      await expect(page.getByText('Changes', { exact: true })).toBeVisible();

      await page.getByRole('button', { name: 'Run Job', exact: true }).first().click();
      await expect(page.getByText('Are you sure you want to run this job?')).toBeVisible();
      await page.getByRole('button', { name: 'Run Job', exact: true }).click();

      await expect(page).toHaveURL(new RegExp(`/ui/jobs/${jobID}/definition`));
      await expectURLParams(page, { namespace: 'default', region: 'global' });
      await expect(page.getByText('Version:').locator('..')).toContainText('1');
      await expect(page.getByLabel('HCL Variables')).toHaveValue('marker = "reviewed"');
      await expect(page.locator('.wander-editor-wrapper')).toContainText('priority = 60');

      await openJobTab(page, 'Versions', 'Version History');
      await expect(page.getByRole('columnheader', { name: 'Changes' })).toBeVisible();
      await expect(page.getByText(/\d+ changes/).first()).toBeVisible();
      await page.getByRole('button', { name: 'Diff', exact: true }).first().click();
      const diffDialog = page.getByRole('dialog', { name: 'Version 1 vs Version 0' });
      await expect(diffDialog).toBeVisible();
      await expect(diffDialog.getByText('Priority', { exact: true })).toBeVisible();
      await expect(diffDialog.getByRole('columnheader', { name: 'Previous value' })).toBeVisible();
      await expect(diffDialog.getByRole('columnheader', { name: 'New value' })).toBeVisible();
      await diffDialog.locator('button').filter({ hasText: /^Close$/ }).click();
      await expect(diffDialog).toBeHidden();
      expect(browserErrors).toEqual([]);
    } finally {
      await page.request.post(`/api/jobs/${jobID}/purge?namespace=default&region=global`);
    }
  });

  test('renders and composes live job-list parity filters', async ({ page }) => {
    const browserErrors = await openCheckedDesktopPage(page, '/ui/jobs');

    await expectColumnHeaders(page, ['Namespace', 'Datacenter', 'Status', 'Type', 'Node Pool', 'Task Groups', 'Allocations']);

    const row = page.locator('[data-job-id="wander-ui-smoke"]');
    await expect(row).toContainText('default');
    await expect(row).toContainText('dc1');
    await expect(row).toContainText('service');
    await expect(row).toContainText('running');
    await expect(row).toContainText('1 running / 1 total');

    await page.goto('/ui/jobs?page=2&per_page=10');
    await page.getByLabel('Rows per page').selectOption('25');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expectURLParams(page, { per_page: '25', sort: 'submitted', dir: 'desc' });
    expect(new URL(page.url()).searchParams.has('page')).toBe(false);
    await expect(page.getByLabel('Rows per page')).toHaveValue('25');
    await expect(row).toBeVisible();

    await page.getByLabel('ID prefix').fill('missing');
    await page.getByRole('button', { name: 'Apply filters' }).click();
    await expect(page).toHaveURL(/prefix=missing/);
    await expect(row).toHaveCount(0);

    await page.getByLabel('ID prefix').fill('wander-ui');
    await page.locator('input[name="status"][value="running"]').check();
    await page.locator('input[name="type"][value="service"]').check();
    await page.getByLabel('Datacenter').selectOption('dc1');
    await page.getByLabel('Node pool').selectOption('default');
    await page.getByRole('button', { name: 'Apply filters' }).click();

    await expectURLParams(page, { status: 'running', type: 'service', datacenter: 'dc1', node_pool: 'default', per_page: '25' });
    await expect(row).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('streams client agent logs and persists the selected level', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/clients?region=global');
    await page.locator('[data-client-id] a[href^="/ui/clients/"]').first().click();
    await exerciseAgentMonitor(page, /\/ui\/clients\/[0-9a-f-]+\/monitor/, 'debug', true);
    expect(browserErrors).toEqual([]);
  });

  test('streams server agent logs for the selected server and region', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/servers?region=global');

    await expectColumnHeaders(page, ['Name', 'Status', 'Leader', 'Address', 'Port', 'Datacenter', 'Version']);
    await expect(page.locator('[data-server-port]').first()).not.toBeEmpty();
    await expect(page.locator('[data-server-version]').first()).not.toBeEmpty();

    await page.locator('a[href^="/ui/servers/"]').first().click();
    await expect(page).toHaveURL(/\/ui\/servers\/[^/]+/);
    await expect(page.getByRole('heading', { name: 'Server Information' })).toBeVisible();
    const tagNames = await page.locator('[data-server-tag] dt').allTextContents();
    if (tagNames.length === 0) {
      await expect(page.getByText('No tags available', { exact: true })).toBeVisible();
    } else {
      expect(tagNames).toEqual([...tagNames].sort());
      const copyButton = page.locator('[data-server-tag] [data-copy-button]').first();
      await copyButton.click();
      await expect(copyButton).toContainText('Copied');
    }

    await exerciseAgentMonitor(page, /\/ui\/servers\/[^/]+\/monitor/, 'warn');
    expect(browserErrors).toEqual([]);
  });

  test('opens live allocation logs and exec views', async ({ page }) => {
    test.setTimeout(90_000);
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/allocations');
    const smokeAllocation = page.locator('[data-alloc-id]').filter({ hasText: 'wander-ui-smoke' }).first();
    const href = await smokeAllocation.locator('a[href^="/ui/allocations/"]').first().getAttribute('href');
    expect(href).toBeTruthy();

	await page.goto(href!);
	await expect(page.locator('[data-testid="allocation-ports"]')).toContainText('http');
	await expect(page.locator('[data-testid="allocation-services"]')).toContainText('wander-ui-http');
	await expect(page.locator('[data-task-lifecycle="prepare"]')).toContainText('prestart');
	await expect(page.locator('[data-task-restart="fixture"]')).toBeVisible();
	await expect(page.locator('[data-all-tasks-restart]')).toBeVisible();
	const liveStats = page.locator('[data-testid="allocation-live-stats"]');
	await expect(liveStats).toContainText('Sampled');
	await expect(liveStats).not.toContainText('Statistics unavailable');
	await expect(page.getByText('Live Resource Usage', { exact: true }).first()).toBeVisible();

    await openAllocationView(page, href!, 'logs', /stdout|logs/i);
    const logViewer = page.locator('[data-log-viewer]');
    const logOutput = page.locator('[data-log-output]');
    await expect(logOutput).toHaveAttribute('role', 'log');
    await expect(logOutput).toHaveAttribute('aria-label', 'Allocation log output');
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveAttribute('role', 'status');
    await expect(page.getByRole('button', { name: 'stdout', exact: true })).toHaveAttribute('aria-pressed', 'true');
    for (const mode of ['stdout', 'both']) {
      if (mode === 'both') await page.getByRole('button', { name: mode, exact: true }).click();
      await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('connected');
      await expect(logOutput).toContainText('wander-ui-smoke');
    }
    const currentOutput = await logOutput.innerText();
    await expect.poll(() => logOutput.innerText(), { timeout: 10_000 }).not.toBe(currentOutput);

    const wrapToggle = page.locator('[data-log-wrap-toggle]');
    await expect(wrapToggle).toHaveAttribute('aria-pressed', 'false');
    await wrapToggle.click();
    await expect(wrapToggle).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('[data-log-line]').first()).toHaveCSS('white-space', 'pre-wrap');

    await page.locator('[data-log-action="head"]').click();
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('complete');
    await expect(logOutput).toContainText('wander-ui-smoke-head');

    await page.locator('[data-log-action="tail"]').click();
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('complete');
    await expect(logOutput).toContainText('wander-ui-smoke');

    await page.locator('[data-log-action="live"]').click();
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('connected');

    await logViewer.locator('[data-log-pause]').click();
    await expect.poll(async () => Number(await logViewer.locator('[data-log-buffered-count]').textContent()), { timeout: 10_000 }).toBeGreaterThan(0);
    await logViewer.locator('[data-log-pause]').click();
    await expect(logViewer.locator('[data-log-buffered-count]')).toHaveText('0');
    const downloadPromise = page.waitForEvent('download');
    await logViewer.locator('[data-log-download]').click();
    const logDownload = await downloadPromise;
    expect(logDownload.suggestedFilename()).toMatch(/^[0-9a-f]{8}-fixture-both\.log$/);
    await logViewer.locator('[data-log-clear]').click();
    await expect(logViewer.getByText('Lines:').locator('span')).toHaveText('0');
    const layout = await logViewer.evaluate(element => {
      const output = element.querySelector<HTMLElement>('[data-log-output]')!;
      const style = window.getComputedStyle(output);
      return {
        outputHeight: output.getBoundingClientRect().height,
        lineHeight: Number.parseFloat(style.lineHeight),
        verticalPadding: Number.parseFloat(style.paddingTop) + Number.parseFloat(style.paddingBottom),
        maxHeight: Number.parseFloat(style.maxHeight),
        paddingBottom: window.getComputedStyle(element).paddingBottom,
      };
    });
    expect((layout.outputHeight - layout.verticalPadding) / layout.lineHeight).toBeCloseTo(5, 0);
    expect(layout.maxHeight).toBeGreaterThanOrEqual(550);
    expect(layout.paddingBottom).toBe('32px');

    await page.getByRole('button', { name: 'stdout', exact: true }).click();
    await page.getByLabel('Select Task').selectOption('rotation');
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('connected');
    await expect(logOutput).toContainText('rotation-live-');

    await page.locator('[data-log-action="head"]').click();
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('complete');
    await expect(logOutput).toContainText('rotation-bulk-');
    await expect(logOutput).not.toContainText('ROTATION-PURGED-MARKER');

    await page.locator('[data-log-action="tail"]').click();
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('complete');
    await expect(logOutput).toContainText('rotation-live-');

    await page.locator('[data-log-action="live"]').click();
    await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('connected');
    const rotatedOutput = await logOutput.innerText();
    await expect.poll(() => logOutput.innerText(), { timeout: 10_000 }).not.toBe(rotatedOutput);

    await openAllocationView(page, href!, 'exec', /exec|terminal/i);
    const terminal = page.locator('[data-exec-terminal]');
    await expect(terminal).toHaveAttribute('role', 'region');
    await expect(terminal).toHaveAttribute('aria-label', 'Interactive terminal');
    await expect(page.locator('[data-exec-connection-state]')).toHaveAttribute('role', 'status');
    await expect(terminal.locator('.xterm')).toHaveCount(1);
    const terminalHeight = await terminal.evaluate(element => element.getBoundingClientRect().height);
    expect(terminalHeight).toBe(layout.maxHeight);
    const mainScroll = await page.locator('#main-content').evaluate(element => {
      element.scrollTop = element.scrollHeight;
      return {
        clientHeight: element.clientHeight,
        scrollHeight: element.scrollHeight,
        scrollTop: element.scrollTop,
      };
    });
    expect(mainScroll.scrollHeight).toBeGreaterThan(mainScroll.clientHeight);
    expect(mainScroll.scrollTop).toBeGreaterThan(0);

    await page.getByRole('button', { name: 'Connect', exact: true }).first().click();
    await expect(page.locator('[data-exec-connection-state]')).toHaveText('connected');
    await terminal.click();
    await page.keyboard.type('echo wander-browser-exec');
    await page.keyboard.press('Enter');
    await expect(terminal.locator('.xterm-rows')).toContainText('wander-browser-exec');
    await page.keyboard.type('exit');
    await page.keyboard.press('Enter');
    await expect(page.locator('[data-exec-connection-state]')).toHaveText('complete');
    await expect(page.locator('[data-exec-exit-code]')).toHaveText('0');

    await page.getByRole('button', { name: 'Connect', exact: true }).first().click();
    await expect(page.locator('[data-exec-connection-state]')).toHaveText('connected');
    await page.getByRole('button', { name: 'Disconnect', exact: true }).click();
    await expect(page.locator('[data-exec-connection-state]')).toHaveText('disconnected');

    const allocationURL = new URL(href!, 'http://wander.test');
    await page.goto(`${allocationURL.pathname}/fs${allocationURL.search}`);
    await expect(page.getByText('fixture', { exact: true }).last()).toBeVisible();
    await page.goto(`${allocationURL.pathname}/fixture/fs${allocationURL.search}`);
    await expect(page).toHaveURL(/\/ui\/allocations\/[^/]+\/fixture\/fs/);
    await expect(page.getByText('local', { exact: true })).toBeVisible();

    await openAllocationView(page, href!, 'files', /files|local/i);
    const localDirectory = page.getByRole('button', { name: /local/ });
    await localDirectory.click();
    await expect(localDirectory).toHaveAttribute('aria-expanded', 'true');
    const deepDirectory = page.getByRole('button', { name: /deep/ });
    await deepDirectory.click();
    await expect(deepDirectory).toHaveAttribute('aria-expanded', 'true');
    await expect(page.getByText('file-30.txt', { exact: true })).toBeVisible({ timeout: 30_000 });
    const privateDirectory = page.getByRole('button', { name: /private/ });
    await privateDirectory.focus();
    await page.keyboard.press('Enter');
    await expect(privateDirectory).toHaveAttribute('aria-expanded', 'true');
    const fileTree = page.locator('[data-file-tree-scroll]');
    const fileTreeLayout = await fileTree.evaluate(element => {
      const style = window.getComputedStyle(element);
      element.scrollTop = element.scrollHeight;
      return {
        clientHeight: element.clientHeight,
        scrollHeight: element.scrollHeight,
        scrollTop: element.scrollTop,
        overflowY: style.overflowY,
      };
    });
    expect(fileTreeLayout.clientHeight).toBeGreaterThanOrEqual(320);
    expect(fileTreeLayout.scrollHeight).toBeGreaterThan(fileTreeLayout.clientHeight);
    expect(fileTreeLayout.scrollTop).toBeGreaterThan(0);
    expect(fileTreeLayout.overflowY).toBe('auto');

    const largeFileRow = page.locator('[data-file-name="large.txt"]');
    await expect(largeFileRow).toBeVisible();
    const largeDownloadURL = await largeFileRow.getAttribute('data-download-url');
    await largeFileRow.focus();
    await page.keyboard.press('Enter');
    await expect(page.locator('#file-preview-content')).toContainText('FILE-HEAD-MARKER');
    await expect(page.locator('#file-preview-content')).not.toContainText('FILE-TAIL-MARKER');
    await page.locator('[data-file-preview-mode="tail"]').click();
    await expect(page.locator('#file-preview-content')).toContainText('FILE-TAIL-MARKER');
    await expect(page.locator('#file-preview-content')).not.toContainText('FILE-HEAD-MARKER');
    const liveFileButton = page.locator('[data-file-preview-mode="live"]');
    await liveFileButton.click();
    await expect(liveFileButton).toContainText('Stop Live');
    await expect(page.locator('[data-file-stream-output]')).toContainText('FILE-TAIL-MARKER');
    await liveFileButton.click();
    await expect(liveFileButton).toContainText('Live');
    const largeDownload = await page.request.get(largeDownloadURL!);
    expect(largeDownload.ok()).toBeTruthy();
    const largeDownloadBody = await largeDownload.body();
    expect(largeDownloadBody.length).toBeGreaterThan(50_000);
    expect(largeDownloadBody.toString()).toContain('FILE-HEAD-MARKER');
    expect(largeDownloadBody.toString()).toContain('FILE-TAIL-MARKER');

    const file = page.getByText('executor.out', { exact: true });
    await expect(file).toBeVisible();
    await file.click();
    await expect(page.locator('#file-preview-panel')).toBeVisible();
    await expect(page.locator('#file-preview-content .file-preview-container')).toBeVisible();
    const footer = page.locator('footer');
    await footer.scrollIntoViewIfNeeded();
    await expect(footer).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('renders the live job definition without browser errors', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/jobs/wander-ui-smoke');

    await page.getByRole('tab', { name: 'Definition' }).click();
    const editor = page.locator('.wander-editor-wrapper');
    await expect(editor).toBeVisible();
    await expect(editor).toContainText('job "wander-ui-smoke"');
    await expect(editor.locator('.wander-textarea')).toHaveAttribute('aria-label', 'Code editor');
    await page.getByRole('button', { name: 'Full Definition', exact: true }).click();
    await expect(editor).toContainText('"ID": "wander-ui-smoke"');
    const editorContrast = await editor.locator('.wander-highlighted-code').evaluate(element => {
      const style = window.getComputedStyle(element);
      return { color: style.color, background: style.backgroundColor };
    });
    expect(editorContrast).toEqual({ color: 'rgb(31, 41, 55)', background: 'rgba(0, 0, 0, 0)' });
    expect(await editor.evaluate(element => element.getBoundingClientRect().height)).toBeGreaterThanOrEqual(550);

    expect(browserErrors).toEqual([]);
  });

  test('opens and dismisses task group scale confirmation', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/jobs/wander-ui-smoke/taskgroups/web');

    await expect(page.locator('[data-task-lifecycle="prepare"]')).toContainText('prestart');
    await expect(page.locator('[data-task-lifecycle="prepare"]')).toContainText('Sidecar: No');
    await expect(page.locator('[data-task-lifecycle="fixture"]')).toContainText('Lifecycle: Normal');
    await expect(page.locator('[data-task-group-volumes]')).toContainText('No configured volumes');
    await expect(page.locator('[data-task-group-runtime]')).toContainText('Desired');
    await expect(page.locator('[data-task-group-runtime]')).toContainText('Running');
    await expect(page.locator('[data-task-group-allocations] [data-task-group-allocation]')).toHaveCount(1);
    const allocationFilters = page.locator('[data-task-group-allocations] form');
    await allocationFilters.locator('select[name="status"]').selectOption('running');
    await allocationFilters.locator('select[name="sort"]').selectOption('node');
    await allocationFilters.locator('select[name="order"]').selectOption('asc');
    await page.goto('/ui/jobs/wander-ui-smoke/taskgroups/web?status=running&sort=node&order=asc');
    await expect(page).toHaveURL(/status=running.*sort=node.*order=asc/);
    await expect(page.locator('[data-task-group-allocations] select[name="status"]')).toHaveValue('running');
    await expect(page.locator('[data-task-group-allocations] select[name="sort"]')).toHaveValue('node');
    await expect(page.locator('[data-task-group-allocations] select[name="order"]')).toHaveValue('asc');
    await expect(page.locator('[data-task-group-allocations] [data-task-group-allocation]')).toHaveCount(1);

    const count = page.getByLabel('Instance count');
    const decrement = page.getByRole('button', { name: 'Decrease count' });
    const increment = page.getByRole('button', { name: 'Increase count' });
    const scaleButton = page.getByRole('button', { name: 'Scale', exact: true });
    await expect(count).toHaveValue('1');
    await expect(count).toHaveAttribute('min', '1');
    await expect(count).toHaveAttribute('max', '2');
    await expect(page.getByText('Min:', { exact: true }).locator('..')).toContainText('1');
    await expect(page.getByText('Max:', { exact: true }).locator('..')).toContainText('2');
    await expect(decrement).toBeDisabled();
    await expect(increment).toBeEnabled();
    await expect(scaleButton).toBeDisabled();
    await count.fill('0');
    await expect(scaleButton).toBeDisabled();
    await count.fill('3');
    await expect(scaleButton).toBeDisabled();
    await count.fill('1');
    await increment.click();
    await expect(count).toHaveValue('2');
    await expect(increment).toBeDisabled();
    await expect(decrement).toBeEnabled();
    await expect(scaleButton).toBeEnabled();

    const dialog = page.getByRole('dialog', { name: 'Confirm Scale Operation' });
    const openDialog = async () => {
      await scaleButton.click();
      await expect(dialog).toBeVisible();
      await expect(dialog.getByText('Current Count:').locator('..')).toContainText('1');
      await expect(dialog.getByText('New Count:').locator('..')).toContainText('2');
      await expect(dialog.getByText('Change:').locator('..')).toContainText('+1 instance(s)');
    };
    const dismissDialog = async (buttonName: 'Cancel' | 'Close') => {
      await openDialog();
      await dialog.getByRole('button', { name: buttonName }).click();
      await expect(dialog).toBeHidden();
    };

    await dismissDialog('Cancel');
    await dismissDialog('Close');

    await openDialog();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(page.locator('body')).not.toHaveClass(/overflow-hidden/);
    expect(browserErrors).toEqual([]);
  });

  test('supports basic keyboard and screen-reader navigation', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await page.goto('/ui/');

    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
    await expect(page.getByRole('main')).toBeVisible();
    await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();

    const accessibilityProblems = await page.locator('body').evaluate(() => {
      const visible = (element: Element) => {
        const style = window.getComputedStyle(element);
        return style.display !== 'none' && style.visibility !== 'hidden';
      };
      const problems: string[] = [];
      const ids = new Set<string>();
      document.querySelectorAll<HTMLElement>('[id]').forEach(element => {
        if (ids.has(element.id)) problems.push(`duplicate id: ${element.id}`);
        ids.add(element.id);
      });
      document.querySelectorAll<HTMLElement>('button, a[href]').forEach(element => {
        if (!visible(element)) return;
        const name = element.textContent?.trim() || element.getAttribute('aria-label') || element.getAttribute('title');
        if (!name) problems.push(`unnamed control: ${element.outerHTML.slice(0, 120)}`);
      });
      document.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('input:not([type="hidden"]), select, textarea').forEach(element => {
        if (!visible(element)) return;
        const labelled = element.getAttribute('aria-label') || element.getAttribute('aria-labelledby') || (element.id && document.querySelector(`label[for="${CSS.escape(element.id)}"]`)) || element.closest('label');
        if (!labelled) problems.push(`unlabelled field: ${element.outerHTML.slice(0, 120)}`);
      });
      return problems;
    });
    expect(accessibilityProblems).toEqual([]);

    let reachedJobs = false;
    for (let index = 0; index < 40; index += 1) {
      await page.keyboard.press('Tab');
      const focusedHref = await page.evaluate(() => document.activeElement?.getAttribute('href'));
      if (focusedHref?.startsWith('/ui/jobs')) {
        reachedJobs = true;
        await page.keyboard.press('Enter');
        break;
      }
    }
    expect(reachedJobs).toBe(true);
    await expect(page).toHaveURL(/\/ui\/jobs/);
    await expect(page.getByRole('heading', { level: 1 }).first()).toBeVisible();
    expect(browserErrors).toEqual([]);
  });

  test('supports keyboard and screen-reader semantics for complex controls', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/jobs?namespace=default&region=global');

    const regionTrigger = page.getByRole('button', { name: 'Select region' });
    await expect(regionTrigger).toHaveAttribute('aria-haspopup', 'menu');
    await expect(regionTrigger).toHaveAttribute('aria-expanded', 'false');
    await regionTrigger.focus();
    await page.keyboard.press('ArrowDown');
    await expect(regionTrigger).toHaveAttribute('aria-expanded', 'true');
    const regionItems = page.getByRole('menuitem');
    await expect(regionItems.first()).toBeFocused();
    await page.keyboard.press('End');
    await expect(regionItems.last()).toBeFocused();
    await page.keyboard.press('Home');
    await expect(regionItems.first()).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(regionTrigger).toHaveAttribute('aria-expanded', 'false');
    await expect(regionTrigger).toBeFocused();

    const sortedHeader = page.locator('th[aria-sort]').first();
    await expect(sortedHeader).toHaveAttribute('aria-sort', /ascending|descending/);
    await expect(sortedHeader.getByRole('button')).toBeVisible();

    await page.locator('[data-job-id="wander-ui-smoke"] a').first().click();
    const overviewTab = page.getByRole('tab', { name: 'Overview' });
    await expect(page).toHaveURL(/\/ui\/jobs\/wander-ui-smoke/);
    await expect(overviewTab).toHaveAttribute('aria-selected', 'true');
    await overviewTab.focus();
    await expect(overviewTab).toBeFocused();
    const lastTab = page.getByRole('tab').last();
    await page.keyboard.press('ArrowLeft');
    await expect(lastTab).toBeFocused();
    await expect(lastTab).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('ArrowRight');
    await expect(overviewTab).toBeFocused();
    await expect(overviewTab).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('ArrowRight');
    await expect(page.getByRole('tab', { name: 'Definition' })).toBeFocused();
    await expect(page.getByRole('tab', { name: 'Definition' })).toHaveAttribute('aria-selected', 'true');

    await openDesktopPage(page, '/ui/jobs/wander-ui-smoke/taskgroups/web');
    await page.getByLabel('Instance count').fill('2');
    const scaleButton = page.getByRole('button', { name: 'Scale', exact: true });
    await scaleButton.click();
    const dialog = page.getByRole('dialog', { name: 'Confirm Scale Operation' });
    await expect(dialog.getByRole('button', { name: 'Close' })).toBeFocused();
    await page.keyboard.press('Shift+Tab');
    await expect(dialog.getByRole('button', { name: 'Confirm Scale' })).toBeFocused();
    await page.keyboard.press('Tab');
    await expect(dialog.getByRole('button', { name: 'Close' })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(scaleButton).toBeFocused();
    expect(browserErrors).toEqual([]);
  });

  test('keeps navigation, tables, dialogs, and workspaces reachable across viewport sizes', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    const viewports = [
      { name: 'phone', width: 390, height: 844 },
      { name: 'tablet', width: 768, height: 1024 },
      { name: 'laptop', width: 1366, height: 768 },
      { name: 'wide', width: 1920, height: 1080 },
    ];

    for (const viewport of viewports) {
      await test.step(viewport.name, async () => {
        await page.setViewportSize({ width: viewport.width, height: viewport.height });
        await page.goto('/ui/jobs?namespace=default&region=global');
        await expect(page.getByRole('heading', { name: 'Jobs', exact: true })).toBeVisible();
        await expectNoDocumentOverflow(page);

        const sidebar = page.locator('#sidebar');
        if (viewport.width < 1024) {
          await expect.poll(async () => (await sidebar.boundingBox())?.x).toBeLessThan(0);
          await page.getByRole('button', { name: 'Toggle sidebar' }).click();
          await expect.poll(async () => (await sidebar.boundingBox())?.x).toBeGreaterThanOrEqual(0);
          await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
          await page.keyboard.press('Escape');
          await expect.poll(async () => (await sidebar.boundingBox())?.x).toBeLessThan(0);
        } else {
          await expect.poll(async () => (await sidebar.boundingBox())?.x).toBeGreaterThanOrEqual(0);
        }

        const search = page.locator('[data-global-search]');
        if (viewport.width >= 1280) await expect(search).toBeVisible();
        else await expect(search).toBeHidden();

        const tableScroller = page.locator('#jobs-table-container .overflow-x-auto').first();
        await expect(tableScroller).toBeVisible();
        const tableGeometry = await tableScroller.evaluate(element => ({
          clientWidth: element.clientWidth,
          scrollWidth: element.scrollWidth,
          right: element.getBoundingClientRect().right,
        }));
        expect(tableGeometry.scrollWidth).toBeGreaterThanOrEqual(tableGeometry.clientWidth);
        expect(tableGeometry.right).toBeLessThanOrEqual(viewport.width + 1);

        const footerBox = await page.locator('footer').boundingBox();
        expect(footerBox?.x).toBeGreaterThanOrEqual(0);
        expect(footerBox?.width).toBeLessThanOrEqual(viewport.width);

        await page.goto('/ui/jobs/wander-ui-smoke/taskgroups/web?namespace=default&region=global');
        await page.getByLabel('Instance count').fill('2');
        await page.getByRole('button', { name: 'Scale', exact: true }).click();
        const dialog = page.getByRole('dialog', { name: 'Confirm Scale Operation' });
        await expect(dialog).toBeVisible();
        const dialogBox = await dialog.locator(':scope > div:nth-child(2) > div').boundingBox();
        expect(dialogBox?.x).toBeGreaterThanOrEqual(0);
        expect((dialogBox?.x || 0) + (dialogBox?.width || 0)).toBeLessThanOrEqual(viewport.width);
        expect(dialogBox?.y).toBeGreaterThanOrEqual(0);
        expect((dialogBox?.y || 0) + (dialogBox?.height || 0)).toBeLessThanOrEqual(viewport.height);
        await dialog.getByRole('button', { name: 'Cancel' }).click();
        await expectNoDocumentOverflow(page);

        if (viewport.name === 'phone' || viewport.name === 'wide') {
          await page.goto('/ui/allocations?namespace=default&region=global');
          const allocationHref = await page.locator('[data-alloc-id]').filter({ hasText: 'wander-ui-smoke' }).first()
            .locator('a[href^="/ui/allocations/"]').first().getAttribute('href');
          expect(allocationHref).toBeTruthy();
          for (const workspace of ['logs', 'files', 'exec'] as const) {
            await openAllocationView(page, allocationHref!, workspace, /logs|files|exec|terminal/i);
            await expectNoDocumentOverflow(page);
            await expect(page.getByRole('main')).toBeVisible();
          }
        }
      });
    }
    expect(browserErrors).toEqual([]);
  });

  test('supports global shortcut help, preferences, rebinding, and navigation', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/jobs?namespace=default&region=global');

    await page.keyboard.press('Shift+/');
    let dialog = page.getByRole('dialog', { name: 'Keyboard Shortcuts' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Change shortcut for Go to ACL Tokens' })).toHaveText('G then A');

    const enabled = dialog.getByLabel('Enable keyboard shortcuts');
    await enabled.uncheck();
    await page.keyboard.press('Escape');
    await page.keyboard.press('g');
    await page.keyboard.press('c');
    await expect(page).toHaveURL(/\/ui\/jobs/);

    await page.keyboard.press('Shift+/');
    dialog = page.getByRole('dialog', { name: 'Keyboard Shortcuts' });
    await expect(dialog).toBeVisible();
    await dialog.getByLabel('Enable keyboard shortcuts').check();
    await dialog.getByRole('button', { name: 'Change shortcut for Focus search' }).click();
    await page.keyboard.press('x');
    await page.keyboard.press('Enter');
    await expect(dialog.getByRole('button', { name: 'Change shortcut for Focus search' })).toHaveText('X');
    await page.keyboard.press('Escape');

    await page.keyboard.press('x');
    await expect(page.locator('#global-search')).toBeFocused();
    await page.locator('#global-search').blur();
    await page.keyboard.press('g');
    await page.keyboard.press('c');
    await expect(page).toHaveURL(/\/ui\/clients/);
    await expect(page).toHaveURL(/namespace=default/);
    await expect(page).toHaveURL(/region=global/);

    await page.keyboard.press('Shift+ArrowDown');
    await expect(page).toHaveURL(/\/ui\/servers/);
    await page.keyboard.press('g');
    await page.keyboard.press('j');
    await expect(page).toHaveURL(/\/ui\/jobs/);
    await page.locator('[data-job-id="wander-ui-smoke"] a').first().click();
    await expect(page.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('Shift+ArrowRight');
    await expect(page.getByRole('tab', { name: 'Definition' })).toHaveAttribute('aria-selected', 'true');
    await page.keyboard.press('Shift+ArrowLeft');
    await expect(page.getByRole('tab', { name: 'Overview' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('body')).toHaveAttribute('data-wander-namespace', 'default');
    await expect(page.locator('body')).toHaveAttribute('data-wander-region', 'global');
    await page.keyboard.press('u');
    await expect(page).toHaveURL(/\/ui\/jobs\?/);

    await page.keyboard.press('Control+k');
    await expect(page.locator('#global-search')).toBeFocused();
    expect(browserErrors).toEqual([]);
  });
});

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
    await expect(page.getByRole('columnheader', { name: heading })).toBeVisible();
  }
}

async function expectURLParams(page: Page, expected: Record<string, string>) {
  await expect.poll(() => {
    const params = new URL(page.url()).searchParams;
    return Object.fromEntries(Object.keys(expected).map(name => [name, params.get(name)]));
  }).toEqual(expected);
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

    await page.goto('/ui/jobs/wander-ui-parameterized?namespace=dispatch-test&region=global');
    await expect(page.getByRole('button', { name: 'Dispatch', exact: true })).toBeVisible();
    await expect(page.locator('[data-overview-child-jobs]')).toBeVisible();
    await expect(page.locator('[data-overview-scheduling]')).toContainText('Dispatch Payload');
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

      await submitVariableValue(teamPage, 'updated', 'Save Variable');
      await expect(teamPage.getByText('Variable saved successfully')).toBeVisible();
      await teamPage.reload();
      await expect(teamPage.getByPlaceholder('Value')).toHaveValue('updated');

      const deleteModal = await openDialog(teamPage, 'Delete', 'Delete Variable');
      await deleteModal.getByRole('button', { name: 'Delete Variable' }).click();
      await expect(teamPage).toHaveURL(/\/ui\/variables$/);
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
    await expect(page.locator('footer')).toContainText('v0.1.0-dev');
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
    await expect(page.getByText('v0.1.0-dev', { exact: true })).toHaveCount(1);

    await openResource(page, '/ui/jobs', '[data-job-id="wander-ui-smoke"]', 'a[href^="/ui/jobs/wander-ui-smoke"]', /\/ui\/jobs\/wander-ui-smoke/, 'wander-ui-smoke');
    await expect(page.locator('[data-overview-allocation-summary]')).toContainText('1');
    await expect(page.locator('[data-overview-recent-allocations]')).toContainText('web');
    await expect(page.locator('[data-overview-placement-status]')).toContainText('no placement failures');
    await expect(page.locator('[data-overview-scheduling]')).toContainText('default');
    await expect(page.locator('body')).toContainText('test-purpose');
    await expect(page.locator('body')).toContainText('job-overview');
    await openResource(page, '/ui/allocations', '[data-alloc-id]:has-text("wander-ui-smoke")', 'a[href^="/ui/allocations/"]', /\/ui\/allocations\/[0-9a-f-]+/, 'fixture');
    await page.goto('/ui/clients');
    const clientRow = page.locator('[data-client-id]').first();
    await expect(clientRow).toContainText('CPU allocated');
    await expect(clientRow).toContainText('Memory allocated');
    await clientRow.locator('a[href^="/ui/clients/"]').first().click();
    await expect(page).toHaveURL(/\/ui\/clients\/[0-9a-f-]+/);

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
    const topologyNode = page.locator('[data-topology-node-id]').first();
    await expect(topologyNode).toContainText('Allocations');
    await expect(topologyNode).toContainText('CPU allocated');
    await expect(topologyNode).toContainText('Memory allocated');
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
    await page.getByRole('link', { name: 'Monitor', exact: true }).click();

    await expect(page).toHaveURL(/\/ui\/clients\/[0-9a-f-]+\/monitor/);
    await expect(page.locator('[data-agent-monitor]')).toBeVisible();
    const monitorState = page.locator('[data-monitor-state]');
    await expect(monitorState).toHaveText('connected');
    await expect(page.locator('[data-monitor-level]')).toHaveValue('info');

    await page.locator('[data-monitor-level]').selectOption('debug');
    await expect(page).toHaveURL(/level=debug/);
    await expect(monitorState).toHaveText('connected');

    await page.locator('[data-monitor-toggle]').click();
    await expect(monitorState).toHaveText('paused');
    await expect(page.locator('[data-monitor-toggle]')).toHaveText('Resume');

    await page.locator('[data-monitor-toggle]').click();
    await expect(monitorState).toHaveText('connected');
    expect(browserErrors).toEqual([]);
  });

  test('streams server agent logs for the selected server and region', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/servers?region=global');

	await expectColumnHeaders(page, ['Name', 'Status', 'Leader', 'Address', 'Port', 'Datacenter', 'Version']);
    await expect(page.locator('[data-server-port]').first()).not.toBeEmpty();
    await expect(page.locator('[data-server-version]').first()).not.toBeEmpty();

    await page.locator('a[href^="/ui/servers/"]').first().click();
    const tagNames = await page.locator('[data-server-tag] dt').allTextContents();
    expect(tagNames.length).toBeGreaterThan(0);
    expect(tagNames).toEqual([...tagNames].sort());
    const copyButton = page.locator('[data-copy-button]').first();
    await copyButton.click();
    await expect(copyButton).toContainText('Copied');

    await page.getByRole('link', { name: 'Monitor', exact: true }).click();

    await expect(page).toHaveURL(/\/ui\/servers\/[^/]+\/monitor/);
    await expect(page.locator('[data-agent-monitor]')).toBeVisible();
    const monitorState = page.locator('[data-monitor-state]');
    await expect(monitorState).toHaveText('connected');
    await expect(page.locator('[data-monitor-level]')).toHaveValue('info');

    await page.locator('[data-monitor-level]').selectOption('warn');
    await expect(page).toHaveURL(/level=warn/);
    await expect(monitorState).toHaveText('connected');

    await page.locator('[data-monitor-toggle]').click();
    await expect(monitorState).toHaveText('paused');
    expect(browserErrors).toEqual([]);
  });

  test('opens live allocation logs and exec views', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/allocations');
    const smokeAllocation = page.locator('[data-alloc-id]').filter({ hasText: 'wander-ui-smoke' }).first();
    const href = await smokeAllocation.locator('a[href^="/ui/allocations/"]').getAttribute('href');
    expect(href).toBeTruthy();

	await page.goto(href!);
	await expect(page.locator('[data-testid="allocation-ports"]')).toContainText('http');
	await expect(page.locator('[data-testid="allocation-services"]')).toContainText('wander-ui-http');
	await expect(page.locator('[data-task-lifecycle="prepare"]')).toContainText('prestart');

    await openAllocationView(page, href!, 'logs', /stdout|logs/i);
    const logViewer = page.locator('[data-log-viewer]');
    const logOutput = page.locator('[data-log-output]');
    for (const mode of ['stdout', 'both']) {
      if (mode === 'both') await page.getByRole('button', { name: mode, exact: true }).click();
      await expect(logViewer.locator('[data-log-connection-state]')).toHaveText('connected');
      await expect(logOutput).toContainText('wander-ui-smoke');
    }
    const currentOutput = await logOutput.innerText();
    await expect.poll(() => logOutput.innerText(), { timeout: 10_000 }).not.toBe(currentOutput);

    await page.getByRole('button', { name: 'Pause', exact: true }).click();
    await page.getByRole('button', { name: 'Clear', exact: true }).click();
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

    await openAllocationView(page, href!, 'exec', /exec|terminal/i);
    const terminal = page.locator('[data-exec-terminal]');
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

    await openAllocationView(page, href!, 'files', /files|fixture/i);
    await page.getByText('fixture', { exact: true }).last().click();
    await page.getByText('local', { exact: true }).click();
    await page.getByText('deep', { exact: true }).click();
    await expect(page.getByText('file-30.txt', { exact: true })).toBeVisible();
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

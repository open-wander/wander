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

async function expectTeamNamespaceJobs(page: Page) {
  await expect(page.locator('[data-job-id="wander-ui-team-smoke"]')).toBeVisible();
  await expect(page.locator('[data-job-id="wander-ui-smoke"]')).toHaveCount(0);
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

async function selectNamespace(page: Page, namespace: string) {
  await page.getByLabel('Select namespace').click();
  await page.getByText(namespace, { exact: true }).click();
}

test.describe('Wander release smoke', () => {
  test('isolates namespace selection between browser sessions', async ({ browser }) => {
    const { defaultPage, teamPage, close } = await newIsolatedNamespacePages(browser);

    try {
      await defaultPage.goto('/ui/jobs?namespace=default');
      await teamPage.goto('/ui/jobs?namespace=default');

      await expect(defaultPage.locator('[data-job-id="wander-ui-smoke"]')).toBeVisible();
      await expect(defaultPage.locator('[data-job-id="wander-ui-team-smoke"]')).toHaveCount(0);

      await selectNamespace(teamPage, 'team-a');
      await expect(teamPage).toHaveURL(/\/ui\/jobs\?.*namespace=team-a/);
      await expectTeamNamespaceJobs(teamPage);

      await teamPage.locator('[data-job-id="wander-ui-team-smoke"] a').click();
      await expectTeamJobDetail(teamPage);

      await teamPage.getByRole('link', { name: 'Jobs', exact: true }).click();
      await expectTeamNamespaceJobs(teamPage);

      await defaultPage.reload();
      await expect(defaultPage).toHaveURL(/namespace=default/);
      await expect(defaultPage.locator('[data-job-id="wander-ui-smoke"]')).toBeVisible();
      await expect(defaultPage.locator('[data-job-id="wander-ui-team-smoke"]')).toHaveCount(0);
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
      await selectNamespace(teamPage, 'team-a');
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

      await teamPage.getByRole('button', { name: 'Delete', exact: true }).click();
      const deleteModal = teamPage.getByRole('dialog', { name: 'Delete Variable' });
      await expect(deleteModal).toBeVisible();
      await deleteModal.getByRole('button', { name: 'Delete Variable' }).click();
      await expect(teamPage).toHaveURL(/\/ui\/variables$/);
      await expect(teamPage.getByText(variablePath, { exact: true })).toHaveCount(0);
    } finally {
      await teamPage.request.delete(`/ui/variables/${variablePath}?namespace=team-a`);
      await close();
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
    await openResource(page, '/ui/allocations', '[data-alloc-id]', 'a[href^="/ui/allocations/"]', /\/ui\/allocations\/[0-9a-f-]+/, 'fixture');
    await page.goto('/ui/clients');
    const clientRow = page.locator('[data-client-id]').first();
    await expect(clientRow).toContainText('1');
    await expect(clientRow).toContainText('50 /');
    await clientRow.locator('a[href^="/ui/clients/"]').first().click();
    await expect(page).toHaveURL(/\/ui\/clients\/[0-9a-f-]+/);

    await page.goto('/ui/servers');
    await expect(page.locator('tbody tr').first()).toBeVisible();

    await page.goto('/ui/topology');
    const topologyNode = page.locator('[data-topology-node-id]').first();
    await expect(topologyNode).toContainText('Allocations');
    await expect(topologyNode).toContainText('1');
    await expect(topologyNode).toContainText('50 /');
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
    expect(browserErrors).toEqual([]);
  });

  test('opens live allocation logs and exec views', async ({ page }) => {
    const browserErrors = captureBrowserErrors(page);
    await openDesktopPage(page, '/ui/allocations');
    const href = await page.locator('[data-alloc-id] a[href^="/ui/allocations/"]').first().getAttribute('href');
    expect(href).toBeTruthy();

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
    const terminalHeight = await page.locator('[data-exec-terminal]').evaluate(element => element.getBoundingClientRect().height);
    expect(terminalHeight).toBe(layout.maxHeight);

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
    await page.getByRole('button', { name: 'JSON', exact: true }).click();
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

    const count = page.getByLabel('Instance count');
    const scaleButton = page.getByRole('button', { name: 'Scale', exact: true });
    await expect(count).toHaveValue('1');
    await expect(scaleButton).toBeDisabled();
    await count.fill('-1');
    await expect(scaleButton).toBeDisabled();
    await count.fill('1');
    await page.getByRole('button', { name: 'Increase count' }).click();
    await expect(count).toHaveValue('2');
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
});

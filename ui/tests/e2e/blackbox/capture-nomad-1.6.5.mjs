import { writeFile } from 'node:fs/promises';
import { chromium } from '@playwright/test';

const baseURL = process.env.NOMAD_1_6_5_URL;
const outputPath = process.env.NOMAD_1_6_5_CAPTURE;
const shortcutsOnly = process.env.NOMAD_1_6_5_SHORTCUTS_ONLY === '1';
const aclToken = process.env.NOMAD_1_6_5_TOKEN;
const configuredPaths = (process.env.NOMAD_1_6_5_PATHS || '')
  .split(',')
  .map((value) => value.trim())
  .filter(Boolean);

if (!baseURL || !outputPath) {
  throw new Error('NOMAD_1_6_5_URL and NOMAD_1_6_5_CAPTURE are required');
}

const origin = new URL(baseURL).origin;
const defaultPaths = [
  '/ui/',
  '/ui/jobs',
  '/ui/jobs/run',
  '/ui/clients',
  '/ui/servers',
  '/ui/topology',
  '/ui/storage',
  '/ui/evaluations',
  '/ui/variables',
  '/ui/settings/tokens',
  '/ui/optimize',
];
const pending = configuredPaths.length > 0 ? configuredPaths : defaultPaths;
const queued = new Set(pending);
const visited = new Set();
const pages = [];
const apiRequests = new Set();

function normalizeUIPath(value) {
  const url = new URL(value, origin);
  if (url.origin !== origin || !url.pathname.startsWith('/ui')) {
    return null;
  }
  return url.pathname;
}

function uniqueByNameAndHref(links) {
  const seen = new Set();
  return links.filter((link) => {
    const key = `${link.name}\n${link.href}`;
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function cleanText(values) {
  return values.map((value) => value.trim()).filter(Boolean);
}

const browser = await chromium.launch({ headless: true });
let storageState;

function newBrowserContext() {
  return browser.newContext({
    viewport: { width: 1440, height: 1000 },
    ...(storageState ? { storageState } : {}),
  });
}

async function newIsolatedPage() {
  const context = await newBrowserContext();
  return { context, page: await context.newPage() };
}

async function navigate(page, path, settleMilliseconds) {
  const response = await page.goto(new URL(path, origin).toString(), {
    waitUntil: 'domcontentloaded',
  });
  await page.waitForTimeout(settleMilliseconds);
  return response;
}

if (aclToken) {
  const { context: authContext, page: authPage } = await newIsolatedPage();
  await navigate(authPage, '/ui/settings/tokens', 1000);
  await authPage.getByRole('textbox').last().fill(aclToken);
  await authPage.getByRole('button', { name: 'Sign in with secret' }).click();
  await authPage.waitForTimeout(750);
  storageState = await authContext.storageState();
  await authContext.close();
}

while (!shortcutsOnly && pending.length > 0 && visited.size < 100) {
  const requestedPath = pending.shift();
  if (!requestedPath || visited.has(requestedPath)) {
    continue;
  }
  visited.add(requestedPath);

  const { context, page } = await newIsolatedPage();
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (url.origin === origin && url.pathname.startsWith('/v1/')) {
      apiRequests.add(`${request.method()} ${url.pathname}`);
    }
  });
  try {
    const response = await navigate(page, requestedPath, 750);

    const headings = await page.getByRole('heading').allTextContents();
    const buttons = await page.getByRole('button').allTextContents();
    const tabs = await page.getByRole('tab').allTextContents();
    const links = uniqueByNameAndHref(await page.getByRole('link').evaluateAll((elements) =>
      elements
        .filter((element) => element instanceof HTMLAnchorElement && element.offsetParent !== null)
        .map((element) => ({
          name: (element.innerText || element.getAttribute('aria-label') || '').trim(),
          href: element.href,
        }))
        .filter((link) => link.name && link.href),
    ));

    const finalURL = new URL(page.url());
    pages.push({
      requestedPath,
      finalPath: `${finalURL.pathname}${finalURL.search}`,
      status: response?.status() ?? null,
      title: await page.title(),
      headings: cleanText(headings),
      buttons: cleanText(buttons),
      tabs: cleanText(tabs),
      links: links.map((link) => ({
        name: link.name,
        path: normalizeUIPath(link.href) ?? link.href,
      })),
    });

    for (const link of links) {
      const path = normalizeUIPath(link.href);
      if (path && !visited.has(path) && !queued.has(path)) {
        queued.add(path);
        pending.push(path);
      }
    }
  } finally {
    await context.close();
  }
}

const { context, page } = await newIsolatedPage();
await navigate(page, '/ui/', 1000);
await page.keyboard.type('?');
await page.waitForTimeout(500);
const shortcutTitle = page.getByText('Keyboard Shortcuts', { exact: true }).first();
const keyboardShortcuts = {
  visible: await shortcutTitle.isVisible().catch(() => false),
  headings: cleanText(await page.getByRole('heading').allTextContents()),
  buttons: cleanText(await page.getByRole('button').allTextContents()),
};

await context.close();
await browser.close();
await writeFile(outputPath, `${JSON.stringify({
  capturedAt: new Date().toISOString(),
  baseURL: origin,
  pages,
  apiRequests: [...apiRequests].sort(),
  keyboardShortcuts,
}, null, 2)}\n`, { mode: 0o600 });

/**
 * Admin console walkthrough (read-only). Repeats the 2026-11-24 UI audit.
 *
 * Prerequisites: admin UI on http://localhost:5173, API on :8080 with the demo seed,
 * `npm ci` done in src/admin, chromium installed (`npx playwright install chromium`).
 *
 * Run from the repo root:
 *   node docs/quality/admin-ui-audit/walkthrough.mjs [outDir]
 * Env: ADMIN_URL (default http://localhost:5173), SHOTS=1 to save a PNG per page/viewport.
 *
 * Demo logins only (documented in the audit report): e2e-checker@HPay.dev (superadmin) and
 * e2e-maker@HPay.dev (ops), password E2ETest@123.
 *
 * It logs in through the real form, walks every sidebar page, the detail pages, every service
 * modal tab, every offer detail and every org, at 4 viewports. It records console errors,
 * page errors, HTTP >= 400 responses, horizontal overflow, Vietnamese/mojibake/dev-wording text
 * and writes page text dumps to <outDir>/text/ plus a findings.json. It never clicks a button
 * that writes data (dialogs are opened and cancelled only).
 */
import { createRequire } from 'node:module';
import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const adminDir = path.resolve(here, '../../../src/admin');
const require = createRequire(path.join(adminDir, 'package.json'));
const { chromium } = require('playwright');

const BASE = process.env.ADMIN_URL || 'http://localhost:5173';
const OUT = path.resolve(process.argv[2] || path.join(process.env.TEMP || '.', 'hpay-admin-audit'));
const SHOTS = process.env.SHOTS === '1';
const PASSWORD = 'E2ETest@123';
const ROLES = { superadmin: 'e2e-checker@HPay.dev', ops: 'e2e-maker@HPay.dev' };
const VIEWPORTS = {
  '1920x1080': { width: 1920, height: 1080 },
  '1440x900': { width: 1440, height: 900 },
  '1024x768': { width: 1024, height: 768 },
  '390x844': { width: 390, height: 844 },
};
const ROUTES = ['/dashboard', '/kyc', '/fraud', '/workflows', '/tasks', '/attention', '/recon',
  '/services', '/offers', '/orgs', '/ai-settings', '/audit'];

const VI = /[À-ỹ]/; // Latin letters with diacritics (Vietnamese); review hits manually
const MOJIBAKE = /Ã.|Â.|â€|�/;
const DEV = /TanStack|coming in|sample data|lorem|TODO|undefined|\[object|NaN|null\b|mock/i;

fs.mkdirSync(path.join(OUT, 'text'), { recursive: true });
fs.mkdirSync(path.join(OUT, 'shots'), { recursive: true });
const findings = [];
const note = (role, vp, route, kind, detail) => {
  findings.push({ role, vp, route, kind, detail });
  console.log(`[${role} ${vp}] ${route} ${kind}: ${String(detail).slice(0, 200)}`);
};

async function login(browser, role, vp) {
  const ctx = await browser.newContext({ viewport: VIEWPORTS[vp] });
  const page = await ctx.newPage();
  let route = 'login';
  const setRoute = (r) => { route = r; };
  page.on('console', (m) => {
    if (m.type() === 'error' || m.type() === 'warning') note(role, vp, route, 'console.' + m.type(), m.text());
  });
  page.on('pageerror', (e) => note(role, vp, route, 'pageerror', e.message));
  page.on('response', (r) => {
    if (r.status() >= 400 && r.url().includes('/v1/')) note(role, vp, route, 'http' + r.status(), r.request().method() + ' ' + r.url());
  });
  await page.goto(BASE + '/login');
  await page.fill('input[type=email], input[name=email]', ROLES[role]);
  await page.fill('input[type=password]', PASSWORD);
  await page.click('button[type=submit]');
  await page.waitForURL('**/dashboard', { timeout: 15000 });
  return { ctx, page, setRoute };
}

async function inspect(page, role, vp, route, label) {
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.waitForTimeout(400);
  const info = await page.evaluate(() => ({
    text: document.body.innerText,
    sw: document.documentElement.scrollWidth,
    cw: document.documentElement.clientWidth,
    wide: [...document.querySelectorAll('main *, body > div *')]
      .filter((e) => e.getBoundingClientRect().right > window.innerWidth + 1 && !e.closest('.overflow-x-auto, table, pre'))
      .slice(0, 3).map((e) => e.tagName + '.' + String(e.className).slice(0, 60)),
  }));
  const key = `${role}_${vp}_${label.replace(/[^\w-]+/g, '_')}`;
  fs.writeFileSync(path.join(OUT, 'text', key + '.txt'), info.text, 'utf8');
  if (info.sw > info.cw + 1) note(role, vp, label, 'h-overflow', `scrollWidth ${info.sw} > ${info.cw}; ${info.wide.join(' | ')}`);
  if (VI.test(info.text)) note(role, vp, label, 'vietnamese?', info.text.match(/.{0,30}[À-ỹ].{0,30}/)?.[0]);
  if (MOJIBAKE.test(info.text)) note(role, vp, label, 'mojibake', info.text.match(/.{0,30}(Ã.|Â.|â€|�).{0,30}/)?.[0]);
  if (DEV.test(info.text)) note(role, vp, label, 'dev-wording', info.text.match(/.{0,40}(TanStack|coming in|sample data|lorem|TODO|undefined|\[object|NaN|null\b|mock).{0,40}/i)?.[0]);
  if (SHOTS) await page.screenshot({ path: path.join(OUT, 'shots', key + '.png') });
  return info.text;
}

async function walk(browser, role, vp) {
  const { ctx, page, setRoute } = await login(browser, role, vp);
  for (const r of ROUTES) {
    setRoute(r);
    await page.goto(BASE + r);
    await inspect(page, role, vp, r, r);
  }
  // Detail pages: rows are click targets (no anchors), so click row i and record where it lands.
  for (const list of ['/kyc', '/workflows', '/offers']) {
    setRoute(list);
    await page.goto(BASE + list);
    await page.waitForLoadState('networkidle').catch(() => {});
    const n = Math.min(await page.locator('table tbody tr').count(), 8);
    for (let i = 0; i < n; i++) {
      await page.goto(BASE + list);
      await page.waitForLoadState('networkidle').catch(() => {});
      await page.locator('table tbody tr').nth(i).locator('button, td').last().click().catch(() => {});
      await page.waitForTimeout(500);
      const u = new URL(page.url()).pathname;
      if (u === list) { note(role, vp, list, 'row-click-no-navigation', 'row ' + i); continue; }
      setRoute(u);
      const txt = await inspect(page, role, vp, u, `${list}_row${i}`);
      if (list === '/offers') {
        // click every service inside the offer detail, check each panel changes
        const svc = page.locator('main button').filter({ hasText: /^[A-Z0-9_]{4,}$/ });
        const k = await svc.count();
        for (let j = 0; j < k; j++) {
          await svc.nth(j).click();
          await page.waitForTimeout(150);
          fs.writeFileSync(path.join(OUT, 'text', `${role}_${vp}_offer${i}_svc${j}.txt`), await page.locator('main').innerText(), 'utf8');
        }
      }
    }
  }  // Service modal: every service, every tab (superadmin only: ops cannot reach /services).
  if (role === 'superadmin') {
    setRoute('/services');
    await page.goto(BASE + '/services');
    await page.waitForLoadState('networkidle').catch(() => {});
    const n = await page.locator('table tbody tr').count();
    for (let i = 0; i < n; i++) {
      await page.goto(BASE + '/services');
      await page.waitForLoadState('networkidle').catch(() => {});
      const row = page.locator('table tbody tr').nth(i);
      const btn = row.getByRole('button', { name: /Pipeline/ }).first();
      await btn.click().catch(() => {});
      const dlg = page.getByRole('dialog');
      if (!(await dlg.count())) { note(role, vp, '/services', 'no-dialog', 'row ' + i); continue; }
      const tabs = await dlg.locator('div.overflow-x-auto button').allTextContents();
      for (let t = 0; t < tabs.length; t++) {
        await dlg.locator('div.overflow-x-auto button').nth(t).click();
        await page.waitForTimeout(200);
        const txt = await dlg.innerText();
        fs.writeFileSync(path.join(OUT, 'text', `${role}_${vp}_service${i}_tab${t}.txt`), txt, 'utf8');
        const box = await dlg.boundingBox();
        if (box && (box.height > VIEWPORTS[vp].height || box.width > VIEWPORTS[vp].width)) note(role, vp, '/services', 'modal-too-big', `service ${i} tab ${t}: ${Math.round(box.width)}x${Math.round(box.height)}`);
      }
      await page.keyboard.press('Escape');
      if (await dlg.count()) note(role, vp, '/services', 'escape-does-not-close', 'service ' + i);
    }
  }
  await page.goto(BASE + '/no-such-page');
  await inspect(page, role, vp, '/no-such-page', '404');
  // Session expiry: remove the session and reload a deep link.
  await page.evaluate(() => sessionStorage.clear());
  await page.goto(BASE + '/offers');
  await page.waitForTimeout(1000);
  if (!page.url().includes('/login')) note(role, vp, '/offers', 'no-login-redirect', page.url());
  await ctx.close();
}

const browser = await chromium.launch();
for (const role of Object.keys(ROLES)) {
  for (const vp of Object.keys(VIEWPORTS)) {
    try { await walk(browser, role, vp); } catch (e) { note(role, vp, '-', 'script-error', e.message); }
  }
}
await browser.close();
fs.writeFileSync(path.join(OUT, 'findings.json'), JSON.stringify(findings, null, 2), 'utf8');
console.log(`done: ${findings.length} raw findings -> ${OUT}`);

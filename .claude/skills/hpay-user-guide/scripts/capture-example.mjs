// Pattern for one guide screenshot with numbered red callouts. Copy to %TEMP%, adapt, run with:
//   cd src/admin && node <path-to-copy>.mjs
// (Playwright is a devDependency of src/admin; run `npx playwright install chromium` once.)
// The admin UI and the API must already be running on demo data. Nothing here writes data.
import { chromium } from 'playwright';
import fs from 'node:fs';

const BASE = process.env.ADMIN_URL ?? 'http://localhost:5173';
const OUT = process.env.OUT_DIR ?? 'docs/guides/admin/img';
const USER = process.env.ADMIN_EMAIL ?? 'e2e-checker@HPay.dev'; // demo superadmin, practice system only
const PASS = process.env.ADMIN_PASSWORD ?? 'E2ETest@123'; // demo password, practice system only

/** Draw numbered red badges and a 3 px outline over elements found by a CSS or text selector. */
async function mark(page, items) {
  await page.evaluate((items) => {
    document.querySelectorAll('[data-guide-mark]').forEach((n) => n.remove());
    items.forEach((it, i) => {
      const el = it.css
        ? document.querySelector(it.css)
        : [...document.querySelectorAll('button, a, label, th, h1, h2, h3, strong, span, div')].find(
            (n) => n.children.length === 0 && n.textContent.trim() === it.text,
          );
      if (!el) throw new Error('guide mark: element not found: ' + (it.css ?? it.text));
      const r = el.getBoundingClientRect();
      const box = document.createElement('div');
      box.dataset.guideMark = '1';
      Object.assign(box.style, {
        position: 'fixed', left: r.left - 3 + 'px', top: r.top - 3 + 'px', width: r.width + 6 + 'px', height: r.height + 6 + 'px',
        outline: '3px solid #e11d48', borderRadius: '6px', zIndex: 99998, pointerEvents: 'none',
      });
      const badge = document.createElement('div');
      badge.dataset.guideMark = '1';
      badge.textContent = String(i + 1);
      Object.assign(badge.style, {
        position: 'fixed', left: r.left - 14 + 'px', top: r.top - 14 + 'px', width: '24px', height: '24px', borderRadius: '50%',
        background: '#e11d48', color: '#fff', font: 'bold 13px sans-serif', display: 'flex', alignItems: 'center',
        justifyContent: 'center', zIndex: 99999, pointerEvents: 'none',
      });
      document.body.append(box, badge);
    });
  }, items);
}

const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
fs.mkdirSync(OUT, { recursive: true });

await page.goto(BASE + '/login');
await page.getByLabel('Email').fill(USER);
await page.getByLabel('Password').fill(PASS);
await page.getByRole('button', { name: 'Log in' }).click();
await page.waitForURL(/dashboard/);

await page.goto(BASE + '/attention');
await page.waitForLoadState('networkidle');
await mark(page, [{ text: 'Needs attention' }]);
await page.screenshot({ path: `${OUT}/example.png` });

await browser.close();
// Builds the static site: site/pages/*.html + shared header/footer -> site/dist/.
// No dependencies. Run: node site/build.mjs
import { readFileSync, writeFileSync, mkdirSync, readdirSync, cpSync, rmSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(fileURLToPath(import.meta.url));
const dist = join(root, 'dist');
const REPO = 'https://github.com/hieumai1507/HPay';

const NAV = [
  ['index.html', 'Overview'],
  ['product.html', 'Product'],
  ['technical.html', 'Technical'],
  ['quality.html', 'Quality'],
  ['run.html', 'Run it'],
  ['honest.html', 'Honest notes'],
];

const header = (file, title) => `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title}</title>
<meta name="description" content="HPay: a fintech e-wallet built end to end. Go backend with a double-entry ledger, React admin console, Expo mobile app.">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500&family=IBM+Plex+Sans:wght@400;500;600;700&display=swap">
<link rel="stylesheet" href="assets/style.css">
<script src="assets/app.js" defer></script>
</head>
<body>
<a class="skip" href="#main">Skip to content</a>
<header class="top"><div class="wrap">
  <a class="logo" href="index.html"><b aria-hidden="true">H</b>HPay</a>
  <nav class="main" aria-label="Main">
${NAV.map(([f, l]) => `    <a href="${f}"${f === file ? ' aria-current="page"' : ''}>${l}</a>`).join('\n')}
    <a href="${REPO}" rel="noopener">GitHub</a>
    <button class="theme" id="theme" type="button" aria-label="Switch light or dark theme">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/></svg>
    </button>
  </nav>
</div></header>
<main id="main">
`;

const footer = `</main>
<footer><div class="wrap">
  <div>HPay is a demo project. All data shown is seeded demo data.</div>
  <div><a href="${REPO}" rel="noopener">Source on GitHub</a> · <a href="${REPO}/blob/main/docs/README.md" rel="noopener">Knowledge base</a> · <a href="${REPO}/tree/main/docs/adr" rel="noopener">Decision records</a></div>
</div></footer>
</body>
</html>
`;

if (existsSync(dist)) rmSync(dist, { recursive: true });
mkdirSync(dist, { recursive: true });
cpSync(join(root, 'assets'), join(dist, 'assets'), { recursive: true });
cpSync(join(root, 'img'), join(dist, 'img'), { recursive: true });

for (const f of readdirSync(join(root, 'pages'))) {
  if (!f.endsWith('.html')) continue;
  let body = readFileSync(join(root, 'pages', f), 'utf8').replace(/\r\n/g, '\n');
  const m = body.match(/^<!--title:(.*?)-->\n/);
  const title = m ? m[1].trim() : 'HPay';
  if (m) body = body.slice(m[0].length);
  body = body.replaceAll('{{REPO}}', REPO);
  writeFileSync(join(dist, f), header(f, title) + body + footer);
}
console.log('site built:', readdirSync(dist).filter((x) => x.endsWith('.html')).join(', '));

// Checks the admin user guide. No dependencies. Run from the repo root:
//   node docs/guides/admin/check-guide.mjs
// Fails (exit 1) when an image link is missing, an image file is unreferenced, the HTML copy lacks an
// image or contains an external request, or a forbidden word appears in the guide.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url)); // docs/guides/admin
const guidesDir = path.dirname(here);
const mdPath = path.join(guidesDir, 'admin-user-guide.md');
const htmlPath = path.join(guidesDir, 'admin-user-guide.html');
const imgDir = path.join(here, 'img');

const md = fs.readFileSync(mdPath, 'utf8').replace(/\r\n/g, '\n');
const html = fs.readFileSync(htmlPath, 'utf8');
const failures = [];
const fail = (m) => failures.push(m);

// 1. Image links in the markdown.
const links = [...md.matchAll(/!\[[^\]]*\]\(([^)]+)\)/g)].map((m) => m[1]);
for (const l of links) if (!fs.existsSync(path.join(guidesDir, l))) fail(`missing image file for link: ${l}`);

// 2. Unreferenced image files.
const referenced = new Set(links.map((l) => path.basename(l)));
for (const f of fs.readdirSync(imgDir)) if (!referenced.has(f)) fail(`unreferenced image on disk: ${f}`);

// 3. Every image is embedded in the HTML, and every embedded image carries alt text.
const embedded = [...html.matchAll(/<img\b[^>]*>/g)].map((m) => m[0]);
if (embedded.length !== links.length) fail(`HTML has ${embedded.length} images, markdown has ${links.length}`);
for (const tag of embedded) {
  if (!/src="data:image\/png;base64,[A-Za-z0-9+/=]{100,}"/.test(tag)) fail('an <img> in the HTML is not an embedded PNG');
  if (!/alt="[^"]{5,}"/.test(tag)) fail('an <img> in the HTML has no alt text');
}
for (const l of links) {
  const b64 = fs.readFileSync(path.join(guidesDir, l)).toString('base64');
  if (!html.includes(b64)) fail(`HTML is missing the picture ${l}`);
}
const sizeMb = fs.statSync(htmlPath).size / 1048576;
if (sizeMb > 6) fail(`HTML is ${sizeMb.toFixed(2)} MB, limit is 6 MB`);
for (const f of fs.readdirSync(imgDir)) {
  const kb = fs.statSync(path.join(imgDir, f)).size / 1024;
  if (kb > 250) fail(`image over 250 KB: ${f} (${Math.round(kb)} KB)`);
}

// 4. No external requests in the HTML (the plain-text demo URL is allowed).
const stripped = html.replace(/http:\/\/localhost:5173/g, '');
const ext = stripped.match(/https?:\/\/[^\s"'<>)]+/g);
if (ext) fail(`HTML contains external URLs: ${[...new Set(ext)].join(', ')}`);

// 5. Forbidden content in the markdown.
if (/[À-ÖØ-öø-ɏḀ-ỿ]/.test(md)) fail('markdown contains a Vietnamese (accented Latin) letter');
if (/sample data/i.test(md)) fail('markdown contains "sample data"');
if (/tanstack/i.test(md)) fail('markdown contains "TanStack"');
if (/lorem/i.test(md)) fail('markdown contains "lorem"');
if (md.charCodeAt(0) === 0xfeff) fail('markdown starts with a BOM');

// 6. "API coming in A3" is allowed only inside a "Not available yet" callout (a blockquote that starts with it).
const blocks = md.split(/\n{2,}/);
let blockIsNotAvailable = false;
for (const block of blocks) {
  if (/^>/.test(block)) blockIsNotAvailable = /^>\s*\*\*Not available yet\*\*/.test(block) || (blockIsNotAvailable && /^>/.test(block));
  if (/API coming in A3/.test(block) && !/^>\s*\*\*Not available yet\*\*/.test(block)) fail('"API coming in A3" appears outside a Not available yet callout');
}

// 7. Every numbered list has at most 9 steps; the first lines carry the version and the what-changed section.
let run = 0;
let maxRun = 0;
for (const line of md.split('\n')) {
  if (/^\d+\.\s/.test(line)) { run++; maxRun = Math.max(maxRun, run); } else if (line.trim() === '' ) { /* blank lines keep the list */ } else if (!/^\s/.test(line)) { run = 0; }
}
if (maxRun > 9) fail(`a numbered list has ${maxRun} steps (limit 9)`);
if (!/^Version: 2026-11-24/m.test(md)) fail('version line missing or not dated 2026-11-24');
if (!/^## What changed in this version/m.test(md)) fail('section "What changed in this version" is missing');

// 8. Figure numbers run 1..N in order, once each.
const figs = [...md.matchAll(/^\*Figure (\d+):/gm)].map((m) => Number(m[1]));
figs.forEach((n, i) => { if (n !== i + 1) fail(`figure numbering broken at position ${i + 1}: found Figure ${n}`); });
if (figs.length !== links.length) fail(`${figs.length} captions for ${links.length} images`);

// 9. Warning only: bold labels that look like buttons but are not in the admin source.
const srcDir = path.resolve(guidesDir, '..', '..', 'src', 'admin', 'src');
const warnings = [];
if (fs.existsSync(srcDir)) {
  const walk = (d) => fs.readdirSync(d, { withFileTypes: true }).flatMap((e) => (e.isDirectory() ? walk(path.join(d, e.name)) : [path.join(d, e.name)]));
  const source = walk(srcDir).filter((f) => /\.(tsx?|css)$/.test(f) && !/\.test\./.test(f)).map((f) => fs.readFileSync(f, 'utf8')).join('\n');
  const labels = new Set([...md.matchAll(/\*\*([^*]+)\*\*/g)].map((m) => m[1].trim()));
  for (const lab of labels) {
    if (/^(Tip|Careful|Not available yet|The safe rule|TO BE FILLED.*|You should see:?)$/i.test(lab)) continue;
    const core = lab.replace(/[ ]*[⚡→←✓✕☑⟳⤓↩🔒]+[ ]*/g, ' ').replace(/\s+/g, ' ').trim();
    if (core.length < 3 || core.length > 45 || /[:.?#]|^\d/.test(core) || /^(Task|What|If|When|Section|I |The |A |You )/.test(core)) continue;
    if (!source.includes(core) && !source.toLowerCase().includes(core.toLowerCase())) warnings.push(lab);
  }
}

if (warnings.length) console.log(`WARNING (labels not found in admin source; may be data, role names or Markdown headings): ${warnings.join(' | ')}`);
console.log(`images linked: ${links.length}, figures: ${figs.length}, html size: ${sizeMb.toFixed(2)} MB, longest numbered list: ${maxRun}`);
if (failures.length) {
  console.log('FAIL');
  for (const f of failures) console.log(' - ' + f);
  process.exit(1);
}
console.log('OK: all checks passed');

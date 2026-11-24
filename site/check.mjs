// Checks the built site: every local link and image exists, no mojibake, no BOM. Run after build.mjs.
import { readFileSync, readdirSync, existsSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
const dist = join(dirname(fileURLToPath(import.meta.url)), 'dist');
let bad = 0;
const fail = (m) => { console.error('FAIL', m); bad++; };
for (const f of readdirSync(dist).filter((x) => x.endsWith('.html'))) {
  const t = readFileSync(join(dist, f), 'utf8');
  if (t.charCodeAt(0) === 0xfeff) fail(f + ' starts with a BOM');
  if (/Â|â€|â†|Ã/.test(t)) fail(f + ' has mojibake');
  for (const m of t.matchAll(/(?:href|src)="([^"#?]+)(?:#[^"]*)?"/g)) {
    const u = m[1];
    if (/^(https?:|mailto:|\{\{)/.test(u)) continue;
    if (!existsSync(join(dist, u))) fail(f + ' -> missing ' + u);
  }
  for (const m of t.matchAll(/href="#([^"]+)"/g)) if (!t.includes('id="' + m[1] + '"')) fail(f + ' -> missing anchor #' + m[1]);
  if (t.includes('{{')) fail(f + ' has an unreplaced placeholder');
  for (const m of t.matchAll(/<img\b[^>]*>/g)) if (!/\balt="[^"]+"/.test(m[0])) fail(f + ' image without alt: ' + m[0].slice(0, 60));
}
if (bad) process.exit(1);
console.log('site check ok');
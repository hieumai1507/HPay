// Build docs/guides/<guide>.html from <guide>.md: one self-contained file, images embedded as
// base64, print CSS, no external requests, no dependencies. Usage (repo root):
//   node .claude/skills/hpay-user-guide/scripts/build-html.mjs docs/guides/admin-user-guide.md
import fs from 'node:fs';
import path from 'node:path';

const src = process.argv[2];
if (!src) { console.error('usage: node build-html.mjs <guide.md>'); process.exit(2); }
const dir = path.dirname(src);
const md = fs.readFileSync(src, 'utf8').replace(/\r\n/g, '\n');

const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
const inline = (s) => {
  const codes = [];
  s = s.replace(/`([^`]+)`/g, (_, c) => { codes.push(c); return `\u0000${codes.length - 1}\u0000`; });
  s = esc(s);
  s = s.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_, alt, href) => {
    const file = path.join(dir, href);
    if (!fs.existsSync(file)) throw new Error('missing image: ' + href);
    const b64 = fs.readFileSync(file).toString('base64');
    return `<img alt="${alt}" src="data:image/png;base64,${b64}">`;
  });
  s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, t, h) => `<a href="${h}">${t}</a>`);
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>').replace(/\*([^*]+)\*/g, '<em>$1</em>');
  return s.replace(/\u0000(\d+)\u0000/g, (_, i) => `<code>${esc(codes[Number(i)])}</code>`);
};

const lines = md.split('\n');
const out = [];
const toc = [];
let i = 0;
let list = null; // 'ul' | 'ol'
const closeList = () => { if (list) { out.push(`</${list}>`); list = null; } };
while (i < lines.length) {
  const line = lines[i];
  if (/^---\s*$/.test(line)) { closeList(); out.push('<hr>'); i++; continue; }
  let m = line.match(/^(#{1,4})\s+(.*)$/);
  if (m) {
    closeList();
    const level = m[1].length;
    const id = m[2].toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    if (level >= 2 && level <= 3) toc.push({ level, id, text: m[2] });
    out.push(`<h${level} id="${id}">${inline(m[2])}</h${level}>`);
    i++; continue;
  }
  if (/^>/.test(line)) {
    closeList();
    const block = [];
    while (i < lines.length && /^>/.test(lines[i])) { block.push(lines[i].replace(/^>\s?/, '')); i++; }
    const title = block[0]?.match(/^\*\*(.+)\*\*$/);
    const cls = title ? title[1].toLowerCase().replace(/[^a-z]+/g, '-') : 'note';
    out.push(`<blockquote class="callout ${cls}">${block.map((b) => inline(b)).join('<br>')}</blockquote>`);
    continue;
  }
  if (/^\|/.test(line) && /^\|[\s:|-]+\|\s*$/.test(lines[i + 1] ?? '')) {
    closeList();
    const cells = (l) => l.replace(/^\||\|\s*$/g, '').split('|').map((c) => c.trim());
    const head = cells(line); i += 2; const rows = [];
    while (i < lines.length && /^\|/.test(lines[i])) { rows.push(cells(lines[i])); i++; }
    out.push('<table><thead><tr>' + head.map((h) => `<th>${inline(h)}</th>`).join('') + '</tr></thead><tbody>' +
      rows.map((r) => '<tr>' + r.map((c) => `<td>${inline(c)}</td>`).join('') + '</tr>').join('') + '</tbody></table>');
    continue;
  }
  m = line.match(/^(\d+)\.\s+(.*)$/);
  if (m) { if (list !== 'ol') { closeList(); out.push('<ol>'); list = 'ol'; } out.push(`<li>${inline(m[2])}</li>`); i++; continue; }
  m = line.match(/^[-*]\s+(.*)$/);
  if (m) { if (list !== 'ul') { closeList(); out.push('<ul>'); list = 'ul'; } out.push(`<li>${inline(m[1])}</li>`); i++; continue; }
  if (line.trim() === '') { closeList(); i++; continue; }
  closeList();
  const para = [line]; i++;
  while (i < lines.length && lines[i].trim() !== '' && !/^(#{1,4}\s|>|\||\d+\.\s|[-*]\s|---)/.test(lines[i])) { para.push(lines[i]); i++; }
  out.push(`<p>${inline(para.join(' '))}</p>`);
}
closeList();

const title = (md.match(/^#\s+(.*)$/m) ?? [, 'Guide'])[1];
const css = `body{font:16px/1.55 system-ui,Segoe UI,sans-serif;max-width:860px;margin:0 auto;padding:24px 18px;color:#111827}
h1{font-size:28px}h2{margin-top:36px;border-bottom:2px solid #e5e7eb;padding-bottom:4px}h3{margin-top:26px}
img{max-width:100%;height:auto;border:1px solid #d1d5db;border-radius:6px;margin:8px 0}
table{border-collapse:collapse;width:100%;margin:12px 0;font-size:14px}th,td{border:1px solid #d1d5db;padding:6px 8px;text-align:left;vertical-align:top}th{background:#f3f4f6}
code{background:#f3f4f6;padding:1px 5px;border-radius:4px;font-size:.92em}
blockquote.callout{margin:14px 0;padding:10px 14px;border-left:6px solid #0e90a3;background:#ecfeff;border-radius:4px}
blockquote.careful,blockquote.the-safe-rule{border-color:#dc2626;background:#fef2f2}
blockquote.not-available-yet{border-color:#d97706;background:#fffbeb}
blockquote.tip{border-color:#16a34a;background:#f0fdf4}
nav.toc{background:#f9fafb;border:1px solid #e5e7eb;border-radius:8px;padding:10px 18px;margin:16px 0}nav.toc ul{list-style:none;padding-left:0}nav.toc li.l3{margin-left:18px}
@media print{body{max-width:none;padding:0}h2{page-break-before:always}h2:first-of-type{page-break-before:avoid}img,table,blockquote{page-break-inside:avoid}nav.toc{display:none}}`;
const nav = `<nav class="toc"><strong>Contents</strong><ul>${toc.map((t) => `<li class="l${t.level}"><a href="#${t.id}">${esc(t.text)}</a></li>`).join('')}</ul></nav>`;
const html = `<!doctype html>\n<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${esc(title)}</title><style>${css}</style></head><body>\n${out[0] ?? ''}\n${nav}\n${out.slice(1).join('\n')}\n</body></html>\n`;
const target = src.replace(/\.md$/, '.html');
fs.writeFileSync(target, html, 'utf8');
console.log(`wrote ${target} (${(Buffer.byteLength(html) / 1024 / 1024).toFixed(2)} MB, ${toc.length} headings)`);
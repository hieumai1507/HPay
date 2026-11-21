import * as fs from 'fs';
import * as path from 'path';

/**
 * C-01 / M06 guard: money is bigint end to end. This test fails if Number(), parseInt() or
 * parseFloat() is applied to anything named like an amount, and if a PIN literal is
 * hard-coded again (C-07). It is deliberately a dumb source scan: it is the same rule as the
 * no-restricted-syntax entry in eslint.config.js, but it also runs under `jest`.
 */

const ROOT = path.resolve(__dirname, '..', '..', '..');
const SCAN_DIRS = ['app', 'src'];

function listSources(dir: string, out: string[] = []): string[] {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === '__tests__' || entry.name === 'node_modules') continue;
      listSources(full, out);
    } else if (/\.(ts|tsx)$/.test(entry.name) && !entry.name.endsWith('.d.ts')) {
      out.push(full);
    }
  }
  return out;
}

function sources(): { file: string; lines: string[] }[] {
  return SCAN_DIRS.flatMap((d) => listSources(path.join(ROOT, d))).map((file) => ({
    file: path.relative(ROOT, file),
    lines: fs.readFileSync(file, 'utf8').split(/\r?\n/),
  }));
}

describe('TestEdge_C01_NoFloatMoney', () => {
  it('never applies Number()/parseInt()/parseFloat() to amount-like values', () => {
    const offenders: string[] = [];
    const re = /\b(Number|parseInt|parseFloat)\s*\([^)]*(amount|balance|fee|total|target)/i;
    for (const { file, lines } of sources()) {
      lines.forEach((line, i) => {
        if (re.test(line)) offenders.push(`${file}:${i + 1}: ${line.trim()}`);
      });
    }
    expect(offenders).toEqual([]);
  });
});

describe('TestEdge_C07_NoHardCodedPin', () => {
  it('has no hard-coded PIN literal in app code', () => {
    const offenders: string[] = [];
    const re = /['"]123456['"]/;
    for (const { file, lines } of sources()) {
      lines.forEach((line, i) => {
        if (re.test(line)) offenders.push(`${file}:${i + 1}: ${line.trim()}`);
      });
    }
    expect(offenders).toEqual([]);
  });
});

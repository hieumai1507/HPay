import { describe, expect, it } from 'vitest';

import { formatVnd, parseMinorUnits } from './money';

describe('admin money round trip (comma grouping)', () => {
  const safe = [0, 7, 999, 1000, 1234567, 250000000, 2500000000];

  it.each(safe)('parse(format(%s)) returns the same number', (x) => {
    const text = formatVnd(x);
    expect(text.endsWith(' ₫')).toBe(true);
    const r = parseMinorUnits(text.replace(/ ₫$/, ''));
    expect(r).toEqual({ ok: true, value: x });
  });

  it.each(safe)('format of %s keeps every digit', (x) => {
    expect(formatVnd(x).replace(/[^\d]/g, '')).toBe(String(x));
  });

  it('formats a bigint above Number.MAX_SAFE_INTEGER without losing digits', () => {
    const big = BigInt(Number.MAX_SAFE_INTEGER) * 1000n + 7n;
    expect(formatVnd(big).replace(/[^\d]/g, '')).toBe(big.toString());
    // above the parser bound: still a well-formed grouped number, rejected as too large
    expect(parseMinorUnits(formatVnd(big).replace(/ ₫$/, ''))).toEqual({ ok: false, reason: 'too_large' });
  });

  it('uses comma grouping with the dong sign after the number', () => {
    expect(formatVnd(1000000)).toBe('1,000,000 ₫');
  });
});
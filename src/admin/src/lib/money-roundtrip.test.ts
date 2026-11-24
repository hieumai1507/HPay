import { describe, expect, it } from 'vitest';

import { formatUsd, parseCents } from './money';

describe('admin money round trip (comma grouping, cents)', () => {
  const safe = [0, 1, 5, 7, 99, 100, 1000, 123456, 1234567, 250000000, 2500000000];

  it.each(safe)('parse(format(%s)) returns the same cents', (x) => {
    const text = formatUsd(x);
    expect(text.startsWith('$')).toBe(true);
    expect(parseCents(text.replace(/^\$/, ''))).toEqual({ ok: true, value: x });
  });

  it.each(safe)('format of %s keeps every digit', (x) => {
    const digits = formatUsd(x).replace(/[^\d]/g, '');
    expect(BigInt(digits)).toBe(BigInt(x));
  });

  it('formats a bigint above Number.MAX_SAFE_INTEGER without losing digits and the parser rejects it as too large', () => {
    const big = BigInt(Number.MAX_SAFE_INTEGER) * 1000n + 7n;
    expect(formatUsd(big).replace(/[^\d]/g, '')).toBe(big.toString());
    expect(parseCents(formatUsd(big).replace(/^\$/, ''))).toEqual({ ok: false, reason: 'too_large' });
  });

  it('writes the dollar sign first and always two decimals', () => {
    expect(formatUsd(1000000)).toBe('$10,000.00');
    expect(formatUsd(-123456)).toBe('-$1,234.56');
  });
});

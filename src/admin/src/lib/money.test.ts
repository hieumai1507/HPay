import { describe, expect, it } from 'vitest';

import { MAX_MINOR_UNITS, formatCount, formatMillions, formatVnd, parseMinorUnits, toMinorUnits } from './money';

describe('parseMinorUnits', () => {
  it('accepts whole non-negative integers', () => {
    expect(parseMinorUnits('0')).toEqual({ ok: true, value: 0 });
    expect(parseMinorUnits(' 100000000 ')).toEqual({ ok: true, value: 100000000 });
  });

  it('rejects empty, negative, NaN-like, decimal and exponent input', () => {
    expect(parseMinorUnits('')).toEqual({ ok: false, reason: 'empty' });
    expect(parseMinorUnits('-5')).toEqual({ ok: false, reason: 'negative' });
    for (const bad of ['abc', '1.5', '1e9', '1,00', '1,0000', ',100', '1,,000', '1.000.000', 'NaN', 'Infinity', '0x10', '+5']) {
      expect(parseMinorUnits(bad)).toEqual({ ok: false, reason: 'format' });
    }
  });

  it('accepts correct comma grouping', () => {
    expect(parseMinorUnits('1,000,000')).toEqual({ ok: true, value: 1000000 });
    expect(parseMinorUnits(' 250,000,000 ')).toEqual({ ok: true, value: 250000000 });
  });

  it('enforces the upper bound exactly', () => {
    expect(parseMinorUnits(MAX_MINOR_UNITS.toString()).ok).toBe(true);
    expect(parseMinorUnits((MAX_MINOR_UNITS + 1n).toString())).toEqual({ ok: false, reason: 'too_large' });
    expect(parseMinorUnits('99999999999999999999999')).toEqual({ ok: false, reason: 'too_large' });
  });
});

describe('toMinorUnits', () => {
  it('keeps safe integers and digit strings, drops everything else', () => {
    expect(toMinorUnits(5000000)).toBe(5000000);
    expect(toMinorUnits('5000000')).toBe(5000000);
    for (const bad of [1.5, -1, NaN, undefined, null, {}, '1.5', Number.MAX_SAFE_INTEGER + 2]) {
      expect(toMinorUnits(bad)).toBeNull();
    }
  });
});

describe('formatters', () => {
  it('formatVnd shows an em dash for missing values, not 0', () => {
    expect(formatVnd(null)).toBe('—');
    expect(formatVnd(undefined)).toBe('—');
    expect(formatVnd(1.5)).toBe('—');
    expect(formatVnd(0)).toContain('0');
    expect(formatVnd(1234567)).toBe('1,234,567 ₫');
  });
  it('formatCount shows an em dash for missing values', () => {
    expect(formatCount(null)).toBe('—');
    expect(formatCount(NaN)).toBe('—');
    expect(formatCount(0)).toBe('0');
  });
  it('formatMillions uses integer math', () => {
    expect(formatMillions(10000000)).toBe('10M');
    expect(formatMillions(40000000)).toBe('40M');
    expect(formatMillions(1500000)).toMatch(/₫$/);
    expect(formatMillions(-1)).toBe('—');
  });
});

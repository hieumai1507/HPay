import { describe, expect, it } from 'vitest';

import {
  MAX_MINOR_UNITS,
  centsToInputText,
  formatCount,
  formatThousands,
  formatUsd,
  moneyErrorMessage,
  parseCents,
  parseWholeNumber,
  toMinorUnits,
} from './money';

describe('parseCents (typed dollars to cents)', () => {
  it.each([
    ['0', 0],
    ['12', 1200],
    ['12.5', 1250],
    ['12.50', 1250],
    ['1,234.56', 123456],
    [' 1,000,000 ', 100000000],
    ['0.01', 1],
    ['0.99', 99],
    ['1.00', 100],
    ['0.05', 5],
    ['00.50', 50],
  ])('accepts %s as %s cents', (text, cents) => {
    expect(parseCents(text)).toEqual({ ok: true, value: cents });
  });

  it('rejects empty and negative input with their own reasons', () => {
    expect(parseCents('')).toEqual({ ok: false, reason: 'empty' });
    expect(parseCents('   ')).toEqual({ ok: false, reason: 'empty' });
    expect(parseCents('-1')).toEqual({ ok: false, reason: 'negative' });
    expect(parseCents('-0.50')).toEqual({ ok: false, reason: 'negative' });
  });

  it.each([
    '12.345',
    '1.234.567',
    '.5',
    '12.',
    '+5',
    '1e3',
    'abc',
    '12 .5',
    '1 2',
    '1,00',
    '1,0000',
    ',100',
    '1,,000',
    '1,234,56',
    '1,234.5.6',
    'NaN',
    'Infinity',
    '0x10',
    '$12',
    '12,5',
  ])('rejects %s as a bad format', (bad) => {
    expect(parseCents(bad)).toEqual({ ok: false, reason: 'format' });
  });

  it('enforces the upper bound exactly, in cents', () => {
    expect(parseCents('10000000000.00')).toEqual({ ok: true, value: 1_000_000_000_000 });
    expect(parseCents('10000000000.01')).toEqual({ ok: false, reason: 'too_large' });
    expect(parseCents('99999999999999999999999')).toEqual({ ok: false, reason: 'too_large' });
    expect(parseCents((MAX_MINOR_UNITS / 100n).toString()).ok).toBe(true);
  });

  it('refuses a bound that is not a safe integer', () => {
    expect(() => parseCents('1', BigInt(Number.MAX_SAFE_INTEGER) + 1n)).toThrow(RangeError);
  });
});

describe('parseWholeNumber (counts such as seconds)', () => {
  it('accepts digits with correct grouping and rejects decimals', () => {
    expect(parseWholeNumber('0')).toEqual({ ok: true, value: 0 });
    expect(parseWholeNumber('1,000')).toEqual({ ok: true, value: 1000 });
    expect(parseWholeNumber('1.5')).toEqual({ ok: false, reason: 'format' });
    expect(parseWholeNumber('')).toEqual({ ok: false, reason: 'empty' });
    expect(parseWholeNumber('-5')).toEqual({ ok: false, reason: 'negative' });
    expect(parseWholeNumber('86401', 86_400n)).toEqual({ ok: false, reason: 'too_large' });
  });
});

describe('toMinorUnits (API values, already cents)', () => {
  it('keeps safe integers and digit strings, drops everything else', () => {
    expect(toMinorUnits(5000000)).toBe(5000000);
    expect(toMinorUnits('5000000')).toBe(5000000);
    for (const bad of [1.5, -1, NaN, undefined, null, {}, '1.5', Number.MAX_SAFE_INTEGER + 2]) {
      expect(toMinorUnits(bad)).toBeNull();
    }
  });
});

describe('formatUsd', () => {
  it.each([
    [0, '$0.00'],
    [1, '$0.01'],
    [5, '$0.05'],
    [99, '$0.99'],
    [100, '$1.00'],
    [123456, '$1,234.56'],
    [10000, '$100.00'],
    [4_000_000, '$40,000.00'],
    [-1, '-$0.01'],
    [-123456, '-$1,234.56'],
    [1_000_000_000_000, '$10,000,000,000.00'],
  ])('formats %s cents as %s', (cents, text) => {
    expect(formatUsd(cents)).toBe(text);
  });

  it('shows an em dash for missing or non-integer values, never $0.00', () => {
    expect(formatUsd(null)).toBe('—');
    expect(formatUsd(undefined)).toBe('—');
    expect(formatUsd(1.5)).toBe('—');
    expect(formatUsd(NaN)).toBe('—');
  });

  it('formats a bigint beyond the safe integer range without losing digits', () => {
    const big = BigInt(Number.MAX_SAFE_INTEGER) * 1000n + 7n;
    expect(formatUsd(big).replace(/[^\d]/g, '')).toBe(big.toString());
  });
});

describe('centsToInputText', () => {
  it('writes plain dollars with two decimals and no grouping', () => {
    expect(centsToInputText(0)).toBe('0.00');
    expect(centsToInputText(5)).toBe('0.05');
    expect(centsToInputText(123456)).toBe('1234.56');
    expect(centsToInputText(null)).toBe('');
  });
});

describe('formatCount', () => {
  it('shows an em dash for missing values', () => {
    expect(formatCount(null)).toBe('—');
    expect(formatCount(NaN)).toBe('—');
    expect(formatCount(0)).toBe('0');
  });
});

describe('formatThousands (tier threshold label, whole $1,000 steps)', () => {
  it('abbreviates exact multiples of $1,000 as $nK and shows others in full', () => {
    expect(formatThousands(200000)).toBe('$2K');
    expect(formatThousands(8000)).toBe('$80.00');
    expect(formatThousands(150000)).toBe('$1,500.00');
    expect(formatThousands(0)).toBe('$0.00');
    expect(formatThousands(-1)).toBe('—');
  });
});

describe('moneyErrorMessage', () => {
  it('talks about dollars, not whole units', () => {
    expect(moneyErrorMessage('format')).toContain('Dollars');
    expect(moneyErrorMessage('format')).toContain('two decimals');
    expect(moneyErrorMessage('too_large')).toContain('maximum');
  });
});

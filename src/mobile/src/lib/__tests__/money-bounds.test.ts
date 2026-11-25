import { MAX_AMOUNT, parseAmountStrict, parseCentsStrict } from '../money';

describe('TestEdge_C01_ParseAmountStrict_UpperBound', () => {
  it('has a maximum of $40,000.00 in cents', () => {
    expect(MAX_AMOUNT).toBe(4_000_000n);
  });

  it('accepts a normal typed amount as cents', () => {
    expect(parseAmountStrict('1,500.25')).toEqual({ ok: true, value: 150_025n });
  });

  it('accepts the smallest amounts: 1 cent, 99 cents, one dollar', () => {
    expect(parseAmountStrict('0.01')).toEqual({ ok: true, value: 1n });
    expect(parseAmountStrict('0.99')).toEqual({ ok: true, value: 99n });
    expect(parseAmountStrict('1')).toEqual({ ok: true, value: 100n });
  });

  it('rejects empty and zero', () => {
    expect(parseAmountStrict('')).toEqual({ ok: false, reason: 'empty' });
    expect(parseAmountStrict('0')).toEqual({ ok: false, reason: 'not_positive' });
    expect(parseAmountStrict('0.00')).toEqual({ ok: false, reason: 'not_positive' });
  });

  it('accepts exactly $40,000.00 and rejects one cent above it', () => {
    expect(parseAmountStrict('40,000.00')).toEqual({ ok: true, value: MAX_AMOUNT });
    expect(parseAmountStrict('40000')).toEqual({ ok: true, value: MAX_AMOUNT });
    expect(parseAmountStrict('40,000.01')).toEqual({ ok: false, reason: 'too_large' });
    expect(parseAmountStrict('40000.01')).toEqual({ ok: false, reason: 'too_large' });
  });

  it('rejects values beyond Number.MAX_SAFE_INTEGER without precision loss', () => {
    expect(parseAmountStrict('9007199254740993')).toEqual({ ok: false, reason: 'too_large' });
  });

  it('rejects a pasted "1.234.567" and three decimals', () => {
    expect(parseAmountStrict('1.234.567')).toEqual({ ok: false, reason: 'invalid' });
    expect(parseAmountStrict('12.345')).toEqual({ ok: false, reason: 'invalid' });
  });

  it('rejects signs, letters and misplaced separators', () => {
    for (const bad of ['-5', '+5', '.5', '12.', '1,23', '12,3456', ',123', '1,,234', '1,234,', '1 234', 'abc', '1e5']) {
      expect(parseAmountStrict(bad)).toEqual({ ok: false, reason: 'invalid' });
    }
  });

  it('accepts an optional leading dollar sign', () => {
    expect(parseAmountStrict('$1,500.00')).toEqual({ ok: true, value: 150_000n });
    expect(parseAmountStrict('1500')).toEqual({ ok: true, value: 150_000n });
  });
});
describe('parseCentsStrict (route params in cents)', () => {
  it('applies the same bounds to a cents string', () => {
    expect(parseCentsStrict('1')).toEqual({ ok: true, value: 1n });
    expect(parseCentsStrict('4000000')).toEqual({ ok: true, value: MAX_AMOUNT });
    expect(parseCentsStrict('4000001')).toEqual({ ok: false, reason: 'too_large' });
    expect(parseCentsStrict('0')).toEqual({ ok: false, reason: 'not_positive' });
    expect(parseCentsStrict('')).toEqual({ ok: false, reason: 'empty' });
    expect(parseCentsStrict('12.5')).toEqual({ ok: false, reason: 'invalid' });
  });
});

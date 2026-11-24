import { MAX_AMOUNT, parseAmountStrict } from '../money';

describe('TestEdge_C01_ParseAmountStrict_UpperBound', () => {
  it('accepts a normal formatted amount as bigint', () => {
    expect(parseAmountStrict('1.500.000')).toEqual({ ok: true, value: 1_500_000n });
  });

  it('rejects empty and zero', () => {
    expect(parseAmountStrict('')).toEqual({ ok: false, reason: 'empty' });
    expect(parseAmountStrict('0')).toEqual({ ok: false, reason: 'not_positive' });
  });

  it('accepts exactly the maximum and rejects one above it', () => {
    expect(parseAmountStrict(MAX_AMOUNT.toString())).toEqual({ ok: true, value: MAX_AMOUNT });
    expect(parseAmountStrict((MAX_AMOUNT + 1n).toString())).toEqual({ ok: false, reason: 'too_large' });
  });

  it('rejects values beyond Number.MAX_SAFE_INTEGER without precision loss', () => {
    expect(parseAmountStrict('9007199254740993')).toEqual({ ok: false, reason: 'too_large' });
  });
});

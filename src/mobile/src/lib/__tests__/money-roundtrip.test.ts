import { formatAmountDisplay, formatAmountUSD, formatAmountWhileTyping, parseAmountToBigInt } from '../money';

const AMOUNTS = [0n, 1n, 5n, 99n, 100n, 999n, 1000n, 123456n, 4_000_000n, 250000000n, BigInt(Number.MAX_SAFE_INTEGER) * 1000n + 7n];

describe('money format and parse round trip (cents <-> dollar text)', () => {
  it.each(AMOUNTS)('parse(format(%s)) is the same bigint', (x) => {
    expect(parseAmountToBigInt(formatAmountDisplay(x))).toBe(x);
    expect(parseAmountToBigInt(formatAmountUSD(x))).toBe(x);
  });

  it.each(AMOUNTS)('format of %s keeps every digit', (x) => {
    const digits = formatAmountDisplay(x).replace(/[,.]/g, '');
    expect(BigInt(digits)).toBe(x);
  });

  it('typing the displayed text back is stable', () => {
    for (const x of AMOUNTS) {
      const shown = formatAmountDisplay(x);
      expect(formatAmountWhileTyping(shown)).toBe(shown === '0.00' ? '0.00' : shown);
    }
  });

  it('uses comma grouping with the dollar sign before the number', () => {
    expect(formatAmountUSD(100000000n)).toBe('$1,000,000.00');
  });

  it('rejects a pasted "1.234.567" rather than reading it as a decimal', () => {
    expect(parseAmountToBigInt('1.234.567')).toBeNull();
    expect(parseAmountToBigInt('1.5')).toBe(150n);
  });
});
import { formatAmountDisplay, formatAmountVND, formatAmountWhileTyping, parseAmountToBigInt } from '../money';

const AMOUNTS = [0n, 7n, 999n, 1000n, 1234567n, 250000000n, 2500000000n, BigInt(Number.MAX_SAFE_INTEGER) * 1000n + 7n];

describe('money format and parse round trip (comma grouping)', () => {
  it.each(AMOUNTS)('parse(format(%s)) is the same bigint', (x) => {
    expect(parseAmountToBigInt(formatAmountDisplay(x))).toBe(x);
    expect(parseAmountToBigInt(formatAmountVND(x))).toBe(x);
    expect(parseAmountToBigInt(formatAmountWhileTyping(x.toString()))).toBe(x);
  });

  it.each(AMOUNTS)('format of %s keeps every digit', (x) => {
    expect(formatAmountDisplay(x).replace(/,/g, '')).toBe(x.toString());
  });

  it('uses comma grouping with the dong sign after the number', () => {
    expect(formatAmountVND(1000000n)).toBe('1,000,000 ₫');
    expect(formatAmountWhileTyping('1234567')).toBe('1,234,567');
  });

  it('rejects a pasted Vietnamese "1.234.567" rather than reading it as a decimal', () => {
    expect(parseAmountToBigInt('1.234.567')).toBeNull();
    expect(parseAmountToBigInt('1.5')).toBeNull();
  });
});
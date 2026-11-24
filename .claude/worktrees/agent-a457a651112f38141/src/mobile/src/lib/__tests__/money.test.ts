import {
  formatAmountDisplay,
  formatAmountVND,
  formatAmountWhileTyping,
  parseAmountToBigInt,
  sanitizeAmountInput,
} from '../money';

describe('TestEdge_M06_Money_BigIntOnly', () => {
  it('parses beyond Number.MAX_SAFE_INTEGER without precision loss', () => {
    // Number('9007199254740993') rounds to 9007199254740992 — BigInt must not.
    const raw = '9007199254740993';
    const parsed = parseAmountToBigInt(raw);
    expect(parsed).toBe(9007199254740993n);
    expect(parsed?.toString()).toBe(raw);
  });

  it('sanitizes pasted input, dropping every non-digit character', () => {
    expect(sanitizeAmountInput('1,234,567')).toBe('1234567');
    expect(sanitizeAmountInput('  1.234.567 ₫')).toBe('1234567');
    expect(sanitizeAmountInput('abc123xyz456')).toBe('123456');
    expect(sanitizeAmountInput('💰1e5')).toBe('15'); // 'e' stripped — never treated as scientific notation
  });

  it('formats thousands separators live while typing', () => {
    expect(formatAmountWhileTyping('1')).toBe('1');
    expect(formatAmountWhileTyping('12')).toBe('12');
    expect(formatAmountWhileTyping('1234')).toBe('1.234');
    expect(formatAmountWhileTyping('1234567')).toBe('1.234.567');
    expect(formatAmountWhileTyping('01234')).toBe('1.234'); // leading zero stripped
    expect(formatAmountWhileTyping('')).toBe('');
  });

  it('formats a BigInt amount for read-only display with VND sign', () => {
    expect(formatAmountDisplay(1234567n)).toBe('1.234.567');
    expect(formatAmountDisplay(0n)).toBe('0');
    expect(formatAmountDisplay(-50000n)).toBe('-50.000');
    expect(formatAmountVND(1234567n)).toBe('1.234.567 ₫');
  });

  it('returns null for empty input instead of coercing to 0 via Number', () => {
    expect(parseAmountToBigInt('')).toBeNull();
    expect(parseAmountToBigInt('₫')).toBeNull();
  });

  it('never round-trips through a float — exact for values with 15+ significant digits', () => {
    const huge = '123456789012345678901234567890';
    expect(parseAmountToBigInt(huge)?.toString()).toBe(huge);
  });
});

import {
  formatAmountDisplay,
  formatAmountUSD,
  formatAmountWhileTyping,
  parseAmountToBigInt,
  parseCentsParam,
  sanitizeAmountInput,
} from '../money';

describe('TestEdge_M06_Money_BigIntOnly', () => {
  it('parses beyond Number.MAX_SAFE_INTEGER without precision loss', () => {
    const parsed = parseAmountToBigInt('9007199254740993');
    expect(parsed).toBe(900719925474099300n);
  });

  it('sanitizes pasted input to digits and dots only', () => {
    expect(sanitizeAmountInput('$1,234.56')).toBe('1234.56');
    expect(sanitizeAmountInput('abc123xyz456')).toBe('123456');
    expect(sanitizeAmountInput('1e5')).toBe('15'); // never scientific notation
  });

  it('never round-trips through a float for values with 15+ significant digits', () => {
    const huge = '123456789012345678901234567890';
    expect(parseAmountToBigInt(huge)).toBe(BigInt(huge) * 100n);
  });
});

describe('AC-3 parseAmountToBigInt (dollars typed, cents returned)', () => {
  it.each([
    ['12', 1200n],
    ['12.5', 1250n],
    ['12.50', 1250n],
    ['1,234.56', 123456n],
    ['$1,234.56', 123456n],
    ['$ 12', 1200n],
    ['0.05', 5n],
    ['0.01', 1n],
    ['0', 0n],
    ['0.00', 0n],
    ['99', 9900n],
    ['100', 10000n],
    ['  12.5  ', 1250n],
  ])('accepts %p as %p cents', (raw, cents) => {
    expect(parseAmountToBigInt(raw)).toBe(cents);
  });

  it.each(['12.345', '1.234.567', '.5', '12.', '-1', '+1', '1e3', 'abc', '', '$', '1,23', '1,,234', ',123', '1,234,', '1 234', '12,3456', '12.3.4', '$$5'])(
    'rejects %p',
    (raw) => {
      expect(parseAmountToBigInt(raw)).toBeNull();
    },
  );
});

describe('AC-2 formatting', () => {
  it('formats cents as US dollars', () => {
    expect(formatAmountUSD(123456n)).toBe('$1,234.56');
    expect(formatAmountUSD(5n)).toBe('$0.05');
    expect(formatAmountUSD(0n)).toBe('$0.00');
    expect(formatAmountUSD(100n)).toBe('$1.00');
    expect(formatAmountUSD(99n)).toBe('$0.99');
    expect(formatAmountUSD(1n)).toBe('$0.01');
    expect(formatAmountUSD(100000000n)).toBe('$1,000,000.00');
  });

  it('formats a negative amount with the sign before the dollar sign', () => {
    expect(formatAmountUSD(-123456n)).toBe('-$1,234.56');
    expect(formatAmountUSD(-5n)).toBe('-$0.05');
  });

  it('formatAmountDisplay has grouping and two decimals, no sign and no symbol', () => {
    expect(formatAmountDisplay(123456n)).toBe('1,234.56');
    expect(formatAmountDisplay(0n)).toBe('0.00');
    expect(formatAmountDisplay(-5000n)).toBe('50.00');
  });
});

describe('AC-3 typing sequence keeps a valid prefix', () => {
  it('builds 1,234.56 one key at a time', () => {
    let v = '';
    const steps: [string, string][] = [
      ['1', '1'],
      ['12', '12'],
      ['123', '123'],
      ['1234', '1,234'],
      ['1,234.', '1,234.'],
      ['1,234.5', '1,234.5'],
      ['1,234.56', '1,234.56'],
    ];
    for (const [typed, shown] of steps) {
      v = formatAmountWhileTyping(typed);
      expect(v).toBe(shown);
    }
  });

  it('ignores a third decimal digit', () => {
    expect(formatAmountWhileTyping('12.345')).toBe('12.34');
    expect(formatAmountWhileTyping('1,234.567')).toBe('1,234.56');
  });

  it('ignores a second dot and anything after it', () => {
    expect(formatAmountWhileTyping('1.2.3')).toBe('1.2');
    expect(formatAmountWhileTyping('1.234.567')).toBe('1.23');
  });

  it('trims leading zeros except before the point', () => {
    expect(formatAmountWhileTyping('01234')).toBe('1,234');
    expect(formatAmountWhileTyping('0')).toBe('0');
    expect(formatAmountWhileTyping('00')).toBe('0');
    expect(formatAmountWhileTyping('0.')).toBe('0.');
    expect(formatAmountWhileTyping('0.5')).toBe('0.5');
    expect(formatAmountWhileTyping('007.50')).toBe('7.50');
  });

  it('turns a leading dot into 0.', () => {
    expect(formatAmountWhileTyping('.')).toBe('0.');
    expect(formatAmountWhileTyping('.5')).toBe('0.5');
  });

  it('strips signs, letters and the dollar sign; empty stays empty', () => {
    expect(formatAmountWhileTyping('')).toBe('');
    expect(formatAmountWhileTyping('-5')).toBe('5');
    expect(formatAmountWhileTyping('$12')).toBe('12');
    expect(formatAmountWhileTyping('abc')).toBe('');
  });
});

describe('parseCentsParam (route params carry cents, not dollars)', () => {
  it('reads a plain digit string as cents', () => {
    expect(parseCentsParam('1250')).toBe(1250n);
    expect(parseCentsParam('0')).toBe(0n);
    expect(parseCentsParam('9007199254740993')).toBe(9007199254740993n);
  });

  it('rejects anything else', () => {
    for (const bad of ['', '12.5', '1,250', '-1', 'abc', '$5', undefined, null]) {
      expect(parseCentsParam(bad as string | undefined)).toBeNull();
    }
  });
});
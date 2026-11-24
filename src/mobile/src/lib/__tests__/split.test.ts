import { splitEven, validateCustomSplit } from '../split';

describe('TestEdge_M06_SplitBigIntRounding', () => {
  it('splits even numbers perfectly', () => {
    // $1,000.00 (100,000 cents) split among 4 people -> $250.00 each
    const result = splitEven(100000n, 4);
    expect(result).toEqual([25000n, 25000n, 25000n, 25000n]);
    expect(result.reduce((a, b) => a + b, 0n)).toBe(100000n);
  });

  it('AC-7: $10.00 split three ways is 334, 333, 333 cents', () => {
    const result = splitEven(1000n, 3);
    expect(result).toEqual([334n, 333n, 333n]);
    expect(result.reduce((a, b) => a + b, 0n)).toBe(1000n);
  });

  it('handles remainder correctly by spreading to the head (largest-remainder)', () => {
    const result = splitEven(100000n, 3);
    expect(result).toEqual([33334n, 33333n, 33333n]);
    expect(result.reduce((a, b) => a + b, 0n)).toBe(100000n);

    const result2 = splitEven(84000n, 9);
    expect(result2.reduce((a, b) => a + b, 0n)).toBe(84000n);
  });

  it('gives every person at least 1 cent when the total equals the head count', () => {
    expect(splitEven(3n, 3)).toEqual([1n, 1n, 1n]);
  });

  it('rejects a total smaller than the number of people', () => {
    expect(() => splitEven(2n, 3)).toThrow();
    expect(() => splitEven(0n, 1)).toThrow();
  });

  it('fails on negative totals or 0 participants', () => {
    expect(() => splitEven(-100n, 3)).toThrow();
    expect(() => splitEven(100n, 0)).toThrow();
  });

  it('validates custom split sum perfectly', () => {
    expect(validateCustomSplit(100n, [30n, 30n, 40n])).toBe(true);
    expect(validateCustomSplit(100n, [30n, 30n, 50n])).toBe(false);
    expect(validateCustomSplit(100n, [30n, -10n, 80n])).toBe(false); // negative part
    expect(validateCustomSplit(100n, [])).toBe(false);
  });
});
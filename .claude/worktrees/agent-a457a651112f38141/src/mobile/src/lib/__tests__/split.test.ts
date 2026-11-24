import { splitEven, validateCustomSplit } from '../split';

describe('TestEdge_M06_SplitBigIntRounding', () => {
  it('splits even numbers perfectly', () => {
    // 100,000 VND split among 4 people -> 25,000 each
    const result = splitEven(100000n, 4);
    expect(result).toEqual([25000n, 25000n, 25000n, 25000n]);
    expect(result.reduce((a, b) => a + b, 0n)).toBe(100000n);
  });

  it('handles remainder correctly by spreading to the head (largest-remainder)', () => {
    // 100,000 VND split among 3 people -> 33,334 for the first, 33,333 for others
    const result = splitEven(100000n, 3);
    expect(result).toEqual([33334n, 33333n, 33333n]);
    expect(result.reduce((a, b) => a + b, 0n)).toBe(100000n);

    // 840,000 VND split among 9 people
    const result2 = splitEven(840000n, 9);
    expect(result2.reduce((a, b) => a + b, 0n)).toBe(840000n);
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

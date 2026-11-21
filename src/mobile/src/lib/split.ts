/**
 * Split Bill math — pure BigInt, mirrors the backend's largest-remainder rule
 * (openapi /v1/money-requests/split). Client-side result is PREVIEW ONLY; the
 * server recomputes and is the source of truth. Money never touches Number/float (M06).
 */

/**
 * Even split of `total` minor units across `n` payers. The remainder is spread
 * one minor unit at a time from the head of the list (largest-remainder), so
 * the parts always sum to exactly `total`.
 */
export function splitEven(total: bigint, n: number): bigint[] {
  if (n <= 0) throw new Error('splitEven: n must be > 0');
  if (total < 0n) throw new Error('splitEven: total must be >= 0');
  const count = BigInt(n);
  const base = total / count;
  const remainder = total % count;
  return Array.from({ length: n }, (_, i) => (BigInt(i) < remainder ? base + 1n : base));
}

/** Custom split is valid only when every part is > 0 and the parts sum to exactly `total`. */
export function validateCustomSplit(total: bigint, parts: bigint[]): boolean {
  if (parts.length === 0) return false;
  if (parts.some((p) => p <= 0n)) return false;
  return parts.reduce((acc, p) => acc + p, 0n) === total;
}

/**
 * Money helpers for the admin app (constitution C-01).
 *
 * Amounts are integers in minor units (VND has no sub-unit, so 1 = 1 dong).
 * Inputs are parsed from strings with a strict digits-only grammar and an
 * explicit upper bound; there is no float arithmetic and no Number()/parseInt()
 * on user text. The single Number() below runs only after the value has been
 * proven <= MAX_SAFE_INTEGER, to hand a plain JSON integer to the API.
 */

/** Upper bound for a single configured amount: 1,000,000,000,000 minor units. */
export const MAX_MINOR_UNITS = 1_000_000_000_000n;

export type MoneyParseResult =
  | { ok: true; value: number }
  | { ok: false; reason: 'empty' | 'negative' | 'format' | 'too_large' };

export function parseMinorUnits(input: string, max: bigint = MAX_MINOR_UNITS): MoneyParseResult {
  if (max > BigInt(Number.MAX_SAFE_INTEGER)) throw new RangeError('max must be a safe integer');
  const text = input.trim();
  if (text === '') return { ok: false, reason: 'empty' };
  if (text.startsWith('-')) return { ok: false, reason: 'negative' };
  if (!/^\d+$/.test(text)) return { ok: false, reason: 'format' };
  const big = BigInt(text);
  if (big > max) return { ok: false, reason: 'too_large' };
  return { ok: true, value: Number(big) };
}

export function moneyErrorMessage(reason: Extract<MoneyParseResult, { ok: false }>['reason']): string {
  switch (reason) {
    case 'empty':
      return 'Enter an amount.';
    case 'negative':
      return 'Amount cannot be negative.';
    case 'format':
      return 'Digits only: a whole number with no decimals, spaces or letters.';
    case 'too_large':
      return 'Amount is above the allowed maximum.';
  }
}

/**
 * Accepts an API value that should be an integer amount: a safe non-negative
 * integer number, or a digits-only string. Anything else (float, NaN, negative,
 * undefined, object) becomes null so the UI shows an em dash instead of a
 * wrong number.
 */
export function toMinorUnits(value: unknown): number | null {
  if (typeof value === 'number') return Number.isSafeInteger(value) && value >= 0 ? value : null;
  if (typeof value === 'string') {
    const r = parseMinorUnits(value);
    return r.ok ? r.value : null;
  }
  return null;
}

/** Whole-million label for tier thresholds, integer math only: 10000000 -> "10M". Non-multiples show the full amount. */
export function formatMillions(value: number): string {
  if (!Number.isSafeInteger(value) || value < 0) return '—';
  const big = BigInt(value);
  return big % 1_000_000n === 0n ? `${big / 1_000_000n}M` : formatVnd(value);
}

/** Shared amount formatter. Missing or non-integer values render as an em dash, never as 0. */
export function formatVnd(value: number | bigint | null | undefined): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'number' && !Number.isSafeInteger(value)) return '—';
  return `${BigInt(value).toLocaleString('vi-VN')} ₫`;
}

/** Shared count/number formatter with the same missing-value rule. */
export function formatCount(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '—';
  return value.toLocaleString('vi-VN');
}
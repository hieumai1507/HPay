/**
 * Money helpers for the admin app (constitution C-01, ADR-0019).
 *
 * Amounts are integers in cents (1 = $0.01). Typed dollar text is parsed with a
 * strict grammar (digits, optional comma grouping like 1,234, optional one or two
 * decimals) and converted to cents by digit string and BigInt: there is no float
 * arithmetic and no Number()/parseInt() on user text. The single Number() below
 * runs only after the value has been proven <= MAX_SAFE_INTEGER, to hand a plain
 * JSON integer to the API.
 */

/** Upper bound for a single configured amount: 1,000,000,000,000 cents ($10,000,000,000.00). */
export const MAX_MINOR_UNITS = 1_000_000_000_000n;

export type MoneyParseResult =
  | { ok: true; value: number }
  | { ok: false; reason: 'empty' | 'negative' | 'format' | 'too_large' };

const WHOLE = String.raw`(?:\d+|\d{1,3}(?:,\d{3})+)`;
const DOLLARS_RE = new RegExp(`^(${WHOLE})(?:\\.(\\d{1,2}))?$`);
const WHOLE_RE = new RegExp(`^${WHOLE}$`);

function assertSafeBound(max: bigint): void {
  if (max > BigInt(Number.MAX_SAFE_INTEGER)) throw new RangeError('max must be a safe integer');
}

/** Typed dollars ("12", "12.5", "1,234.56") to integer cents. `max` is a bound in cents. */
export function parseCents(input: string, max: bigint = MAX_MINOR_UNITS): MoneyParseResult {
  assertSafeBound(max);
  const text = input.trim();
  if (text === '') return { ok: false, reason: 'empty' };
  if (text.startsWith('-')) return { ok: false, reason: 'negative' };
  const m = DOLLARS_RE.exec(text);
  if (!m) return { ok: false, reason: 'format' };
  const dollars = BigInt(m[1].replace(/,/g, ''));
  const cents = BigInt((m[2] ?? '').padEnd(2, '0'));
  const total = dollars * 100n + cents;
  if (total > max) return { ok: false, reason: 'too_large' };
  return { ok: true, value: Number(total) };
}

/** Typed whole number (counts such as seconds). Same grammar as parseCents without decimals. */
export function parseWholeNumber(input: string, max: bigint = MAX_MINOR_UNITS): MoneyParseResult {
  assertSafeBound(max);
  const text = input.trim();
  if (text === '') return { ok: false, reason: 'empty' };
  if (text.startsWith('-')) return { ok: false, reason: 'negative' };
  if (!WHOLE_RE.test(text)) return { ok: false, reason: 'format' };
  const big = BigInt(text.replace(/,/g, ''));
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
      return 'Dollars with at most two decimals, e.g. 12, 12.50 or 1,234.56: no signs, spaces or letters.';
    case 'too_large':
      return 'Amount is above the allowed maximum.';
  }
}

/**
 * Accepts an API value that should be an integer amount in cents: a safe
 * non-negative integer number, or a digits-only string. Anything else (float,
 * NaN, negative, undefined, object) becomes null so the UI shows an em dash
 * instead of a wrong number.
 */
export function toMinorUnits(value: unknown): number | null {
  if (typeof value === 'number') return Number.isSafeInteger(value) && value >= 0 ? value : null;
  if (typeof value === 'string') {
    const r = parseWholeNumber(value);
    return r.ok ? r.value : null;
  }
  return null;
}

/** Shared amount formatter: cents to "$1,234.56", "-$1,234.56". Missing or non-integer values render as an em dash, never $0.00. */
export function formatUsd(value: number | bigint | null | undefined): string {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'number' && !Number.isSafeInteger(value)) return '—';
  const big = BigInt(value);
  const abs = big < 0n ? -big : big;
  const dollars = (abs / 100n).toLocaleString('en-US');
  const cents = (abs % 100n).toString().padStart(2, '0');
  return `${big < 0n ? '-' : ''}$${dollars}.${cents}`;
}

/** Cents to the plain text of an editable dollar field: 123456 -> "1234.56" (no grouping, always two decimals). */
export function centsToInputText(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isSafeInteger(value) || value < 0) return '';
  const big = BigInt(value);
  return `${big / 100n}.${(big % 100n).toString().padStart(2, '0')}`;
}

/**
 * Tier-threshold label, integer math only. Exact multiples of $1,000 (and above zero) read "$2K";
 * everything else shows the full amount, e.g. $80.00. Negative or non-integer input is an em dash.
 */
export function formatThousands(cents: number): string {
  if (!Number.isSafeInteger(cents) || cents < 0) return '—';
  const big = BigInt(cents);
  return big > 0n && big % 100_000n === 0n ? `$${big / 100_000n}K` : formatUsd(big);
}

/** Shared count/number formatter with the same missing-value rule. */
export function formatCount(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '—';
  return value.toLocaleString('en-US');
}
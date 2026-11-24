/**
 * Money is always int64 minor units (VND dong — there is no sub-dong unit).
 * Every parse/format path here goes through BigInt. Never introduce a `Number(...)`
 * or `parseInt(...)` call on an amount string — that's how float bugs get in (M06).
 */

const THOUSANDS_SEPARATOR = ',';
const DIGITS_ONLY = /\d/g;

/** Strips everything that isn't a digit — used on paste and on every keystroke. */
export function sanitizeAmountInput(raw: string): string {
  return (raw.match(DIGITS_ONLY) ?? []).join('');
}

/** Plain digits, or digits grouped in threes with ',' ("1,234,567"). Nothing else is an amount. */
const AMOUNT_GRAMMAR = /^(?:\d+|\d{1,3}(?:,\d{3})+)$/;

/** True when the text is non-empty after trimming spaces and an optional trailing dong sign. */
function stripAmountDecorations(raw: string): string {
  return raw.trim().replace(/\s*₫$/, '').trim();
}

/**
 * Parses a typed or formatted amount ("1234567" or "1,234,567", optional trailing "₫") into
 * minor units. Returns null for empty input AND for anything that is not a whole number
 * written with correct comma grouping: decimals, negatives, a Vietnamese "1.234.567",
 * misplaced separators, letters. Never throws.
 */
export function parseAmountToBigInt(raw: string): bigint | null {
  const text = stripAmountDecorations(raw);
  if (!AMOUNT_GRAMMAR.test(text)) return null;
  return BigInt(text.replace(/,/g, ''));
}

/** Inserts `,` every 3 digits from the right, e.g. "1234567" -> "1,234,567". */
function groupThousands(digits: string): string {
  let result = '';
  for (let i = 0; i < digits.length; i++) {
    const posFromRight = digits.length - i;
    result += digits[i];
    if (posFromRight > 1 && posFromRight % 3 === 1) {
      result += THOUSANDS_SEPARATOR;
    }
  }
  return result;
}

/** Live-formats what the user is typing into an AmountInput (digits only, grouped). */
export function formatAmountWhileTyping(raw: string): string {
  const digits = sanitizeAmountInput(raw).replace(/^0+(?=\d)/, '');
  if (digits.length === 0) return '';
  return groupThousands(digits);
}

/** Formats a minor-unit amount for read-only display, e.g. 1234567n -> "1,234,567". */
export function formatAmountDisplay(value: bigint): string {
  const negative = value < 0n;
  const digits = (negative ? -value : value).toString();
  const grouped = groupThousands(digits);
  return negative ? `-${grouped}` : grouped;
}

/** Formats with the VND sign, e.g. 1234567n -> "1,234,567 ₫". */
export function formatAmountVND(value: bigint): string {
  return `${formatAmountDisplay(value)} ₫`;
}

/**
 * Upper bound for any single user-entered amount (C-01): 1,000,000,000 VND, the largest
 * per-transaction cap the backend applies (org disbursement). Everything below is far
 * under Number.MAX_SAFE_INTEGER, so API responses that arrive as JSON numbers stay exact.
 */
export const MAX_AMOUNT = 1_000_000_000n;

export type StrictAmount =
  | { ok: true; value: bigint }
  | { ok: false; reason: 'empty' | 'invalid' | 'not_positive' | 'too_large' };

/** Parses user input into a positive bigint no larger than MAX_AMOUNT. Never uses Number. */
export function parseAmountStrict(raw: string): StrictAmount {
  const value = parseAmountToBigInt(raw);
  if (value === null) {
    return { ok: false, reason: stripAmountDecorations(raw) === '' ? 'empty' : 'invalid' };
  }
  if (value <= 0n) return { ok: false, reason: 'not_positive' };
  if (value > MAX_AMOUNT) return { ok: false, reason: 'too_large' };
  return { ok: true, value };
}

/** Whole-number percentage of `part` in `whole`, clamped to 0..100, computed in bigint. */
export function percentOf(part: bigint, whole: bigint): number {
  if (whole <= 0n || part <= 0n) return 0;
  const pct = (part * 100n) / whole;
  return pct > 100n ? 100 : Number(pct);
}
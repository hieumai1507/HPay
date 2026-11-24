/**
 * Money is always int64 minor units: US cents (ADR-0019). Users type dollars; every
 * parse/format path here goes through BigInt on digit strings. Never introduce a
 * `Number(...)`, `parseInt(...)` or `parseFloat(...)` call on an amount string: that is how
 * float bugs get in (M06, C-01).
 */

const THOUSANDS_SEPARATOR = ',';
const CENTS_PER_DOLLAR = 100n;

/** Keeps only digits and dots; used on paste and on every keystroke. */
export function sanitizeAmountInput(raw: string): string {
  return raw.replace(/[^\d.]/g, '');
}

/**
 * Dollars as typed: an integer part (plain digits, or digits grouped in threes with ',')
 * and an optional '.' followed by one or two digits. Nothing else is an amount.
 */
const AMOUNT_GRAMMAR = /^(\d+|\d{1,3}(?:,\d{3})+)(?:\.(\d{1,2}))?$/;

/** Trims spaces and an optional leading dollar sign. */
function stripAmountDecorations(raw: string): string {
  return raw.trim().replace(/^\$\s*/, '').trim();
}

/**
 * Parses a typed or formatted dollar amount ("12", "12.5", "12.50", "1,234.56", optional
 * leading "$") into cents. Returns null for empty input and for anything else: three
 * decimals, "1.234.567", ".5", "12.", signs, letters, misplaced separators. Never throws.
 */
export function parseAmountToBigInt(raw: string): bigint | null {
  const match = AMOUNT_GRAMMAR.exec(stripAmountDecorations(raw));
  if (!match) return null;
  const dollars = BigInt(match[1].replace(/,/g, ''));
  const cents = BigInt((match[2] ?? '').padEnd(2, '0'));
  return dollars * CENTS_PER_DOLLAR + cents;
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

/**
 * Live-formats what the user is typing into an AmountInput. Always returns a valid prefix
 * of an amount: grouped integer part, optionally a '.' and at most two decimals. A second
 * dot and a third decimal are dropped; leading zeros are trimmed except before the point.
 */
export function formatAmountWhileTyping(raw: string): string {
  const cleaned = sanitizeAmountInput(raw);
  if (cleaned.length === 0) return '';
  const parts = cleaned.split('.');
  const hasDot = parts.length > 1;
  const intDigits = parts[0].replace(/^0+(?=\d)/, '');
  const decimals = hasDot ? parts[1].slice(0, 2) : '';
  if (intDigits === '' && !hasDot) return '';
  const grouped = groupThousands(intDigits === '' ? '0' : intDigits);
  return hasDot ? `${grouped}.${decimals}` : grouped;
}

/** Formats cents without a sign or symbol, e.g. 123456n -> "1,234.56" (magnitude only). */
export function formatAmountDisplay(value: bigint): string {
  const magnitude = value < 0n ? -value : value;
  const dollars = magnitude / CENTS_PER_DOLLAR;
  const cents = (magnitude % CENTS_PER_DOLLAR).toString().padStart(2, '0');
  return `${groupThousands(dollars.toString())}.${cents}`;
}

/** Formats cents as US dollars, e.g. 123456n -> "$1,234.56", -5n -> "-$0.05". */
export function formatAmountUSD(value: bigint): string {
  return `${value < 0n ? '-' : ''}$${formatAmountDisplay(value)}`;
}

/**
 * Upper bound for any single user-entered amount (C-01): 4,000,000 cents ($40,000.00),
 * the largest per-transaction cap the backend applies (org disbursement). Far under
 * Number.MAX_SAFE_INTEGER, so API responses that arrive as JSON numbers stay exact.
 */
export const MAX_AMOUNT = 4_000_000n;

export type StrictAmount =
  | { ok: true; value: bigint }
  | { ok: false; reason: 'empty' | 'invalid' | 'not_positive' | 'too_large' };

function checkBounds(value: bigint): StrictAmount {
  if (value <= 0n) return { ok: false, reason: 'not_positive' };
  if (value > MAX_AMOUNT) return { ok: false, reason: 'too_large' };
  return { ok: true, value };
}

/** Parses typed dollars into positive cents no larger than MAX_AMOUNT. Never uses Number. */
export function parseAmountStrict(raw: string): StrictAmount {
  const value = parseAmountToBigInt(raw);
  if (value === null) {
    return { ok: false, reason: stripAmountDecorations(raw) === '' ? 'empty' : 'invalid' };
  }
  return checkBounds(value);
}

/**
 * Route params and API values carry cents as a plain digit string. Returns null for
 * anything that is not one (so a dollar string is never misread as cents).
 */
export function parseCentsParam(raw: string | undefined | null): bigint | null {
  if (raw == null || !/^\d+$/.test(raw)) return null;
  return BigInt(raw);
}

/** Same bounds as parseAmountStrict, for a cents digit string. */
export function parseCentsStrict(raw: string | undefined | null): StrictAmount {
  if (raw == null || raw.trim() === '') return { ok: false, reason: 'empty' };
  const value = parseCentsParam(raw);
  if (value === null) return { ok: false, reason: 'invalid' };
  return checkBounds(value);
}

/** Whole-number percentage of `part` in `whole`, clamped to 0..100, computed in bigint. */
export function percentOf(part: bigint, whole: bigint): number {
  if (whole <= 0n || part <= 0n) return 0;
  const pct = (part * 100n) / whole;
  return pct > 100n ? 100 : Number(pct);
}
/**
 * Money is always int64 minor units (VND dong — there is no sub-dong unit).
 * Every parse/format path here goes through BigInt. Never introduce a `Number(...)`
 * or `parseInt(...)` call on an amount string — that's how float bugs get in (M06).
 */

const THOUSANDS_SEPARATOR = '.';
const DIGITS_ONLY = /\d/g;

/** Strips everything that isn't a digit — used on paste and on every keystroke. */
export function sanitizeAmountInput(raw: string): string {
  return (raw.match(DIGITS_ONLY) ?? []).join('');
}

/**
 * Parses a user-typed or formatted amount string into minor units.
 * Returns null for empty input; throws never — invalid input degrades to "0 digits kept".
 */
export function parseAmountToBigInt(raw: string): bigint | null {
  const digits = sanitizeAmountInput(raw).replace(/^0+(?=\d)/, '');
  if (digits.length === 0) return null;
  return BigInt(digits);
}

/** Inserts `.` every 3 digits from the right, e.g. "1234567" -> "1.234.567". */
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

/** Formats a minor-unit amount for read-only display, e.g. 1234567n -> "1.234.567". */
export function formatAmountDisplay(value: bigint): string {
  const negative = value < 0n;
  const digits = (negative ? -value : value).toString();
  const grouped = groupThousands(digits);
  return negative ? `-${grouped}` : grouped;
}

/** Formats with the VND sign, e.g. 1234567n -> "1.234.567 ₫". */
export function formatAmountVND(value: bigint): string {
  return `${formatAmountDisplay(value)} ₫`;
}

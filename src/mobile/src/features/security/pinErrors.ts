import { isApiError } from '@/src/lib/errors';

/** Where the Set PIN screen lives. Screens that hit ERR_PIN_NOT_SET offer this. */
export const SET_PIN_ROUTE = '/(main)/security/set-pin' as const;
/** Where the forgotten-PIN flow lives (PIN prompt and account link here). */
export const RESET_PIN_ROUTE = '/(main)/security/reset-pin' as const;

export const PIN_LENGTH = 6;

/** True when a money (or PIN) call failed because the user has no PIN yet. */
export function isPinNotSet(err: unknown): boolean {
  return isApiError(err) && err.code === 'ERR_PIN_NOT_SET';
}

/**
 * Client-side hint that mirrors the server rule (ValidatePIN in the backend): exactly
 * 6 digits, not all the same digit, not an ascending or descending run. The server stays
 * the authority (ERR_PIN_WEAK); this only lets the user fix a PIN before sending it.
 * Returns null when the PIN looks acceptable.
 */
export function weakPinReason(pin: string): string | null {
  if (!/^\d{6}$/.test(pin)) return 'Use exactly 6 digits.';
  let same = true;
  let asc = true;
  let desc = true;
  for (let i = 1; i < pin.length; i++) {
    const cur = pin.charCodeAt(i);
    const prev = pin.charCodeAt(i - 1);
    if (pin[i] !== pin[0]) same = false;
    if (cur !== prev + 1) asc = false;
    if (cur !== prev - 1) desc = false;
  }
  if (same) return 'Do not use the same digit six times.';
  if (asc || desc) return 'Do not use a run of digits such as 123456 or 654321.';
  return null;
}
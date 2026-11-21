import * as Crypto from 'expo-crypto';
import { useMemo } from 'react';

/** Generates one fresh Idempotency-Key. Call this once per money-moving flow, never per tap. */
export function generateIdempotencyKey(): string {
  return Crypto.randomUUID();
}

/**
 * Stable Idempotency-Key for one confirm flow (M01).
 *
 * The key is created once when the confirm screen mounts and stays the same across
 * re-renders and repeated taps of the submit button — that's what makes a double-tap
 * collapse into a single request instead of creating two transactions. Pass a
 * `resetKey` (e.g. `${receiverPhone}:${amountMinorUnits}`) so navigating to a *new*
 * transfer (different receiver/amount) gets a fresh key instead of reusing a stale one.
 *
 * Deliberately just `useMemo` keyed on `resetKey` — no ref-mutation-during-render,
 * which the new `react-hooks/refs` lint rule (and the React Compiler) disallow.
 */
export function useIdempotencyKey(resetKey: string): string {
  // eslint-disable-next-line react-hooks/exhaustive-deps -- regenerate exactly when resetKey changes, nothing else
  return useMemo(() => generateIdempotencyKey(), [resetKey]);
}

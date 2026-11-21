import { useCallback, useRef, useState } from 'react';

/**
 * Single-flight guard for every money-moving action (M01, C-03).
 *
 * `isPending` is React state, so it only flips after a re-render: two taps in the same
 * tick both see `false`. The ref flag is synchronous, so the second call is rejected
 * immediately. `run` resolves `undefined` when the call was swallowed by the guard and
 * rethrows the action's own error so the screen can show it; the guard is always released.
 */
export function useSubmitGuard() {
  const inFlight = useRef(false);
  const [isPending, setIsPending] = useState(false);

  const run = useCallback(async <T>(action: () => Promise<T>): Promise<T | undefined> => {
    if (inFlight.current) return undefined;
    inFlight.current = true;
    setIsPending(true);
    try {
      return await action();
    } finally {
      inFlight.current = false;
      setIsPending(false);
    }
  }, []);

  return { run, isPending };
}
import { useCallback, useRef, useState, type ReactElement } from 'react';

import { PinPrompt } from './components/PinPrompt';

/**
 * The single seam through which every money-moving screen obtains a PIN (C-07).
 *
 *   const { requestPin, pinPrompt } = usePin();
 *   const pin = await requestPin();      // null when the user cancels
 *   if (pin === null) return;
 *   await confirmTransaction({ transactionId, pin });
 *   ... render {pinPrompt} once in the screen ...
 *
 * The hook only COLLECTS the digits and hands them to the caller; it never validates or
 * stores them. The server-side PIN does not exist yet (ADR-0008), so the backend currently
 * ignores the `pin` field (audit BE-C2). When ADR-0008 lands, this is the only place that
 * needs to learn about set-up, hashing hints or lockout copy.
 */
export function usePin(): { requestPin: () => Promise<string | null>; pinPrompt: ReactElement } {
  const [visible, setVisible] = useState(false);
  const resolver = useRef<((pin: string | null) => void) | null>(null);

  const requestPin = useCallback(() => {
    // A second request while one is open cancels the first so no promise is left dangling.
    resolver.current?.(null);
    return new Promise<string | null>((resolve) => {
      resolver.current = resolve;
      setVisible(true);
    });
  }, []);

  const finish = useCallback((pin: string | null) => {
    const resolve = resolver.current;
    resolver.current = null;
    setVisible(false);
    resolve?.(pin);
  }, []);

  const pinPrompt = (
    <PinPrompt visible={visible} onSubmit={(pin) => finish(pin)} onCancel={() => finish(null)} />
  );

  return { requestPin, pinPrompt };
}
import { useCallback, useRef, useState, type ReactElement } from 'react';

import { useRouter } from 'expo-router';

import { RESET_PIN_ROUTE } from './pinErrors';
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
 * stores them. The server verifies the PIN (argon2id hash, attempt counter, 15 minute lockout,
 * ADR-0008/0012/0016); screens show the server's error through describeError. Set-up
 * (security/set-pin) and the forgotten-PIN flow (security/reset-pin) are separate screens;
 * the prompt links to the latter (docs/specs/w5-mobile-security/plan.md).
 */
export function usePin(): { requestPin: () => Promise<string | null>; pinPrompt: ReactElement } {
  const router = useRouter();
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
    <PinPrompt
      visible={visible}
      onSubmit={(pin) => finish(pin)}
      onCancel={() => finish(null)}
      onForgot={() => {
        finish(null); // cancel the pending payment step, then open the reset flow
        router.push(RESET_PIN_ROUTE);
      }}
    />
  );

  return { requestPin, pinPrompt };
}
import { useEffect } from 'react';
import * as ScreenCapture from 'expo-screen-capture';

/**
 * Blocks screenshots and screen recording while a screen is mounted (M07).
 *
 * Applied per screen rather than app-wide on purpose: people legitimately
 * screenshot receipts and QR codes, so only surfaces exposing balances, OTP
 * codes or KYC document photos opt in.
 *
 * The native module is unavailable in Expo Go, where these calls reject. That
 * must never take a screen down with it, so failures are swallowed — the app
 * simply runs without the protection, exactly as it did before.
 */
export function useSecureScreen(enabled = true) {
  useEffect(() => {
    if (!enabled) return;

    void ScreenCapture.preventScreenCaptureAsync().catch(() => {
      // Expo Go / unsupported platform — degrade quietly.
    });

    return () => {
      void ScreenCapture.allowScreenCaptureAsync().catch(() => {});
    };
  }, [enabled]);
}

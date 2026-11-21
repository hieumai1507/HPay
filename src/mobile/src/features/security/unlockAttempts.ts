/**
 * App-lock attempt accounting (C-07).
 *
 * The app lock no longer knows any PIN. It unlocks only when the OS confirms the user with
 * biometrics or the device credential (`LocalAuthentication.authenticateAsync`). After
 * MAX_UNLOCK_ATTEMPTS failed attempts the session is signed out.
 *
 * TODO(ADR-0008): once the server-side PIN exists, add a server verification step here
 * (attempt counter and lockout enforced by the server, not by this client). Until then the
 * lock is a device-level convenience gate, NOT an authentication of the account holder
 * against HPay. Nothing in this file may be made to "accept" a locally stored secret.
 */

export const MAX_UNLOCK_ATTEMPTS = 5;

export interface AuthResult {
  success: boolean;
  error?: string;
}

export type UnlockOutcome = 'unlocked' | 'failed' | 'cancelled' | 'signed_out';

export interface UnlockResult {
  outcome: UnlockOutcome;
  failedAttempts: number;
}

const CANCEL_ERRORS = new Set(['user_cancel', 'system_cancel', 'app_cancel']);

export async function runUnlockAttempt(
  authenticate: () => Promise<AuthResult>,
  failedAttemptsSoFar: number,
): Promise<UnlockResult> {
  let result: AuthResult;
  try {
    result = await authenticate();
  } catch {
    result = { success: false, error: 'hardware_error' };
  }

  if (result.success) return { outcome: 'unlocked', failedAttempts: 0 };
  if (result.error && CANCEL_ERRORS.has(result.error)) {
    return { outcome: 'cancelled', failedAttempts: failedAttemptsSoFar };
  }

  const failedAttempts = failedAttemptsSoFar + 1;
  return {
    outcome: failedAttempts >= MAX_UNLOCK_ATTEMPTS ? 'signed_out' : 'failed',
    failedAttempts,
  };
}
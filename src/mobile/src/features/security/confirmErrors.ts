import { SECURITY_ROUTES } from './routes';
import { describeError, isApiError } from '@/src/lib/errors';

import { AuthCancelledError } from '@/src/queries/useTransactionQueries';

/**
 * How a money screen reacts to a failed request/confirm. The copy always comes from
 * describeError (src/lib/errors.ts); this only decides what the screen DOES next.
 *
 *  - restart: the held request expired; go back to the amount screen.
 *  - blocked: the user cannot succeed by retrying now (limits, lock); stay, no retry button.
 *  - action:  offer one button that fixes the cause (set a PIN, turn on two-step verification).
 *  - cancelled: the user dismissed the prompt; show nothing.
 */
export interface ConfirmFailure {
  kind: 'cancelled' | 'restart' | 'blocked' | 'set-pin' | 'enroll-totp' | 'other';
  message: string;
  /** True when the primary button must be disabled so there is no retry loop. */
  blocking: boolean;
  actionLabel?: string;
  actionRoute?: string;
}

const BLOCKING = new Set([
  'ERR_LIMIT_DAILY',
  'ERR_LIMIT_MONTHLY',
  'ERR_LIMIT_PER_TXN',
  'ERR_TOO_FREQUENT',
  'ERR_PIN_LOCKED',
  'ERR_TOTP_LOCKED',
]);

export function classifyConfirmError(err: unknown): ConfirmFailure {
  if (err instanceof AuthCancelledError) {
    return { kind: 'cancelled', message: '', blocking: false };
  }
  const message = describeError(err);
  const code = isApiError(err) ? err.code : undefined;
  if (code === 'ERR_AUTH_EXPIRED') return { kind: 'restart', message, blocking: true };
  if (code && BLOCKING.has(code)) return { kind: 'blocked', message, blocking: true };
  if (code === 'ERR_PIN_NOT_SET') {
    return {
      kind: 'set-pin',
      message,
      blocking: false,
      actionLabel: 'Set PIN',
      actionRoute: SECURITY_ROUTES.setPin,
    };
  }
  if (code === 'ERR_TOTP_NOT_ENROLLED') {
    return {
      kind: 'enroll-totp',
      message,
      blocking: false,
      actionLabel: 'Turn on two-step verification',
      actionRoute: SECURITY_ROUTES.totpEnroll,
    };
  }
  return { kind: 'other', message, blocking: false };
}
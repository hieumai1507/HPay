import { describeError, isApiError } from '@/src/lib/errors';
import {
  AuthCancelledError,
  type EngineConfirmPayload,
  type EngineRequestResponse,
} from '@/src/queries/useTransactionQueries';

import type { Collect } from './useTransactionAuth';

/** Wrong or expired credential: the held transaction is still valid, so ask again. */
const RE_ASK = new Set([
  'ERR_PIN_INCORRECT',
  'ERR_OTP_INVALID',
  'ERR_OTP_EXPIRED',
  'ERR_TOTP_INVALID',
  'ERR_CREDENTIAL_REQUIRED',
]);
/** The SMS code is spent; a new one must be issued before asking again. */
const NEW_CODE = new Set(['ERR_OTP_EXPIRED', 'ERR_OTP_MAX_ATTEMPTS']);

const MAX_ASKS = 5;

/**
 * Confirm a held transaction: collect the factors of its band, confirm, and on a wrong
 * credential ask again for the SAME transaction (never a second request, so money cannot be
 * requested twice). Lockouts end the loop because the server answers ERR_PIN_LOCKED /
 * ERR_TOTP_LOCKED. Throws AuthCancelledError when the user dismisses the prompt.
 */
export async function confirmWithAuth<R>(args: {
  request: Pick<EngineRequestResponse, 'transaction_id' | 'auth_required'>;
  collect: Collect;
  confirm: (payload: EngineConfirmPayload) => Promise<R>;
}): Promise<R> {
  const { request, collect, confirm } = args;
  let notice: string | undefined;
  let skipChallenge = false;
  for (let i = 0; i < MAX_ASKS; i++) {
    const factors = await collect(
      request.auth_required,
      { transactionId: request.transaction_id },
      { notice, skipChallenge },
    );
    if (factors === null) throw new AuthCancelledError();
    try {
      return await confirm({ transactionId: request.transaction_id, ...factors });
    } catch (err) {
      // The pin band has no inline notice, so its error goes back to the screen instead.
      if (request.auth_required === 'pin' || !isApiError(err) || !RE_ASK.has(err.code)) throw err;
      notice = describeError(err);
      skipChallenge = !NEW_CODE.has(err.code);
    }
  }
  throw new AuthCancelledError();
}
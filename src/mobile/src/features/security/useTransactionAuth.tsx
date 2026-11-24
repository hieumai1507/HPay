import { useCallback, useEffect, useRef, useState, type ReactElement } from 'react';

import { describeError, isApiError } from '@/src/lib/errors';
import { useTransactionChallenge } from '@/src/queries/useTransactionQueries';

import { StepUpPrompt, type StepUpBand } from './components/StepUpPrompt';
import { usePin } from './usePin';

export type AuthRequired = 'none' | 'pin' | 'otp' | '2fa';

/** Exactly the factors of the band. A band never carries a factor of another band. */
export interface AuthFactors {
  pin?: string;
  otp?: string;
  totp?: string;
}

export interface CollectOptions {
  /** Message from the previous failed attempt, shown in the prompt. */
  notice?: string;
  /** Re-asking for the same held transaction: do not send another SMS code. */
  skipChallenge?: boolean;
}

export type Collect = (
  authRequired: AuthRequired,
  ctx: { transactionId: string },
  opts?: CollectOptions,
) => Promise<AuthFactors | null>;

/** Seconds before the backend lets another code be issued (OTPCooldown = 30 s). */
export const OTP_RESEND_SECONDS = 30;

interface PromptState {
  band: StepUpBand;
  notice: string | null;
}

/**
 * The single seam for step-up at confirm (ADR-0014, ADR-0017):
 *
 *   const { collect, prompt } = useTransactionAuth();
 *   const factors = await collect(req.auth_required, { transactionId: req.transaction_id });
 *   if (factors === null) return;           // user cancelled; nothing was sent
 *   await confirmTransaction({ transactionId, ...factors });
 *   ... render {prompt} once in the screen ...
 *
 * none -> {} without a prompt; pin -> PIN prompt (usePin); otp -> POST challenge, then PIN +
 * 6-digit code with a resend cooldown; 2fa -> PIN + authenticator or recovery code. Credentials
 * live only in the prompt's state until submitted and are never logged or stored (C-07, C-11).
 * A failed challenge request throws; the caller shows it through describeError.
 */
export function useTransactionAuth(): { collect: Collect; prompt: ReactElement } {
  const { requestPin, pinPrompt } = usePin();
  const challenge = useTransactionChallenge();
  const challengeRef = useRef(challenge.mutateAsync);
  challengeRef.current = challenge.mutateAsync;

  const [state, setState] = useState<PromptState | null>(null);
  const [otpDev, setOtpDev] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [resending, setResending] = useState(false);
  const txId = useRef('');
  const resolver = useRef<((f: AuthFactors | null) => void) | null>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((c) => Math.max(0, c - 1)), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  const settle = useCallback((f: AuthFactors | null) => {
    const r = resolver.current;
    resolver.current = null;
    setState(null);
    r?.(f);
  }, []);

  // Leaving the screen with a prompt open must not leave the caller waiting forever.
  useEffect(() => () => resolver.current?.(null), []);

  const issueCode = useCallback(async (transactionId: string) => {
    const res = await challengeRef.current(transactionId);
    setOtpDev(res.otp_dev ?? null);
    setCooldown(OTP_RESEND_SECONDS);
  }, []);

  const collect = useCallback<Collect>(
    async (authRequired, ctx, opts) => {
      if (authRequired === 'none') return {};
      if (authRequired === 'pin') {
        const pin = await requestPin();
        return pin === null ? null : { pin };
      }
      txId.current = ctx.transactionId;
      if (authRequired === 'otp' && !opts?.skipChallenge) {
        await issueCode(ctx.transactionId); // throws on ERR_RATE_LIMITED etc.
      }
      resolver.current?.(null);
      return new Promise<AuthFactors | null>((resolve) => {
        resolver.current = resolve;
        setState({ band: authRequired, notice: opts?.notice ?? null });
      });
    },
    [requestPin, issueCode],
  );

  const onResend = useCallback(async () => {
    setResending(true);
    try {
      await issueCode(txId.current);
      setState((s) => (s ? { ...s, notice: null } : s));
    } catch (err) {
      if (isApiError(err) && (err.code === 'ERR_OTP_COOLDOWN' || err.code === 'ERR_RATE_LIMITED')) {
        setCooldown(OTP_RESEND_SECONDS);
      }
      setState((s) => (s ? { ...s, notice: describeError(err) } : s));
    } finally {
      setResending(false);
    }
  }, [issueCode]);

  const prompt = (
    <>
      {pinPrompt}
      <StepUpPrompt
        visible={state !== null}
        band={state?.band ?? 'otp'}
        otpDev={otpDev}
        cooldown={cooldown}
        resending={resending}
        notice={state?.notice}
        onResend={() => void onResend()}
        onSubmit={({ pin, code }) =>
          settle(state?.band === 'otp' ? { pin, otp: code } : { pin, totp: code })
        }
        onCancel={() => settle(null)}
      />
    </>
  );

  return { collect, prompt };
}
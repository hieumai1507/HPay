import { useEffect, useState } from 'react';

import { ApiError, describeError } from '@/src/lib/errors';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { useConfirmPinReset, useRequestPinReset } from '@/src/queries/usePinQueries';

import { PIN_LENGTH, isPinNotSet, weakPinReason } from './pinErrors';

export const RESEND_COOLDOWN_SECONDS = 30;

export type ResetStep = 'password' | 'code' | 'done';

/** Logic of the forgotten-PIN flow. Secrets live only in this component state. */
export function useResetPin() {
  const requestMut = useRequestPinReset();
  const confirmMut = useConfirmPinReset();
  const guard = useSubmitGuard();

  const [step, setStep] = useState<ResetStep>('password');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [pin, setPin] = useState('');
  const [repeat, setRepeat] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [pinNotSet, setPinNotSet] = useState(false);
  /** The code can no longer be used (expired or 3 wrong tries): ask for a new one. */
  const [needsNewCode, setNeedsNewCode] = useState(false);
  const [devCode, setDevCode] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setInterval(() => setCooldown((c) => (c > 0 ? c - 1 : 0)), 1000);
    return () => clearInterval(t);
  }, [cooldown > 0]); // eslint-disable-line react-hooks/exhaustive-deps

  const pinHint = pin.length === PIN_LENGTH ? weakPinReason(pin) : null;
  const mismatch = repeat.length === PIN_LENGTH && repeat !== pin;
  const canRequest = password.length > 0 && cooldown === 0;
  const canConfirm =
    !needsNewCode && code.length === PIN_LENGTH && pin.length === PIN_LENGTH &&
    pinHint === null && repeat === pin;

  const requestCode = async () => {
    if (!canRequest) return;
    setError(null);
    setPinNotSet(false);
    try {
      const res = await guard.run(() => requestMut.mutateAsync({ password }));
      if (res === undefined) return;
      setPassword('');
      setCode('');
      setNeedsNewCode(false);
      setDevCode(res.otp_dev ?? null);
      setCooldown(RESEND_COOLDOWN_SECONDS);
      setStep('code');
    } catch (e) {
      if (e instanceof ApiError && e.code === 'ERR_OTP_COOLDOWN') setCooldown(RESEND_COOLDOWN_SECONDS);
      setPinNotSet(isPinNotSet(e));
      setError(describeError(e));
    }
  };

  const confirm = async () => {
    if (!canConfirm) return;
    setError(null);
    try {
      const res = await guard.run(() => confirmMut.mutateAsync({ code, newPin: pin }));
      if (res === undefined) return;
      setCode(''); setPin(''); setRepeat('');
      setDevCode(null);
      setStep('done');
    } catch (e) {
      const c = e instanceof ApiError ? e.code : '';
      if (c === 'ERR_OTP_MAX_ATTEMPTS' || c === 'ERR_OTP_EXPIRED') {
        setNeedsNewCode(true);
        setCode('');
      } else if (c === 'ERR_OTP_INVALID') {
        setCode('');
      }
      setError(describeError(e));
    }
  };

  /** Back to step 1 for a fresh code; the password must be typed again. */
  const startOver = () => {
    setStep('password');
    setNeedsNewCode(false);
    setCode(''); setPin(''); setRepeat('');
    setDevCode(null);
    setError(null);
  };

  return {
    step, password, setPassword, code, setCode, pin, setPin, repeat, setRepeat,
    pinHint, mismatch, canRequest, canConfirm, error, pinNotSet, needsNewCode,
    devCode, cooldown, isPending: guard.isPending,
    requestCode, confirm, startOver,
  };
}
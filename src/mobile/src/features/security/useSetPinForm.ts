import { useState } from 'react';

import { describeError } from '@/src/lib/errors';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { useSetPin } from '@/src/queries/usePinQueries';

import { PIN_LENGTH, weakPinReason } from './pinErrors';

/** Logic of the Set PIN screen. PIN and password live only in this component state. */
export function useSetPinForm() {
  const mutation = useSetPin();
  const guard = useSubmitGuard();
  const [password, setPassword] = useState('');
  const [pin, setPin] = useState('');
  const [repeat, setRepeat] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const pinHint = pin.length === PIN_LENGTH ? weakPinReason(pin) : null;
  const mismatch = repeat.length === PIN_LENGTH && repeat !== pin;
  const canSubmit =
    password.length > 0 && pin.length === PIN_LENGTH && pinHint === null && repeat === pin;

  const submit = async () => {
    if (!canSubmit) return;
    setError(null);
    try {
      const res = await guard.run(() => mutation.mutateAsync({ password, newPin: pin }));
      if (res === undefined) return; // swallowed double tap
      setPassword('');
      setPin('');
      setRepeat('');
      setDone(true);
    } catch (e) {
      setError(describeError(e));
    }
  };

  return {
    password, setPassword, pin, setPin, repeat, setRepeat,
    pinHint, mismatch, canSubmit, error, done,
    isPending: guard.isPending,
    submit,
  };
}
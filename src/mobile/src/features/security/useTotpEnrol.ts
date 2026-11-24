import { useCallback, useEffect, useRef, useState } from 'react';

import { describeError } from '@/src/lib/errors';
import { useSubmitGuard } from '@/src/lib/useSubmitGuard';
import { useTotpEnrolConfirm, useTotpEnrolStart } from '@/src/queries/useTotpQueries';

export type EnrolStep = 'password' | 'code' | 'recovery';

/** Groups of 4 so the secret is easy to type into an authenticator app. */
export function groupSecret(secret: string): string {
  return secret.replace(/\s+/g, '').replace(/(.{4})/g, '$1 ').trim();
}

/**
 * Logic of the authenticator enrolment (ADR-0017). The password, secret, first code and
 * recovery codes live only in this hook's state: never logged, stored or put in a route
 * param, and cleared on unmount (C-07, C-11). The screen only renders.
 */
export function useTotpEnrol() {
  const startM = useTotpEnrolStart();
  const confirmM = useTotpEnrolConfirm();
  const guard = useSubmitGuard();

  const [step, setStep] = useState<EnrolStep>('password');
  const [password, setPassword] = useState('');
  const [secret, setSecret] = useState('');
  const [uri, setUri] = useState('');
  const [code, setCode] = useState('');
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [saved, setSaved] = useState(false);
  const [error, setError] = useState<{ code?: string; message: string } | null>(null);

  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      // Drop every credential when the user leaves the screen.
      setPassword('');
      setSecret('');
      setUri('');
      setCode('');
      setRecoveryCodes([]);
    };
  }, []);

  const fail = (err: unknown) => {
    const code = (err as { code?: string } | null)?.code;
    setError({ code, message: describeError(err) });
  };

  const start = useCallback(
    () =>
      guard.run(async () => {
        if (password.length === 0) return;
        setError(null);
        try {
          const res = await startM.mutateAsync(password);
          setSecret(res.secret);
          setUri(res.otpauth_uri);
          setPassword('');
          setStep('code');
        } catch (e) {
          fail(e);
        }
      }),
    [guard, password, startM],
  );

  const confirm = useCallback(
    () =>
      guard.run(async () => {
        if (code.length !== 6) return;
        setError(null);
        try {
          const res = await confirmM.mutateAsync(code);
          setRecoveryCodes(res.recovery_codes);
          setSecret('');
          setUri('');
          setCode('');
          setStep('recovery');
        } catch (e) {
          fail(e);
        }
      }),
    [guard, code, confirmM],
  );

  return {
    step,
    password,
    setPassword,
    secret,
    groupedSecret: groupSecret(secret),
    uri,
    code,
    setCode: (v: string) => setCode(v.replace(/[^0-9]/g, '').slice(0, 6)),
    recoveryCodes,
    saved,
    setSaved,
    canFinish: step === 'recovery' && saved,
    error,
    isPending: guard.isPending,
    start,
    confirm,
  };
}
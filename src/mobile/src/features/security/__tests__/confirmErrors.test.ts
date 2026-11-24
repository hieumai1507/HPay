import { SECURITY_ROUTES } from '../routes';
import { classifyConfirmError } from '../confirmErrors';
import { ApiError } from '@/src/lib/errors';
import { AuthCancelledError } from '@/src/queries/useTransactionQueries';

describe('Test_classifyConfirmError', () => {
  it.each(['ERR_LIMIT_DAILY', 'ERR_LIMIT_MONTHLY', 'ERR_LIMIT_PER_TXN', 'ERR_TOO_FREQUENT', 'ERR_PIN_LOCKED', 'ERR_TOTP_LOCKED'])(
    '%s blocks retry',
    (code) => {
      const f = classifyConfirmError(new ApiError(code, 400));
      expect(f).toMatchObject({ kind: 'blocked', blocking: true });
      expect(f.message.length).toBeGreaterThan(0);
    },
  );
  it('ERR_AUTH_EXPIRED restarts with the product copy', () => {
    expect(classifyConfirmError(new ApiError('ERR_AUTH_EXPIRED', 410))).toMatchObject({
      kind: 'restart',
      message: 'This payment expired. Start again.',
    });
  });
  it('ERR_PIN_NOT_SET offers Set PIN at the shared route', () => {
    expect(classifyConfirmError(new ApiError('ERR_PIN_NOT_SET', 403))).toMatchObject({
      actionLabel: 'Set PIN',
      actionRoute: SECURITY_ROUTES.setPin,
    });
    expect(SECURITY_ROUTES.setPin).toBe('/(main)/security/set-pin');
  });
  it('ERR_TOTP_NOT_ENROLLED offers enrolment', () => {
    expect(classifyConfirmError(new ApiError('ERR_TOTP_NOT_ENROLLED', 403))).toMatchObject({
      actionLabel: 'Turn on two-step verification',
      actionRoute: '/(main)/security/totp-enroll',
    });
  });
  it('ERR_PIN_COOLING shows its message, no retry block', () => {
    expect(classifyConfirmError(new ApiError('ERR_PIN_COOLING', 403))).toMatchObject({ kind: 'other', blocking: false });
  });
  it('cancel is silent', () => {
    expect(classifyConfirmError(new AuthCancelledError())).toMatchObject({ kind: 'cancelled', message: '' });
  });
});
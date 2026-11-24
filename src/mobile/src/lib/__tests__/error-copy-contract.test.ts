import * as fs from 'node:fs';
import * as path from 'node:path';

import { ERROR_CODES_WITH_COPY, getErrorMessage, DEFAULT_ERROR_MESSAGE } from '../errors';

// Contract: every ERR_* code the backend documents must either have user-facing copy in
// errors.ts or be listed here as NOT reachable from the mobile app, with the reason. A new
// backend code therefore fails this test until someone decides which of the two it is.
const NOT_FOR_MOBILE: Record<string, string> = {
  ERR_INTERNAL: 'generic 500: the default message is the right copy',
  ERR_KEY_INACTIVE: 'admin AI key management',
  ERR_KEY_NOT_FOUND: 'admin AI key management',
  ERR_POLICY_VERSION_CONFLICT: 'admin approval policy editor',
  ERR_RECON_ALREADY_RAN: 'admin reconciliation',
  ERR_RECON_NOT_FOUND: 'admin reconciliation',
  ERR_FLAG_NOT_FOUND: 'admin fraud review',
  ERR_ALREADY_RESOLVED: 'admin fraud review',
  ERR_ALREADY_REVERSED: 'admin reversal',
  ERR_REVERSAL_PENDING: 'admin reversal',
  ERR_TXN_NOT_REVERSIBLE: 'admin reversal',
  ERR_SELF_APPROVAL: 'admin approval (checkers act in the admin app)',
  ERR_INVALID_CALLBACK_SECRET: 'bank callback, called by the bank not the app',
  ERR_CALLBACK_AMOUNT_MISMATCH: 'bank callback, called by the bank not the app',
  ERR_AUTH_NOT_CONFIGURED: 'server misconfiguration: the user sees the default message',
  ERR_OTP_DELIVERY_UNAVAILABLE: 'covered by the default message until a delivery channel exists',
};

function documentedCodes(): string[] {
  const file = path.resolve(__dirname, '../../../../../docs/reference/error-codes.md');
  const text = fs.readFileSync(file, 'utf8');
  return Array.from(new Set(Array.from(text.matchAll(/\|\s*(ERR_[A-Z0-9_]+)\s*\|/g), (m) => m[1]))).sort();
}

describe('error copy covers every documented backend code', () => {
  const codes = documentedCodes();

  it('reads the reference doc', () => {
    expect(codes.length).toBeGreaterThan(50);
  });

  it('every code has copy or an explicit reason it is not for mobile', () => {
    const missing = codes.filter((c) => !ERROR_CODES_WITH_COPY.includes(c) && !(c in NOT_FOR_MOBILE));
    expect(missing).toEqual([]);
  });

  it('a code is never both in the exclusion list and given copy', () => {
    const both = Object.keys(NOT_FOR_MOBILE).filter((c) => ERROR_CODES_WITH_COPY.includes(c));
    expect(both).toEqual([]);
  });

  it('every code with copy returns something other than the default message', () => {
    for (const c of ERROR_CODES_WITH_COPY) {
      expect(getErrorMessage(c)).not.toBe(DEFAULT_ERROR_MESSAGE);
    }
  });

  it('the exclusion list names no code the backend no longer documents', () => {
    const stale = Object.keys(NOT_FOR_MOBILE).filter((c) => !codes.includes(c));
    expect(stale).toEqual([]);
  });
});

describe('copy for the PIN, 2fa and limit codes (BUG-0017/0018, ADR-0016/0017)', () => {
  const cases: [string, RegExp][] = [
    ['ERR_PIN_WEAK', /PIN/],
    ['ERR_PIN_NOT_SET', /PIN/],
    ['ERR_PIN_INCORRECT', /PIN/],
    ['ERR_PIN_LOCKED', /PIN/],
    ['ERR_PIN_COOLING', /\$100\.00/],
    ['ERR_PIN_RESET_RATE_LIMITED', /PIN reset/],
    ['ERR_TOTP_NOT_ENROLLED', /authenticator/],
    ['ERR_TOTP_INVALID', /[Aa]uthenticator code/],
    ['ERR_TOTP_LOCKED', /locked/],
    ['ERR_TOTP_ALREADY_ENABLED', /authenticator/],
    ['ERR_AUTH_EXPIRED', /10 minutes|expired/],
    ['ERR_LIMIT_DAILY', /daily/],
    ['ERR_LIMIT_MONTHLY', /monthly/],
    ['ERR_LIMIT_PER_TXN', /transaction/],
    ['ERR_TOO_FREQUENT', /try again/],
    ['ERR_LOGIN_LOCKED', /15 minutes/],
  ];
  it.each(cases)('%s', (code, pattern) => {
    expect(getErrorMessage(code)).toMatch(pattern);
  });
});
import { ApiError, DEFAULT_ERROR_MESSAGE, NetworkError, describeError, getErrorMessage } from '../errors';

describe('error mapping (doc 02 §6 — no scattered messages)', () => {
  it('maps known ERR_* codes to English copy', () => {
    expect(getErrorMessage('ERR_INSUFFICIENT_BALANCE')).toBe('Insufficient balance for this transaction.');
    expect(getErrorMessage('ERR_TOKEN_REUSED')).toContain('Unusual');
  });

  it('falls back to a default message for unknown or missing codes', () => {
    expect(getErrorMessage('ERR_SOMETHING_NEW_FROM_BACKEND')).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage(undefined)).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage(null)).toBe(DEFAULT_ERROR_MESSAGE);
  });

  it('describeError resolves ApiError and NetworkError to user-facing copy', () => {
    expect(describeError(new ApiError('ERR_AMOUNT_INVALID', 400))).toBe('Invalid amount.');
    expect(describeError(new NetworkError())).toMatch(/network/);
    expect(describeError(new Error('boom'))).toBe(DEFAULT_ERROR_MESSAGE);
  });
});

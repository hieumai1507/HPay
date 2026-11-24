import { ApiError, DEFAULT_ERROR_MESSAGE, NetworkError, describeError, getErrorMessage } from '../errors';

describe('error mapping (doc 02 §6 — no message rải rác)', () => {
  it('maps known ERR_* codes to Vietnamese copy', () => {
    expect(getErrorMessage('ERR_INSUFFICIENT_BALANCE')).toBe('Số dư không đủ để thực hiện giao dịch.');
    expect(getErrorMessage('ERR_TOKEN_REUSED')).toContain('bất thường');
  });

  it('falls back to a default message for unknown or missing codes', () => {
    expect(getErrorMessage('ERR_SOMETHING_NEW_FROM_BACKEND')).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage(undefined)).toBe(DEFAULT_ERROR_MESSAGE);
    expect(getErrorMessage(null)).toBe(DEFAULT_ERROR_MESSAGE);
  });

  it('describeError resolves ApiError and NetworkError to user-facing copy', () => {
    expect(describeError(new ApiError('ERR_AMOUNT_INVALID', 400))).toBe('Số tiền không hợp lệ.');
    expect(describeError(new NetworkError())).toMatch(/mạng/);
    expect(describeError(new Error('boom'))).toBe(DEFAULT_ERROR_MESSAGE);
  });
});

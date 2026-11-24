import { renderHook, act } from '@testing-library/react-native';

import { apiRequest } from '@/src/api/client';
import { ApiError } from '@/src/lib/errors';
import { wrapper } from '@/src/features/security/testUtils';

import { useVaultMove } from '../useVaultQueries';

jest.mock('@/src/api/client', () => ({ apiRequest: jest.fn() }));
const api = apiRequest as jest.Mock;
const calls = (p: string) => api.mock.calls.filter((c) => c[0] === p);

describe('Test_useVaultMove_StepUp', () => {
  beforeEach(() => api.mockReset());

  it('requests once then confirms with exactly the factors collected for the band', async () => {
    api.mockImplementation(async (p: string) =>
      p === '/v1/transactions/request' ? { transaction_id: 'tx-9', auth_required: '2fa' } : { ref_code: 'R' },
    );
    const collect = jest.fn().mockResolvedValue({ pin: '482913', totp: '123456' });
    const { result } = renderHook(() => useVaultMove(), { wrapper: wrapper() });
    await act(async () => {
      await result.current.deposit('v1', 5000n, { idempotencyKey: 'k', collect });
    });
    expect(collect).toHaveBeenCalledWith('2fa', { transactionId: 'tx-9' }, expect.anything());
    expect(calls('/v1/transactions/request')).toHaveLength(1);
    expect(calls('/v1/transactions/confirm')[0][1].body).toStrictEqual({
      transaction_id: 'tx-9',
      pin: '482913',
      totp: '123456',
    });
  });

  it('cancel sends no confirm', async () => {
    api.mockResolvedValue({ transaction_id: 'tx-9', auth_required: 'pin' });
    const collect = jest.fn().mockResolvedValue(null);
    const { result } = renderHook(() => useVaultMove(), { wrapper: wrapper() });
    await act(async () => {
      await expect(result.current.withdraw('v1', 5000n, { idempotencyKey: 'k', collect })).rejects.toMatchObject({
        name: 'AuthCancelledError',
      });
    });
    expect(calls('/v1/transactions/confirm')).toHaveLength(0);
  });

  it('ERR_AUTH_EXPIRED propagates for the screen to classify', async () => {
    api.mockImplementation(async (p: string) => {
      if (p === '/v1/transactions/request') return { transaction_id: 'tx-9', auth_required: 'pin' };
      throw new ApiError('ERR_AUTH_EXPIRED', 410);
    });
    const collect = jest.fn().mockResolvedValue({ pin: '482913' });
    const { result } = renderHook(() => useVaultMove(), { wrapper: wrapper() });
    await act(async () => {
      await expect(result.current.deposit('v1', 5000n, { idempotencyKey: 'k', collect })).rejects.toMatchObject({
        code: 'ERR_AUTH_EXPIRED',
      });
    });
    expect(calls('/v1/transactions/request')).toHaveLength(1);
  });
});
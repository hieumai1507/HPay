import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react-native';
import type { ReactNode } from 'react';

import { deferred } from '@/src/test-utils/security';

import { useResetPin } from '../useResetPin';
import { useSetPinForm } from '../useSetPinForm';

const mockApi = jest.fn();
jest.mock('@/src/api/client', () => ({ apiRequest: (...a: unknown[]) => mockApi(...a) }));

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
);

// The screens also disable the button while pending; these tests prove the hook itself
// rejects a second call made in the same tick, before React re-renders.
describe('hooks send once on a same-tick double submit', () => {
  beforeEach(() => mockApi.mockReset());

  it('useSetPinForm', async () => {
    const d = deferred<unknown>();
    mockApi.mockReturnValue(d.promise);
    const { result } = renderHook(() => useSetPinForm(), { wrapper });
    act(() => {
      result.current.setPassword('pw');
      result.current.setPin('482913');
      result.current.setRepeat('482913');
    });
    await act(async () => {
      void result.current.submit();
      void result.current.submit();
    });
    expect(mockApi).toHaveBeenCalledTimes(1);
    await act(async () => d.resolve({ status: 'pin_set' }));
  });

  it('useResetPin request', async () => {
    const d = deferred<unknown>();
    mockApi.mockReturnValue(d.promise);
    const { result } = renderHook(() => useResetPin(), { wrapper });
    act(() => result.current.setPassword('pw'));
    await act(async () => {
      void result.current.requestCode();
      void result.current.requestCode();
    });
    expect(mockApi).toHaveBeenCalledTimes(1);
    await act(async () => d.resolve({ status: 'code_sent', expires_in: 300 }));
  });
});
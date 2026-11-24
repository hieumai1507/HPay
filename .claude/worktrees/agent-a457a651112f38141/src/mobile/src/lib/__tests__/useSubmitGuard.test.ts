import { act, renderHook } from '@testing-library/react-native';

import { useSubmitGuard } from '../useSubmitGuard';

function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

describe('TestEdge_M01_SubmitGuard_SingleFlight', () => {
  it('runs the action once when called twice synchronously (before any re-render)', async () => {
    const { result } = await renderHook(() => useSubmitGuard());
    const d = deferred<string>();
    const action = jest.fn(() => d.promise);

    let first: Promise<string | undefined>;
    let second: Promise<string | undefined>;
    await act(async () => {
      first = result.current.run(action);
      second = result.current.run(action); // same tick: React state has not updated yet
    });

    expect(action).toHaveBeenCalledTimes(1);
    expect(result.current.isPending).toBe(true);

    await act(async () => {
      d.resolve('done');
    });
    await expect(first!).resolves.toBe('done');
    await expect(second!).resolves.toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('releases the guard after a rejection so the user can retry', async () => {
    const { result } = await renderHook(() => useSubmitGuard());
    const failing = jest.fn().mockRejectedValue(new Error('boom'));

    await act(async () => {
      await expect(result.current.run(failing)).rejects.toThrow('boom');
    });
    expect(result.current.isPending).toBe(false);

    const ok = jest.fn().mockResolvedValue(1);
    await act(async () => {
      await result.current.run(ok);
    });
    expect(ok).toHaveBeenCalledTimes(1);
  });
});

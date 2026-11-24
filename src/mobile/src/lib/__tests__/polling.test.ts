import { POLL_BACKOFF_MS, POLL_TIMEOUT_MS, pollUntilSettled } from '../polling';

describe('polling backoff (M02/M03 groundwork)', () => {
  afterEach(() => {
    jest.useRealTimers();
  });

  it('exposes the 1s -> 2s -> 4s backoff shape and 30s cap from the architecture doc', () => {
    expect(POLL_BACKOFF_MS).toEqual([1000, 2000, 4000]);
    expect(POLL_TIMEOUT_MS).toBe(30000);
  });

  it('resolves immediately once isSettled is true, without waiting out any backoff', async () => {
    const fetcher = jest.fn().mockResolvedValue({ status: 'completed' });
    const result = await pollUntilSettled(fetcher, (d: { status: string }) => d.status === 'completed');
    expect(result).toEqual({ kind: 'settled', data: { status: 'completed' } });
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('never reports "failed" on timeout — reports "timeout" so the UI shows "Processing"', async () => {
    jest.useFakeTimers();
    const fetcher = jest.fn().mockResolvedValue({ status: 'pending' });

    const promise = pollUntilSettled(fetcher, (d: { status: string }) => d.status === 'completed', {
      timeoutMs: 7000,
    });
    await jest.advanceTimersByTimeAsync(7000);
    const result = await promise;

    expect(result).toEqual({ kind: 'timeout' });
    // t=0 fetch, sleep 1s -> t=1000 fetch, sleep 2s -> t=3000 fetch, sleep 4s -> t=7000 (cap reached)
    expect(fetcher).toHaveBeenCalledTimes(3);
  });

  it('returns aborted immediately if the signal is already aborted', async () => {
    const controller = new AbortController();
    controller.abort();
    const fetcher = jest.fn();

    const result = await pollUntilSettled(fetcher, () => false, { signal: controller.signal });

    expect(result).toEqual({ kind: 'aborted' });
    expect(fetcher).not.toHaveBeenCalled();
  });
});

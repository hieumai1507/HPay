/**
 * Groundwork for M02/M03: after a transfer is accepted (202), the client must NOT
 * declare failure just because the network hiccups or the workflow is still running.
 * It polls with backoff and only after POLL_TIMEOUT_MS gives up — surfaced to the user
 * as "processing" (still processing), never as "failed".
 *
 * `TransactionWatcher` is the interface screens depend on; `PollingTransactionWatcher`
 * is the v1 implementation. Doc 02 §6 reserves this seam so a later SSE-based watcher
 * can be swapped in without touching screen code.
 */

export const POLL_BACKOFF_MS = [1000, 2000, 4000] as const;
export const POLL_TIMEOUT_MS = 30000;

export type PollOutcome<T> = { kind: 'settled'; data: T } | { kind: 'timeout' } | { kind: 'aborted' };

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener(
      'abort',
      () => {
        clearTimeout(timer);
        reject(new DOMException('Aborted', 'AbortError'));
      },
      { once: true },
    );
  });
}

/**
 * Polls `fetcher` with 1s -> 2s -> 4s -> 4s... backoff until `isSettled(data)` is true
 * or POLL_TIMEOUT_MS elapses. Never rejects on timeout — callers show "processing" copy.
 */
export async function pollUntilSettled<T>(
  fetcher: () => Promise<T>,
  isSettled: (data: T) => boolean,
  options?: { signal?: AbortSignal; timeoutMs?: number },
): Promise<PollOutcome<T>> {
  const timeoutMs = options?.timeoutMs ?? POLL_TIMEOUT_MS;
  const signal = options?.signal;
  const start = Date.now();
  let attempt = 0;

  while (Date.now() - start < timeoutMs) {
    if (signal?.aborted) return { kind: 'aborted' };

    const data = await fetcher();
    if (isSettled(data)) return { kind: 'settled', data };

    const delay = POLL_BACKOFF_MS[Math.min(attempt, POLL_BACKOFF_MS.length - 1)];
    const remaining = timeoutMs - (Date.now() - start);
    if (remaining <= 0) break;

    try {
      await sleep(Math.min(delay, remaining), signal);
    } catch {
      return { kind: 'aborted' };
    }
    attempt++;
  }

  return { kind: 'timeout' };
}

export interface TransactionWatcher<T> {
  /** Starts polling; resolves with the terminal outcome. Call `cancel()` to stop early. */
  watch(refCode: string): { result: Promise<PollOutcome<T>>; cancel: () => void };
}

/** v1 watcher: plain HTTP polling with backoff. Swap for an SSE watcher later without screen changes. */
export class PollingTransactionWatcher<T> implements TransactionWatcher<T> {
  constructor(
    private readonly fetchStatus: (refCode: string) => Promise<T>,
    private readonly isSettled: (data: T) => boolean,
  ) {}

  watch(refCode: string): { result: Promise<PollOutcome<T>>; cancel: () => void } {
    const controller = new AbortController();
    const result = pollUntilSettled(() => this.fetchStatus(refCode), this.isSettled, {
      signal: controller.signal,
    });
    return { result, cancel: () => controller.abort() };
  }
}

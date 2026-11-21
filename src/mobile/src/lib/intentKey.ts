import * as Crypto from 'expo-crypto';
import * as SecureStore from 'expo-secure-store';

import { ApiError } from './errors';

/**
 * One Idempotency-Key per user intent (C-03), persisted so it is stable across retries,
 * re-renders, remounts of the confirm screen and app restarts. The old approach
 * (`useMemo` per mount) minted a new key every time the screen remounted, so a retry after
 * a timeout created a second transaction.
 *
 * An "intent" is a string that identifies what the user is trying to do, for example
 * `transfer:<receiver>:<amount>:<note>`. The key is released after success or after a
 * definitive rejection (4xx other than 409/429); on network errors, 5xx, 409 and 429 the
 * outcome is unknown, so the same key must be reused on retry.
 */

const STORE_KEY = 'hpay_intent_keys';
export const INTENT_KEY_TTL_MS = 30 * 60 * 1000;

interface Entry {
  key: string;
  at: number;
}

let memory: Record<string, Entry> | null = null;
let chain: Promise<unknown> = Promise.resolve();

/** Serialises read-modify-write on the persisted map. */
function exclusive<T>(fn: () => Promise<T>): Promise<T> {
  const next = chain.then(fn, fn);
  chain = next.catch(() => undefined);
  return next;
}

async function load(): Promise<Record<string, Entry>> {
  if (memory) return memory;
  try {
    const raw = await SecureStore.getItemAsync(STORE_KEY);
    memory = raw ? (JSON.parse(raw) as Record<string, Entry>) : {};
  } catch {
    memory = {};
  }
  return memory;
}

async function persist(map: Record<string, Entry>): Promise<void> {
  try {
    await SecureStore.setItemAsync(STORE_KEY, JSON.stringify(map));
  } catch {
    // Persistence is best effort; the in-memory copy still guards this process.
  }
}

export function getIntentKey(intent: string): Promise<string> {
  return exclusive(async () => {
    const map = await load();
    const now = Date.now();
    for (const k of Object.keys(map)) {
      if (now - map[k].at > INTENT_KEY_TTL_MS) delete map[k];
    }
    let entry = map[intent];
    if (!entry) {
      entry = { key: Crypto.randomUUID(), at: now };
      map[intent] = entry;
      await persist(map);
    }
    return entry.key;
  });
}

export function releaseIntentKey(intent: string): Promise<void> {
  return exclusive(async () => {
    const map = await load();
    if (intent in map) {
      delete map[intent];
      await persist(map);
    }
  });
}

/**
 * Runs a money-moving call under the intent's persisted key. The key is released on
 * success and on a definitive rejection, and kept on ambiguous failures so a retry of the
 * same intent replays the same key (server dedupes).
 */
export async function withIntentKey<T>(intent: string, run: (key: string) => Promise<T>): Promise<T> {
  const key = await getIntentKey(intent);
  try {
    const result = await run(key);
    await releaseIntentKey(intent);
    return result;
  } catch (err) {
    if (shouldReleaseIntentKey(err)) await releaseIntentKey(intent);
    throw err;
  }
}

/** True only when the server definitively rejected the request, so a new attempt is a new intent. */
export function shouldReleaseIntentKey(err: unknown): boolean {
  if (err instanceof ApiError) {
    return err.status >= 400 && err.status < 500 && err.status !== 409 && err.status !== 429;
  }
  return false;
}

/** Test helper: forget the in-process copy, as a remount or restart would. */
export function __resetIntentKeyMemory(): void {
  memory = null;
}